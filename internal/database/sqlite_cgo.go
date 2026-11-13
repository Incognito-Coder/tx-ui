//go:build cgo

package database

import (
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSQLite(dbPath string) gorm.Dialector {
	dsn := dbPath
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	if !strings.Contains(dsn, "_busy_timeout") {
		dsn += sep + "_journal_mode=WAL&_busy_timeout=10000&_synchronous=NORMAL&_txlock=immediate"
	}
	return sqlite.Open(dsn)
}

func openSQLiteSimple(dbPath string) gorm.Dialector {
	return sqlite.Open(dbPath)
}
