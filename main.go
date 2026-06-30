package main

import (
	"log"

	"github.com/mogensen/lensrace/internal/config"
	"github.com/mogensen/lensrace/internal/db"
	"github.com/mogensen/lensrace/internal/server"
)

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

	app := server.New(conn)
	log.Fatal(app.Listen(":" + cfg.Port))
}
