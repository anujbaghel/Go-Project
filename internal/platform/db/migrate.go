package db

import (
	"database/sql"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func RunMigrations(dbURL string, fsys fs.FS, versionTable string) error {
	sqlDb, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}

	defer sqlDb.Close()

	// goose reading embedded files
	goose.SetBaseFS(fsys)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if versionTable != "" {
		goose.SetTableName(versionTable) // isolate each service's migration history
	}
	return goose.Up(sqlDb, ".") // "." = root of the embedded FS
}
