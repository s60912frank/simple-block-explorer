package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"portto-explorer/pkg/database"
	"portto-explorer/pkg/service"
	"syscall"
	"time"
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

	indexer, err := service.NewIndexer(db)
	if err != nil {
		log.Fatal(err)
	}

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
