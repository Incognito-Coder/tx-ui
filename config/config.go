package config

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed version
var version string

//go:embed name
var name string

type LogLevel string

const (
	Debug  LogLevel = "debug"
	Info   LogLevel = "info"
	Notice LogLevel = "notice"
	Warn   LogLevel = "warn"
	Error  LogLevel = "error"
)

func GetVersion() string {
	return strings.TrimSpace(version)
}

func GetName() string {
	return strings.TrimSpace(name)
}

func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("XUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return os.Getenv("XUI_DEBUG") == "true"
}

func GetBinFolderPath() string {
	binFolderPath := os.Getenv("XUI_BIN_FOLDER")
	if binFolderPath == "" {
		binFolderPath = "bin"
	}
	return binFolderPath
}

func GetDBFolderPath() string {
	dbFolderPath := os.Getenv("XUI_DB_FOLDER")
	if dbFolderPath == "" {
		dbFolderPath = "/etc/x-ui"
	}
	return dbFolderPath
}

func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

func GetDBType() DBType {
	t := strings.ToLower(strings.TrimSpace(os.Getenv("XUI_DB_TYPE")))
	if t == "postgres" || t == "postgresql" || t == "pg" {
		return DBTypePostgres
	}
	return DBTypeSQLite
}

func GetPostgresDSN() string {
	dsn := os.Getenv("XUI_DB_POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn != "" {
		return dsn
	}

	host := os.Getenv("XUI_DB_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("XUI_DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("XUI_DB_USER")
	if user == "" {
		user = "postgres"
	}
	password := os.Getenv("XUI_DB_PASSWORD")
	dbname := os.Getenv("XUI_DB_NAME")
	if dbname == "" {
		dbname = "xui"
	}
	sslmode := os.Getenv("XUI_DB_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}
	timezone := os.Getenv("XUI_DB_TIMEZONE")
	if timezone == "" {
		timezone = "UTC"
	}

	if password != "" {
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
			host, port, user, password, dbname, sslmode, timezone)
	}
	return fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s TimeZone=%s",
		host, port, user, dbname, sslmode, timezone)
}

func GetLogFolder() string {
	logFolderPath := os.Getenv("XUI_LOG_FOLDER")
	if logFolderPath == "" {
		logFolderPath = "/var/log"
	}
	return logFolderPath
}
