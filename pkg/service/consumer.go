package service

import (
	"context"
	"encoding/hex"
	"log"
	"math/big"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/model"
	"strconv"

	"github.com/adjust/rmq/v4"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type BlockTaskConsumer struct {
	txReceiptQ rmq.Queue
	db         *database.Database
	ethClient  *ethclient.Client
}

func NewBlockTaskConsumer(txReceiptQ rmq.Queue, db *database.Database, ethClient *ethclient.Client) *BlockTaskConsumer {
	return &BlockTaskConsumer{
		db:         db,
		txReceiptQ: txReceiptQ,
		ethClient:  ethClient,
	}
}

func (c *BlockTaskConsumer) Consume(delivery rmq.Delivery) {
	var err error
	defer func() {
		if err != nil {
			log.Printf("%s failed: %s", delivery.Payload(), err)
			if err = delivery.Reject(); err != nil {
				log.Fatalf("reject task failed: %s", err)
			}
		} else {
			// log.Printf("%s DONE", delivery.Payload())
			if err := delivery.Ack(); err != nil {
				log.Fatalf("ack task failed: %s", err)
			}
		}
	}()

	var blockNum uint64
	if blockNum, err = strconv.ParseUint(delivery.Payload(), 10, 64); err != nil {
		return
	}

	err = c.handleGetBlock(blockNum)
}

func (c *BlockTaskConsumer) handleGetBlock(blockNum uint64) (err error) {
	// TODO: context
	var block *ethTypes.Block
	block, err = c.ethClient.BlockByNumber(context.Background(), big.NewInt(int64(blockNum)))
	if err != nil {
		return
	}

	// log.Printf("Got block %d with %d txs", blockNum, len(block.Transactions()))
	// check if block with same hash already in db
	var count int64
	err = c.db.Tx(func(tx *gorm.DB) error {
		return tx.Model(&model.Block{}).Where("hash = ?", block.Hash().Hex()).Count(&count).Error
	})
	if err != nil {
		return
	}
	if count > 0 {
		// if not not found (found), we can just return
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
		err = c.txReceiptQ.Publish(tx.Hash().Hex())
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
		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Create(&blockDB).Error
	})
	return
}

type TxReceiptTaskConsumer struct {
	db        *database.Database
	ethClient *ethclient.Client
}

func NewTxReceiptTaskConsumer(db *database.Database, ethClient *ethclient.Client) *TxReceiptTaskConsumer {
	return &TxReceiptTaskConsumer{
		db:        db,
		ethClient: ethClient,
	}
}

func (c *TxReceiptTaskConsumer) Consume(delivery rmq.Delivery) {
	var err error
	defer func() {
		if err != nil {
			log.Printf("%s failed: %s", delivery.Payload(), err)
			if err = delivery.Reject(); err != nil {
				log.Fatalf("reject task failed: %s", err)
			}
		} else {
			// log.Printf("%s DONE", delivery.Payload())
			if err := delivery.Ack(); err != nil {
				log.Fatalf("ack task failed: %s", err)
			}
		}
	}()

	err = c.handleGetTxReceipt(delivery.Payload())
}

func (c *TxReceiptTaskConsumer) handleGetTxReceipt(txHash string) (err error) {
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
