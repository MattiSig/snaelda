package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func OpenSQL(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	return stdlib.OpenDB(*cfg), nil
}

func Migrate(ctx context.Context, db *sql.DB, direction string) error {
	if err := configureGoose(); err != nil {
		return err
	}

	switch direction {
	case "up":
		return goose.UpContext(ctx, db, "migrations")
	case "down":
		return goose.DownContext(ctx, db, "migrations")
	case "status":
		return goose.StatusContext(ctx, db, "migrations")
	case "reset":
		return goose.ResetContext(ctx, db, "migrations")
	default:
		return fmt.Errorf("unknown migration direction %q", direction)
	}
}

func configureGoose() error {
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("configure goose dialect: %w", err)
	}
	return nil
}
