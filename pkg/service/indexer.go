package service

import (
	"context"
	"encoding/json"
	"errors"
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
	db        *database.Database
	ethClient *ethclient.Client
	q         rmq.Queue
	qConn     rmq.Connection
}

func NewIndexer(db *database.Database, ethClient *ethclient.Client, q rmq.Queue, qConn rmq.Connection) *Indexer {
	return &Indexer{
		db:        db,
		ethClient: ethClient,
		q:         q,
		qConn:     qConn,
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
		task := Task{
			Type:   TaskTypeGetTxReceipt,
			TxHash: tx.Hash,
		}

		var payload []byte
		payload, err = json.Marshal(&task)
		if err != nil {
			return
		}

		err = i.q.PublishBytes(payload)
		if err != nil {
			return
		}
	}

	// get latest block number
	// TODO: manage context
	var latestBlockNum uint64
	latestBlockNum, err = i.ethClient.BlockNumber(context.Background())
	if err != nil {
		return
	}

	// find block numbers in db
	var blockNumbersInDB []uint64
	err = i.db.Tx(func(tx *gorm.DB) error {
		return tx.Model(&model.Block{}).Select("number").Find(&blockNumbersInDB).Error
	})
	if err != nil {
		return
	}
	blocksToGet := make(map[uint64]struct{})
	var n uint64
	for ; n <= latestBlockNum; n++ {
		blocksToGet[n] = struct{}{}
	}
	for _, n := range blockNumbersInDB {
		delete(blocksToGet, n)
	}

	i.syncBlocks(blocksToGet)

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

func (i *Indexer) syncBlocks(blockNums map[uint64]struct{}) (err error) {
	for n := range blockNums {
		i.addGetBlockTask(n)
	}

	return
}

func (i *Indexer) addGetBlockTask(blockNum uint64) (err error) {
	// TODO: try make producer slower?
	// for {
	// 	qName := config.GetConfig().Indexer.TaskQueueName
	// 	s, _ := i.qConn.CollectStats([]string{qName})
	// 	stats := s.QueueStats[qName]
	// 	pendingCount := stats.ReadyCount + stats.RejectedCount
	// 	if pendingCount < 100 {
	// 		break
	// 	}
	// 	time.Sleep(time.Millisecond * time.Duration(pendingCount))
	// }

	task := Task{
		Type:        TaskTypeGetBlock,
		BlockNumber: blockNum,
	}

	var payload []byte
	payload, err = json.Marshal(&task)
	if err != nil {
		return
	}

	return i.q.PublishBytes(payload)
}
