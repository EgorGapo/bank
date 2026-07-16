package db

import (
	"embed"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func SetupPostgres(pool *pgxpool.Pool) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "up":
		err = goose.Up(db, "migrations")
	case "down":
		err = goose.Down(db, "migrations")
	case "status":
		err = goose.Status(db, "migrations")
	}
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
