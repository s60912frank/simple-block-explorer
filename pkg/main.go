package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"portto-explorer/pkg/config"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/model"
	"portto-explorer/pkg/service"
	"syscall"
	"time"

	"github.com/adjust/rmq/v4"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer cancel()

	db := database.New()

	webSrv := service.NewWebServer(db)
	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := webSrv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server existed with error: %s\n", err)
		}
	}()

	conf := config.GetConfig().Indexer
	ethClient, err := ethclient.Dial(conf.RpcUrl)
	if err != nil {
		log.Fatal(err)
	}

	// open task queue
	// TODO: handle error channel
	redisConn, err := rmq.OpenConnection("my service", "tcp", "localhost:6379", 1, nil)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
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
	}()

	blockTaskQueue, err := redisConn.OpenQueue(conf.BlockTaskQueueName)
	if err != nil {
		log.Fatal(err)
	}
	txReceiptTaskQueue, err := redisConn.OpenQueue(conf.TxReceiptTaskQueueName)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		// start block queue
		err := blockTaskQueue.StartConsuming(50, time.Millisecond*500)
		if err != nil {
			panic(err)
		}
		for i := 0; i < 50; i++ {
			name := fmt.Sprintf("block consumer %d", i)
			if _, err := blockTaskQueue.AddConsumer(name, service.NewBlockTaskConsumer(txReceiptTaskQueue, db, ethClient)); err != nil {
				panic(err)
			}
		}

		// start tx receipt queue
		err = txReceiptTaskQueue.StartConsuming(50, time.Millisecond*500)
		if err != nil {
			panic(err)
		}
		for i := 0; i < 50; i++ {
			name := fmt.Sprintf("tx receipt consumer %d", i)
			if _, err := txReceiptTaskQueue.AddConsumer(name, service.NewTxReceiptTaskConsumer(db, ethClient)); err != nil {
				panic(err)
			}
		}

		for {
			time.Sleep(time.Second * 3)
			// TODO: handle return error
			blockQReturned, _ := blockTaskQueue.ReturnRejected(100)
			txQReturned, _ := txReceiptTaskQueue.ReturnRejected(100)
			log.Printf("return rejected: (block) %d (tx) %d", blockQReturned, txQReturned)
		}
	}()

	indexer := service.NewIndexer(db, ethClient, blockTaskQueue, txReceiptTaskQueue)

	go func() {
		// TODO: indexer graceful shutdown
		if err := indexer.Run(); err != nil {
			log.Fatalf("indexer existed with error: %s\n", err)
		}
	}()

	<-ctx.Done()
	cancel()

	log.Println("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := webSrv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

}
