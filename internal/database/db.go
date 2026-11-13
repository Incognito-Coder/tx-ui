package database

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/internal/database/model"
	"x-ui/internal/util/crypto"
	"x-ui/xray"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db            *gorm.DB
	currentDBPath string
)

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
	defaultSecret   = ""
)

func initModels() error {
	models := []interface{}{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&model.NodeClient{},
		&model.NodeClientLink{},
		&model.ClientDevice{},
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

func initUser() error {
	empty, err := isTableEmpty("users")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}
	if empty {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)

		if err != nil {
			log.Printf("Error hashing default password: %v", err)
			return err
		}

		user := &model.User{
			Username:        defaultUsername,
			Password:        hashedPassword,
			LoginSecret:     defaultSecret,
			Role:            "admin",
			Permissions:     "*",
			AllowedInbounds: "*",
			CanResetTraffic: true,
			Status:          "active",
		}
		return db.Create(user).Error
	}
	return nil
}

func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	} else {
		var seedersHistory []string
		if err := db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory).Error; err != nil {
			return err
		}

		if !slices.Contains(seedersHistory, "UserPasswordHash") && !isUsersEmpty {
			var users []model.User
			if err := db.Find(&users).Error; err != nil {
				return err
			}

			for _, user := range users {
				hashedPassword, err := crypto.HashPasswordAsBcrypt(user.Password)
				if err != nil {
					log.Printf("Error hashing password for user '%s': %v", user.Username, err)
					return err
				}
				if err := db.Model(&user).Update("password", hashedPassword).Error; err != nil {
					return err
				}
			}

			hashSeeder := &model.HistoryOfSeeders{
				SeederName: "UserPasswordHash",
			}
			return db.Create(hashSeeder).Error
		}
	}

	return nil
}

func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

func InitDB(dbPath string) error {
	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}

	if config.GetDBType() == config.DBTypePostgres {
		dsn := config.GetPostgresDSN()
		currentDBPath = dsn
		var err error
		db, err = gorm.Open(postgres.Open(dsn), c)
		if err != nil {
			return err
		}

		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		// Connection pooling for PostgreSQL
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		dir := path.Dir(dbPath)
		err := os.MkdirAll(dir, fs.ModePerm)
		if err != nil {
			return err
		}

		currentDBPath = dbPath
		db, err = gorm.Open(openSQLite(dbPath), c)
		if err != nil {
			db, err = gorm.Open(openSQLiteSimple(dbPath), c)
			if err != nil {
				return err
			}
		}

		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		// Configure connection pool for SQLite to prevent starvation and lock contention
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Hour)

		// Enable WAL mode, set a 10s busy timeout, and use NORMAL synchronous mode for high throughput without lock contention.
		db.Exec("PRAGMA journal_mode = WAL;")
		db.Exec("PRAGMA busy_timeout = 10000;")
		db.Exec("PRAGMA synchronous = NORMAL;")
	}

	if err := initModels(); err != nil {
		return err
	}

	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
		return err
	}

	if err := initUser(); err != nil {
		return err
	}
	return runSeeders(isUsersEmpty)
}

func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func ValidateSQLiteDB(filePath string) error {
	c := &gorm.Config{
		Logger: logger.Discard,
	}

	tempDB, err := gorm.Open(openSQLiteSimple(filePath), c)
	if err != nil {
		return fmt.Errorf("failed to open database file: %w", err)
	}

	sqlDB, err := tempDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql handle: %w", err)
	}
	defer sqlDB.Close()

	// Integrity check to ensure pages and tree structures are valid
	var integrityResult string
	if err := tempDB.Raw("PRAGMA integrity_check(1);").Scan(&integrityResult).Error; err != nil {
		return fmt.Errorf("integrity check query failed: %w", err)
	}
	if !strings.EqualFold(integrityResult, "ok") {
		return fmt.Errorf("database integrity check failed: %s", integrityResult)
	}

	// Verify that at least one of the essential x-ui tables exists
	var tableCount int64
	if err := tempDB.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('inbounds', 'settings', 'users');").Scan(&tableCount).Error; err != nil {
		return fmt.Errorf("failed to inspect database tables: %w", err)
	}
	if tableCount == 0 {
		return fmt.Errorf("database does not contain valid x-ui tables (inbounds, settings, or users)")
	}

	// Truncate any WAL created during open/read so the file remains clean
	_ = tempDB.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
	return nil
}

func Checkpoint() error {
	if config.GetDBType() == config.DBTypePostgres {
		return nil
	}
	// Update WAL
	err := db.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return err
	}
	return nil
}

func IsPostgres() bool {
	return config.GetDBType() == config.DBTypePostgres
}
