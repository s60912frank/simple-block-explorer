package service

import (
	"context"
	"encoding/hex"
	"errors"
	"log"
	"math/big"
	"portto-explorer/pkg/config"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/model"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type Indexer struct {
	db        *database.Database
	ethClient *ethclient.Client
}

func NewIndexer(db *database.Database) (i *Indexer, err error) {
	conf := config.GetConfig().Indexer
	i = &Indexer{
		db: db,
	}
	i.ethClient, err = ethclient.Dial(conf.RpcUrl)
	if err != nil {
		return
	}

	return
}

func (i *Indexer) Run() (err error) {
	// get latest block number
	// TODO: manage context
	var latestBlockNum uint64
	latestBlockNum, err = i.ethClient.BlockNumber(context.Background())
	if err != nil {
		return
	}

	var latestBlockInDB model.Block
	var oldestBlockInDB model.Block
	err = i.db.Tx(func(tx *gorm.DB) error {
		err := tx.Select("number").Order("number DESC").First(&latestBlockInDB).Error
		if err != nil {
			return err
		}
		return tx.Select("number").Order("number ASC").First(&oldestBlockInDB).Error
	})

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// if it's error other than not found, throw it
		return
	}

	// TODO: put sync in go-routine?
	if err != nil {
		// database is empty, sync from latest
		i.syncBlocks(latestBlockNum, 0)
	} else {
		i.syncBlocks(latestBlockInDB.Number, latestBlockNum)
		if oldestBlockInDB.Number != 0 {
			// we haven't sync to oldest yet
			i.syncBlocks(0, oldestBlockInDB.Number)
		}
	}

	i.keepSync()

	return
}

// this function will keep checking latest blocks
func (i *Indexer) keepSync() (err error) {
	// get latest block number in db
	var latestBlockInDB model.Block
	err = i.db.Tx(func(tx *gorm.DB) error {
		return tx.Select("number").Order("number DESC").First(&latestBlockInDB).Error
	})

	if err != nil {
		// at this point db should not be empty
		return
	}

	blockNum := latestBlockInDB.Number + 1
	for {
		var blockH *ethTypes.Header
		blockH, err = i.ethClient.HeaderByNumber(context.Background(), big.NewInt(int64(blockNum)))
		if errors.Is(err, ethereum.NotFound) {
			// block not yet produced, wait few seconds and retry
			time.Sleep(time.Second * 5)
			continue
		}

		i.syncBlock(blockH.Number.Uint64())
	}
}

func (i *Indexer) syncBlocks(fromBlock, toBlock uint64) (err error) {
	// TODO: need some performance tuning

	if fromBlock > toBlock {
		// scan backward
		for n := fromBlock; n >= toBlock; n-- {
			i.syncBlock(n)
		}
	} else {
		// scan forward
		for n := fromBlock; n <= toBlock; n++ {
			i.syncBlock(n)
		}
	}

	return
}

func (i *Indexer) syncBlock(num uint64) (err error) {
	// TODO: context
	var block *ethTypes.Block
	block, err = i.ethClient.BlockByNumber(context.Background(), big.NewInt(int64(num)))
	if err != nil {
		return
	}

	log.Printf("Got block %d with %d txs", num, len(block.Transactions()))

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

		// get tx recipient
		// TODO: this can be expensive
		var txReceipt *ethTypes.Receipt
		txReceipt, err = i.ethClient.TransactionReceipt(context.Background(), tx.Hash())
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
			Logs:         txReceipt.Logs,
		}
		if tx.To() != nil {
			to := tx.To().Hex()
			txDB.To = &to
		}

		blockDB.Transactions = append(blockDB.Transactions, txDB)
	}

	err = i.db.Tx(func(tx *gorm.DB) error {
		// save block
		// blockDB.Transactions = []*model.Transaction{}
		// return tx.Save(&blockDB).Error
		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Create(&blockDB).Error
	})
	return
}
