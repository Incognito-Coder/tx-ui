//go:build !cgo

package database

import (
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openSQLite(dbPath string) gorm.Dialector {
	dsn := dbPath
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	if !strings.Contains(dsn, "_pragma") {
		dsn += sep + "_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	}
	return sqlite.Open(dsn)
}

func openSQLiteSimple(dbPath string) gorm.Dialector {
	return sqlite.Open(dbPath)
}
