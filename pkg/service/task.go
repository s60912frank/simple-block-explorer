package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log"
	"math/big"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/model"

	"github.com/adjust/rmq/v4"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type TaskConsumer struct {
	q         rmq.Queue
	db        *database.Database
	ethClient *ethclient.Client
}

func NewTaskConsumer(q rmq.Queue, db *database.Database, ethClient *ethclient.Client) *TaskConsumer {
	return &TaskConsumer{
		db:        db,
		q:         q,
		ethClient: ethClient,
	}
}

type TaskType uint8

const (
	TaskTypeGetBlock TaskType = iota
	TaskTypeGetTxReceipt
)

type Task struct {
	Type        TaskType `json:"type"`
	BlockNumber uint64   `json:"block_number"`
	TxHash      string   `json:"tx_hash"`
}

func (c *TaskConsumer) Consume(delivery rmq.Delivery) {
	var err error
	defer func() {
		if err != nil {
			log.Printf("%s failed: %s", delivery.Payload(), err)
			if err = delivery.Reject(); err != nil {
				log.Fatalf("reject task failed: %s", err)
			}
		} else {
			log.Printf("%s DONE", delivery.Payload())
			if err := delivery.Ack(); err != nil {
				log.Fatalf("ack task failed: %s", err)
			}
		}
	}()

	var task Task
	if err = json.Unmarshal([]byte(delivery.Payload()), &task); err != nil {
		return
	}

	switch task.Type {
	case TaskTypeGetBlock:
		err = c.handleGetBlock(task.BlockNumber)
	case TaskTypeGetTxReceipt:
		err = c.handleGetTxReceipt(task.TxHash)
	default:
		log.Printf("Unknown task type %d", task.Type)
	}

}

func (c *TaskConsumer) handleGetBlock(blockNum uint64) (err error) {
	// TODO: context
	var block *ethTypes.Block
	block, err = c.ethClient.BlockByNumber(context.Background(), big.NewInt(int64(blockNum)))
	if err != nil {
		return
	}

	log.Printf("Got block %d with %d txs", blockNum, len(block.Transactions()))
	// check if block with same hash already in db
	var count int64
	err = c.db.Tx(func(tx *gorm.DB) error {
		return tx.Model(&model.Block{}).Where("hash = ?", block.Hash().Hex()).Count(&count).Error
	})
	if err != nil {
		return
	}
	if count > 0 {
		// if not error (found), we can just return
		return
	}

	blockDB := &model.Block{
		Number:     block.NumberU64(),
		Hash:       block.Hash().Hex(),
		ParentHash: block.ParentHash().Hex(),
		Timestamp:  block.Time(),
	}

	for _, tx := range block.Transactions() {
		var from common.Address
		from, err = ethTypes.Sender(ethTypes.NewEIP155Signer(tx.ChainId()), tx)
		if err != nil {
			from, err = ethTypes.Sender(ethTypes.HomesteadSigner{}, tx)
			if err != nil {
				return
			}
		}

		// push get tx receipt to queue, so we can consume it later
		receiptTask := Task{
			Type:   TaskTypeGetTxReceipt,
			TxHash: tx.Hash().Hex(),
		}
		var payload []byte
		payload, err = json.Marshal(&receiptTask)
		if err != nil {
			return
		}
		err = c.q.PublishBytes(payload)
		if err != nil {
			return
		}

		txDB := &model.Transaction{
			Hash:         tx.Hash().Hex(),
			RefBlockHash: blockDB.Hash,
			From:         from.Hex(),
			Data:         hex.EncodeToString(tx.Data()),
			Nonce:        tx.Nonce(),
			Value:        tx.Value().Uint64(),
		}
		if tx.To() != nil {
			to := tx.To().Hex()
			txDB.To = &to
		}

		blockDB.Transactions = append(blockDB.Transactions, txDB)
	}

	err = c.db.Tx(func(tx *gorm.DB) error {
		// save block
		// blockDB.Transactions = []*model.Transaction{}
		// return tx.Save(&blockDB).Error
		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Create(&blockDB).Error
	})
	return
}

func (c *TaskConsumer) handleGetTxReceipt(txHash string) (err error) {
	var txReceipt *ethTypes.Receipt
	txReceipt, err = c.ethClient.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		return
	}

	// update db
	err = c.db.Tx(func(tx *gorm.DB) error {
		return tx.
			Model(&model.Transaction{Hash: txHash}).
			Updates(map[string]interface{}{"receipt_ready": true, "logs": txReceipt.Logs}).
			Error
	})
	return
}
