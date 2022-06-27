package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/model"
	"time"

	"github.com/adjust/rmq/v4"
	"github.com/ethereum/go-ethereum"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type Indexer struct {
	db                 *database.Database
	ethClient          *ethclient.Client
	blockTaskQueue     rmq.Queue
	txReceiptTaskQueue rmq.Queue
}

func NewIndexer(
	db *database.Database,
	ethClient *ethclient.Client,
	blockTaskQueue rmq.Queue,
	txReceiptTaskQueue rmq.Queue,
) *Indexer {
	return &Indexer{
		db:                 db,
		ethClient:          ethClient,
		blockTaskQueue:     blockTaskQueue,
		txReceiptTaskQueue: txReceiptTaskQueue,
	}
}

func (i *Indexer) Run() (err error) {
	// check if there are pending tx receipt in db not processed yet
	var pendingTxs []*model.Transaction
	err = i.db.Tx(func(tx *gorm.DB) error {
		return tx.Select("hash").Where("receipt_ready = ?", false).Find(&pendingTxs).Error
	})
	if err != nil {
		return
	}

	for _, tx := range pendingTxs {
		err = i.txReceiptTaskQueue.Publish(tx.Hash)
		if err != nil {
			return
		}
	}

	// get latest block number
	var latestBlockNum uint64
	latestBlockNum, err = i.ethClient.BlockNumber(context.Background())
	if err != nil {
		return
	}

	i.syncBlocks(latestBlockNum)

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

		i.addGetBlockTask(blockH.Number.Uint64())
	}
}

func (i *Indexer) syncBlocks(latestBlockNum uint64) (err error) {
	// find block numbers in db
	var blockNumbersInDB []uint64
	err = i.db.Tx(func(tx *gorm.DB) error {
		return tx.Model(&model.Block{}).Select("number").Find(&blockNumbersInDB).Error
	})
	if err != nil {
		return
	}
	blockNumberInDBMap := make(map[uint64]struct{})
	for _, n := range blockNumbersInDB {
		blockNumberInDBMap[n] = struct{}{}
	}

	for n := latestBlockNum; ; n-- {
		if _, exist := blockNumberInDBMap[n]; exist {
			continue
		}
		i.addGetBlockTask(n)

		if n == 0 {
			// n is uint, when n = 0, n-- will result in underflow, keep loop never stop
			break
		}
	}

	return
}

func (i *Indexer) addGetBlockTask(blockNum uint64) (err error) {
	return i.blockTaskQueue.Publish(fmt.Sprintf("%d", blockNum))
}
