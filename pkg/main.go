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
	"portto-explorer/pkg/service"
	"portto-explorer/pkg/utils"
	"syscall"
	"time"

	"github.com/adjust/rmq/v4"
	"github.com/ethereum/go-ethereum/ethclient"
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
	redisErrCh := make(chan error)
	redisConn, err := rmq.OpenConnection(conf.RedisTag, "tcp", conf.RedisEndpoint, 1, redisErrCh)
	if err != nil {
		log.Fatal(err)
	}

	go utils.LogRedisErrors(redisErrCh)
	go utils.LogStats(redisConn, db)

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
			blockQReturned, err := blockTaskQueue.ReturnRejected(100)
			if err != nil {
				log.Printf("return block task queue err: %s", err)
			}
			txQReturned, _ := txReceiptTaskQueue.ReturnRejected(100)
			if err != nil {
				log.Printf("return tx receipt task queue err: %s", err)
			}
			log.Printf("return rejected: (block) %d (tx) %d", blockQReturned, txQReturned)
		}
	}()

	indexer := service.NewIndexer(db, ethClient, blockTaskQueue, txReceiptTaskQueue)

	go func() {
		if err := indexer.Run(); err != nil {
			log.Fatalf("indexer existed with error: %s\n", err)
		}
	}()

	<-ctx.Done()
	cancel()

	log.Println("shutting down gracefully, press Ctrl+C again to force")
	<-redisConn.StopAllConsuming() // wait for all Consume() calls to finish

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := webSrv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}
}
