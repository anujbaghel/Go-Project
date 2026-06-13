package db

import (
	"Go-project/migrations"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func RunMigrations(dbURL string) error {
	sqlDb, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}

	defer sqlDb.Close()

	// goose reading embedded files
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqlDb, ".") // "." = root of the embedded FS
}
