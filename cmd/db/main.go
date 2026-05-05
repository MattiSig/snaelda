package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/MattiSig/snaelda/internal/platform/config"
	"github.com/MattiSig/snaelda/internal/platform/database"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch os.Args[1] {
	case "migrate":
		direction := "up"
		if len(os.Args) >= 3 {
			direction = os.Args[2]
		}
		sqlDB, err := database.OpenSQL(cfg.DatabaseURL)
		if err != nil {
			logger.Error("open database", "error", err)
			os.Exit(1)
		}
		defer sqlDB.Close()

		if err := database.Migrate(ctx, sqlDB, direction); err != nil {
			logger.Error("run migrations", "direction", direction, "error", err)
			os.Exit(1)
		}
	case "seed":
		pool, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("open database", "error", err)
			os.Exit(1)
		}
		defer pool.Close()

		if err := database.SeedDevelopment(ctx, pool); err != nil {
			logger.Error("seed database", "error", err)
			os.Exit(1)
		}
		logger.Info("database seeded")
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/db migrate [up|down|status|reset]")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/db seed")
}
