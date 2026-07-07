package main

import (
	"context"
	"log"
	"time"

	"github.com/mogensen/lensrace/internal/catalog"
	"github.com/mogensen/lensrace/internal/config"
	"github.com/mogensen/lensrace/internal/db"
	"github.com/mogensen/lensrace/internal/realtime"
	"github.com/mogensen/lensrace/internal/server"
	"github.com/mogensen/lensrace/internal/store"
)

const expiryCheckInterval = time.Second

func main() {
	cfg := config.Load()

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	cat, err := catalog.Load()
	if err != nil {
		log.Fatalf("load catalog: %v", err)
	}

	hub := realtime.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go realtime.WatchExpirations(ctx, hub, store.New(conn, cat), expiryCheckInterval)

	app := server.New(conn, hub, cat)
	log.Fatal(app.Listen(":" + cfg.Port))
}
