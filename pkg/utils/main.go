package utils

import (
	"errors"
	"log"
	"net/http"
	"portto-explorer/pkg/config"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/model"
	"time"

	"github.com/adjust/rmq/v4"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func WrapperErr(f func(c *gin.Context) error) func(c *gin.Context) {
	return func(c *gin.Context) {
		if err := f(c); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "record not found, please revise your condition"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			c.Abort()
		}
	}
}

func LogRedisErrors(errChan <-chan error) {
	for err := range errChan {
		switch err := err.(type) {
		case *rmq.HeartbeatError:
			if err.Count == rmq.HeartbeatErrorLimit {
				log.Print("redis heartbeat error (limit): ", err)
			} else {
				log.Print("redis heartbeat error: ", err)
			}
		case *rmq.ConsumeError:
			log.Print("redis consume error: ", err)
		case *rmq.DeliveryError:
			log.Print("redis delivery error: ", err.Delivery, err)
		default:
			log.Print("redis other error: ", err)
		}
	}
}

func LogStats(redisConn rmq.Connection, db *database.Database) {
	conf := config.GetConfig().Indexer
	for {
		s, _ := redisConn.CollectStats([]string{conf.BlockTaskQueueName, conf.TxReceiptTaskQueueName})
		blockStats := s.QueueStats[conf.BlockTaskQueueName]
		log.Printf("block queue stats: pending %d failed %d running %d", blockStats.ReadyCount, blockStats.RejectedCount, blockStats.UnackedCount())
		txStats := s.QueueStats[conf.TxReceiptTaskQueueName]
		log.Printf("tx queue stats: pending %d failed %d running %d", txStats.ReadyCount, txStats.RejectedCount, txStats.UnackedCount())

		var blockCount, txCount, txWithoutReceiptCount int64
		db.Tx(func(tx *gorm.DB) error {
			_ = tx.Model(&model.Block{}).Count(&blockCount).Error
			_ = tx.Model(&model.Transaction{}).Count(&txCount).Error
			_ = tx.Model(&model.Transaction{}).Where("receipt_ready = ?", false).Count(&txWithoutReceiptCount).Error
			return nil
		})
		log.Printf("db stats: block %d tx %d tx w/o receipt %d", blockCount, txCount, txWithoutReceiptCount)

		time.Sleep(time.Second * 3)
	}
}
