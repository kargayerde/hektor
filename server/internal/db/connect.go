package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var DEFAULT_DB_NAME = ".abobarelaydb"
var DB *sqlx.DB

func Connect(ctx context.Context) *sqlx.DB {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	exeDir := filepath.Dir(exePath)
	dbPath := filepath.Join(exeDir, DEFAULT_DB_NAME)

	DB, err = sqlx.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	if err := DB.Ping(); err != nil {
		panic(err)
	}
	if _, err := DB.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		panic(err)
	}
	DB.MustExec(`
	CREATE TABLE IF NOT EXISTS relays (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		relay_index INTEGER UNIQUE,
		label TEXT
	)`)

	tx := DB.MustBegin()
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO relays (relay_index, label) VALUES (?, ?)`)
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	for i := 1; i < 9; i++ {
		if _, err := stmt.Exec(i, fmt.Sprintf("relolo-%d", i)); err != nil {
			tx.Rollback()
			panic(err)
		}
	}

	if err := tx.Commit(); err != nil {
		panic(err)
	}

	return DB
}
