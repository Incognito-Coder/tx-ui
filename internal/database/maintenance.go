package database

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"x-ui/config"
	"x-ui/internal/database/model"
	"x-ui/internal/util/crypto"
	"x-ui/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DBStatus struct {
	Path               string           `json:"path"`
	Size               int64            `json:"size"`
	SizeFormatted      string           `json:"sizeFormatted"`
	WalSize            int64            `json:"walSize"`
	WalSizeFormatted   string           `json:"walSizeFormatted"`
	TotalSize          int64            `json:"totalSize"`
	TotalSizeFormatted string           `json:"totalSizeFormatted"`
	IntegrityCheck     string           `json:"integrityCheck"`
	CorruptedRows      int64            `json:"corruptedRows"`
	Details            map[string]int64 `json:"details"`
}

type CleanResult struct {
	CleanedTraffics int64  `json:"cleanedTraffics"`
	CleanedLinks    int64  `json:"cleanedLinks"`
	CleanedClients  int64  `json:"cleanedClients"`
	CleanedIPs      int64  `json:"cleanedIPs"`
	TotalCleaned    int64  `json:"totalCleaned"`
	Message         string `json:"message"`
}

type VacuumResult struct {
	BeforeSize         int64  `json:"beforeSize"`
	BeforeSizeFormatted string `json:"beforeSizeFormatted"`
	AfterSize          int64  `json:"afterSize"`
	AfterSizeFormatted  string `json:"afterSizeFormatted"`
	ReclaimedBytes     int64  `json:"reclaimedBytes"`
	ReclaimedFormatted string `json:"reclaimedFormatted"`
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func GetCurrentDBPath() string {
	return currentDBPath
}

func GetDBStatus() (*DBStatus, error) {
	status := &DBStatus{
		Path:    currentDBPath,
		Details: make(map[string]int64),
	}

	if config.GetDBType() == config.DBTypePostgres {
		var dbSize int64
		if err := db.Raw("SELECT pg_database_size(current_database());").Scan(&dbSize).Error; err == nil {
			status.Size = dbSize
		}
		status.TotalSize = status.Size
		status.SizeFormatted = FormatBytes(status.Size)
		status.TotalSizeFormatted = FormatBytes(status.TotalSize)
		status.IntegrityCheck = "ok (PostgreSQL)"
	} else {
		if currentDBPath != "" {
			if fi, err := os.Stat(currentDBPath); err == nil {
				status.Size = fi.Size()
			}
			walPath := currentDBPath + "-wal"
			if fi, err := os.Stat(walPath); err == nil {
				status.WalSize = fi.Size()
			}
		}

		status.TotalSize = status.Size + status.WalSize
		status.SizeFormatted = FormatBytes(status.Size)
		status.WalSizeFormatted = FormatBytes(status.WalSize)
		status.TotalSizeFormatted = FormatBytes(status.TotalSize)

		// Run PRAGMA integrity_check
		var integrityResult string
		err := db.Raw("PRAGMA integrity_check(1);").Scan(&integrityResult).Error
		if err != nil {
			status.IntegrityCheck = fmt.Sprintf("Error: %v", err)
		} else if integrityResult == "" {
			status.IntegrityCheck = "ok"
		} else {
			status.IntegrityCheck = integrityResult
		}
	}

	// Scan corrupted / orphaned rows
	counts, err := ScanCorruptedRows()
	if err == nil {
		status.Details = counts
		for _, count := range counts {
			status.CorruptedRows += count
		}
	}

	return status, nil
}

func Optimize() error {
	if config.GetDBType() == config.DBTypePostgres {
		return ExecWithRetry(5, func() error {
			return db.Exec("ANALYZE;").Error
		})
	}
	return ExecWithRetry(5, func() error {
		if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
			return err
		}
		if err := db.Exec("PRAGMA optimize;").Error; err != nil {
			return err
		}
		if err := db.Exec("ANALYZE;").Error; err != nil {
			return err
		}
		return db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
	})
}

func Vacuum() (*VacuumResult, error) {
	res := &VacuumResult{}

	if config.GetDBType() == config.DBTypePostgres {
		var beforeSize int64
		_ = db.Raw("SELECT pg_database_size(current_database());").Scan(&beforeSize)
		res.BeforeSize = beforeSize
		res.BeforeSizeFormatted = FormatBytes(beforeSize)

		err := ExecWithRetry(5, func() error {
			return db.Exec("VACUUM;").Error
		})
		if err != nil {
			return nil, err
		}

		var afterSize int64
		_ = db.Raw("SELECT pg_database_size(current_database());").Scan(&afterSize)
		res.AfterSize = afterSize
		res.AfterSizeFormatted = FormatBytes(afterSize)

		if beforeSize > afterSize {
			res.ReclaimedBytes = beforeSize - afterSize
		}
		res.ReclaimedFormatted = FormatBytes(res.ReclaimedBytes)
		return res, nil
	}

	// Measure sizes before
	if currentDBPath != "" {
		if fi, err := os.Stat(currentDBPath); err == nil {
			res.BeforeSize += fi.Size()
		}
		if fi, err := os.Stat(currentDBPath + "-wal"); err == nil {
			res.BeforeSize += fi.Size()
		}
	}
	res.BeforeSizeFormatted = FormatBytes(res.BeforeSize)

	err := ExecWithRetry(5, func() error {
		_ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
		if err := db.Exec("VACUUM;").Error; err != nil {
			return err
		}
		return db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
	})
	if err != nil {
		return nil, err
	}

	// Measure sizes after
	if currentDBPath != "" {
		if fi, err := os.Stat(currentDBPath); err == nil {
			res.AfterSize += fi.Size()
		}
		if fi, err := os.Stat(currentDBPath + "-wal"); err == nil {
			res.AfterSize += fi.Size()
		}
	}
	res.AfterSizeFormatted = FormatBytes(res.AfterSize)
	if res.BeforeSize >= res.AfterSize {
		res.ReclaimedBytes = res.BeforeSize - res.AfterSize
	}
	res.ReclaimedFormatted = FormatBytes(res.ReclaimedBytes)

	return res, nil
}

func ScanCorruptedRows() (map[string]int64, error) {
	counts := make(map[string]int64)

	// 1. Orphaned client_traffics: missing inbound or empty email
	var orphanedTraffics int64
	db.Model(&xray.ClientTraffic{}).
		Where("TRIM(email) = '' OR email IS NULL OR (inbound_id > 0 AND inbound_id NOT IN (SELECT id FROM inbounds))").
		Count(&orphanedTraffics)
	counts["orphaned_traffics"] = orphanedTraffics

	// 2. Orphaned node_client_links: missing node client or missing inbound
	var orphanedLinks int64
	db.Model(&model.NodeClientLink{}).
		Where("node_client_id NOT IN (SELECT id FROM node_clients) OR inbound_id NOT IN (SELECT id FROM inbounds)").
		Count(&orphanedLinks)
	counts["orphaned_links"] = orphanedLinks

	// 3. Duplicate node_client_links
	var duplicateLinks int64
	db.Model(&model.NodeClientLink{}).
		Where("id NOT IN (SELECT MIN(id) FROM node_client_links GROUP BY node_client_id, inbound_id)").
		Count(&duplicateLinks)
	counts["duplicate_links"] = duplicateLinks

	// 4. Corrupted node_clients: empty email
	var corruptedClients int64
	db.Model(&model.NodeClient{}).
		Where("TRIM(email) = '' OR email IS NULL").
		Count(&corruptedClients)
	counts["corrupted_clients"] = corruptedClients

	// 5. Corrupted/orphaned inbound_client_ips: empty email or orphan
	var corruptedIPs int64
	db.Model(&model.InboundClientIps{}).
		Where("TRIM(client_email) = '' OR client_email IS NULL").
		Count(&corruptedIPs)
	counts["corrupted_ips"] = corruptedIPs

	// 6. Ghost client_traffics: records whose email does not belong to any valid client
	ghostTraffics := countGhostTraffics()
	counts["ghost_traffics"] = ghostTraffics

	return counts, nil
}

func countGhostTraffics() int64 {
	validEmails := collectAllValidEmails()
	if len(validEmails) == 0 {
		return 0
	}

	var count int64
	// In SQLite, query traffics where email not in validEmails
	var traffics []xray.ClientTraffic
	if err := db.Select("id, email").Find(&traffics).Error; err != nil {
		return 0
	}

	for _, t := range traffics {
		trimmed := strings.ToLower(strings.TrimSpace(t.Email))
		if trimmed != "" && !validEmails[trimmed] {
			count++
		}
	}
	return count
}

func collectAllValidEmails() map[string]bool {
	valid := make(map[string]bool)

	// From node_clients
	var ncEmails []string
	db.Model(&model.NodeClient{}).Pluck("email", &ncEmails)
	for _, e := range ncEmails {
		trimmed := strings.ToLower(strings.TrimSpace(e))
		if trimmed != "" {
			valid[trimmed] = true
		}
	}

	// From inbounds settings JSON
	var inbounds []model.Inbound
	db.Select("id, settings").Find(&inbounds)
	for _, ib := range inbounds {
		if ib.Settings == "" {
			continue
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		for _, key := range []string{"clients", "peers"} {
			if list, ok := settings[key].([]any); ok {
				for _, item := range list {
					if c, ok := item.(map[string]any); ok {
						if em, ok := c["email"].(string); ok {
							trimmed := strings.ToLower(strings.TrimSpace(em))
							if trimmed != "" {
								valid[trimmed] = true
							}
						}
					}
				}
			}
		}
	}

	return valid
}

func CleanCorruptedRows() (*CleanResult, error) {
	result := &CleanResult{}

	err := ExecWithRetry(5, func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			// 1. Delete client_traffics with empty email or invalid inbound
			res1 := tx.Where("TRIM(email) = '' OR email IS NULL OR (inbound_id > 0 AND inbound_id NOT IN (SELECT id FROM inbounds))").
				Delete(&xray.ClientTraffic{})
			if res1.Error != nil {
				return res1.Error
			}
			result.CleanedTraffics += res1.RowsAffected

			// 2. Fix broken node_client_id references in client_traffics
			_ = tx.Model(&xray.ClientTraffic{}).
				Where("node_client_id IS NOT NULL AND node_client_id > 0 AND node_client_id NOT IN (SELECT id FROM node_clients)").
				Update("node_client_id", nil).Error

			// 3. Delete ghost client_traffics (not in any inbound settings or node_clients)
			validEmails := collectAllValidEmails()
			if len(validEmails) > 0 {
				var allTraffics []xray.ClientTraffic
				if err := tx.Select("id, email").Find(&allTraffics).Error; err == nil {
					var ghostIds []int
					for _, t := range allTraffics {
						trimmed := strings.ToLower(strings.TrimSpace(t.Email))
						if trimmed != "" && !validEmails[trimmed] {
							ghostIds = append(ghostIds, t.Id)
						}
					}
					if len(ghostIds) > 0 {
						resG := tx.Where("id IN ?", ghostIds).Delete(&xray.ClientTraffic{})
						if resG.Error == nil {
							result.CleanedTraffics += resG.RowsAffected
						}
					}
				}
			}

			// 4. Delete orphaned node_client_links
			res2 := tx.Where("node_client_id NOT IN (SELECT id FROM node_clients) OR inbound_id NOT IN (SELECT id FROM inbounds)").
				Delete(&model.NodeClientLink{})
			if res2.Error != nil {
				return res2.Error
			}
			result.CleanedLinks += res2.RowsAffected

			// 5. Delete duplicate node_client_links
			res3 := tx.Where("id NOT IN (SELECT MIN(id) FROM node_client_links GROUP BY node_client_id, inbound_id)").
				Delete(&model.NodeClientLink{})
			if res3.Error != nil {
				return res3.Error
			}
			result.CleanedLinks += res3.RowsAffected

			// 6. Delete corrupted node_clients (empty email)
			res4 := tx.Where("TRIM(email) = '' OR email IS NULL").
				Delete(&model.NodeClient{})
			if res4.Error != nil {
				return res4.Error
			}
			result.CleanedClients += res4.RowsAffected

			// 7. Delete orphaned / empty inbound_client_ips
			res5 := tx.Where("TRIM(client_email) = '' OR client_email IS NULL").
				Delete(&model.InboundClientIps{})
			if res5.Error != nil {
				return res5.Error
			}
			result.CleanedIPs += res5.RowsAffected

			// Delete IPs whose email is neither in client_traffics nor node_clients
			if len(validEmails) > 0 {
				var allIps []model.InboundClientIps
				if err := tx.Select("id, client_email").Find(&allIps).Error; err == nil {
					var orphanIpIds []int
					for _, ip := range allIps {
						trimmed := strings.ToLower(strings.TrimSpace(ip.ClientEmail))
						if trimmed != "" && !validEmails[trimmed] {
							orphanIpIds = append(orphanIpIds, ip.Id)
						}
					}
					if len(orphanIpIds) > 0 {
						resOrphIP := tx.Where("id IN ?", orphanIpIds).Delete(&model.InboundClientIps{})
						if resOrphIP.Error == nil {
							result.CleanedIPs += resOrphIP.RowsAffected
						}
					}
				}
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	result.TotalCleaned = result.CleanedTraffics + result.CleanedLinks + result.CleanedClients + result.CleanedIPs
	result.Message = fmt.Sprintf("Successfully cleaned %d corrupted/orphaned rows (%d traffic, %d links, %d clients, %d IPs)",
		result.TotalCleaned, result.CleanedTraffics, result.CleanedLinks, result.CleanedClients, result.CleanedIPs)

	// Checkpoint WAL after cleanup if SQLite
	if config.GetDBType() != config.DBTypePostgres {
		_ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
	}

	return result, nil
}

func FactoryReset() error {
	return ExecWithRetry(5, func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			// 1. Delete all client traffic records
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&xray.ClientTraffic{}).Error; err != nil {
				return err
			}
			// 2. Delete all outbound traffics
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.OutboundTraffics{}).Error; err != nil {
				return err
			}
			// 3. Delete all inbound client IPs
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.InboundClientIps{}).Error; err != nil {
				return err
			}
			// 4. Delete all node client links and node clients
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.NodeClientLink{}).Error; err != nil {
				return err
			}
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.NodeClient{}).Error; err != nil {
				return err
			}
			// 5. Delete all client devices
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.ClientDevice{}).Error; err != nil {
				return err
			}
			// 6. Delete all inbounds
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.Inbound{}).Error; err != nil {
				return err
			}
			// 7. Delete all settings
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.Setting{}).Error; err != nil {
				return err
			}
			// 8. Delete all seeders
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
				return err
			}
			// 9. Delete all users
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.User{}).Error; err != nil {
				return err
			}

			// 10. Re-create default admin user
			hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)
			if err != nil {
				return err
			}

			defaultAdmin := &model.User{
				Username:        defaultUsername,
				Password:        hashedPassword,
				LoginSecret:     defaultSecret,
				Role:            "admin",
				Permissions:     "*",
				AllowedInbounds: "*",
				CanResetTraffic: true,
				Status:          "active",
			}
			if err := tx.Create(defaultAdmin).Error; err != nil {
				return err
			}

			// 11. Mark password hash seeder as completed
			hashSeeder := &model.HistoryOfSeeders{
				SeederName: "UserPasswordHash",
			}
			return tx.Create(hashSeeder).Error
		})
	})
}

func ImportSQLiteToPostgres(sqlitePath string) error {
	c := &gorm.Config{
		Logger: logger.Discard,
	}
	srcDB, err := gorm.Open(openSQLiteSimple(sqlitePath), c)
	if err != nil {
		return fmt.Errorf("failed to open source sqlite database: %w", err)
	}
	srcSql, err := srcDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sqlite handle: %w", err)
	}
	defer srcSql.Close()

	return ExecWithRetry(5, func() error {
		return db.Transaction(func(tx *gorm.DB) error {
			// Clear existing data before import
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.ClientDevice{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.NodeClientLink{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.NodeClient{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&xray.ClientTraffic{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.InboundClientIps{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.OutboundTraffics{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.Inbound{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.Setting{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.HistoryOfSeeders{})
			tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Where("1 = 1").Delete(&model.User{})

			// Copy Users
			var users []model.User
			if err := srcDB.Find(&users).Error; err == nil && len(users) > 0 {
				if err := tx.Create(&users).Error; err != nil {
					return fmt.Errorf("failed to import users: %w", err)
				}
			}

			// Copy Inbounds
			var inbounds []model.Inbound
			if err := srcDB.Find(&inbounds).Error; err == nil && len(inbounds) > 0 {
				if err := tx.Create(&inbounds).Error; err != nil {
					return fmt.Errorf("failed to import inbounds: %w", err)
				}
			}

			// Copy Settings
			var settings []model.Setting
			if err := srcDB.Find(&settings).Error; err == nil && len(settings) > 0 {
				if err := tx.Create(&settings).Error; err != nil {
					return fmt.Errorf("failed to import settings: %w", err)
				}
			}

			// Copy OutboundTraffics
			var outbounds []model.OutboundTraffics
			if err := srcDB.Find(&outbounds).Error; err == nil && len(outbounds) > 0 {
				if err := tx.Create(&outbounds).Error; err != nil {
					return fmt.Errorf("failed to import outbound traffics: %w", err)
				}
			}

			// Copy InboundClientIps
			var clientIps []model.InboundClientIps
			if err := srcDB.Find(&clientIps).Error; err == nil && len(clientIps) > 0 {
				if err := tx.Create(&clientIps).Error; err != nil {
					return fmt.Errorf("failed to import client ips: %w", err)
				}
			}

			// Copy ClientTraffic
			var traffics []xray.ClientTraffic
			if err := srcDB.Find(&traffics).Error; err == nil && len(traffics) > 0 {
				if err := tx.Create(&traffics).Error; err != nil {
					return fmt.Errorf("failed to import client traffics: %w", err)
				}
			}

			// Copy HistoryOfSeeders
			var seeders []model.HistoryOfSeeders
			if err := srcDB.Find(&seeders).Error; err == nil && len(seeders) > 0 {
				if err := tx.Create(&seeders).Error; err != nil {
					return fmt.Errorf("failed to import seeders: %w", err)
				}
			}

			// Copy NodeClient
			var nodeClients []model.NodeClient
			if err := srcDB.Find(&nodeClients).Error; err == nil && len(nodeClients) > 0 {
				if err := tx.Create(&nodeClients).Error; err != nil {
					return fmt.Errorf("failed to import node clients: %w", err)
				}
			}

			// Copy NodeClientLink
			var nodeClientLinks []model.NodeClientLink
			if err := srcDB.Find(&nodeClientLinks).Error; err == nil && len(nodeClientLinks) > 0 {
				if err := tx.Create(&nodeClientLinks).Error; err != nil {
					return fmt.Errorf("failed to import node client links: %w", err)
				}
			}

			// Copy ClientDevice
			var clientDevices []model.ClientDevice
			if err := srcDB.Find(&clientDevices).Error; err == nil && len(clientDevices) > 0 {
				if err := tx.Create(&clientDevices).Error; err != nil {
					return fmt.Errorf("failed to import client devices: %w", err)
				}
			}

			// Sync sequences in PostgreSQL
			tablesWithSeq := []string{"users", "inbounds", "settings", "client_traffics", "outbound_traffics", "inbound_client_ips", "history_of_seeders", "node_clients", "node_client_links", "client_devices"}
			for _, t := range tablesWithSeq {
				_ = tx.Exec(fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s', 'id'), coalesce(max(id), 1), max(id) IS NOT NULL) FROM \"%s\";", t, t)).Error
			}

			return nil
		})
	})
}

func ExportPostgresToSQLite(targetPath string) error {
	_ = os.Remove(targetPath)
	_ = os.Remove(targetPath + "-wal")
	_ = os.Remove(targetPath + "-shm")

	c := &gorm.Config{Logger: logger.Discard}
	dstDB, err := gorm.Open(openSQLiteSimple(targetPath), c)
	if err != nil {
		return fmt.Errorf("failed to create destination sqlite database: %w", err)
	}
	dstSql, err := dstDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sqlite handle: %w", err)
	}
	defer dstSql.Close()

	// Auto-migrate models into the destination SQLite database
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
	for _, m := range models {
		if err := dstDB.AutoMigrate(m); err != nil {
			return fmt.Errorf("failed to auto-migrate table in export: %w", err)
		}
	}

	// Copy data from Postgres into SQLite
	var users []model.User
	if err := db.Find(&users).Error; err == nil && len(users) > 0 {
		_ = dstDB.Create(&users).Error
	}
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err == nil && len(inbounds) > 0 {
		_ = dstDB.Create(&inbounds).Error
	}
	var settings []model.Setting
	if err := db.Find(&settings).Error; err == nil && len(settings) > 0 {
		_ = dstDB.Create(&settings).Error
	}
	var outbounds []model.OutboundTraffics
	if err := db.Find(&outbounds).Error; err == nil && len(outbounds) > 0 {
		_ = dstDB.Create(&outbounds).Error
	}
	var clientIps []model.InboundClientIps
	if err := db.Find(&clientIps).Error; err == nil && len(clientIps) > 0 {
		_ = dstDB.Create(&clientIps).Error
	}
	var traffics []xray.ClientTraffic
	if err := db.Find(&traffics).Error; err == nil && len(traffics) > 0 {
		_ = dstDB.Create(&traffics).Error
	}
	var seeders []model.HistoryOfSeeders
	if err := db.Find(&seeders).Error; err == nil && len(seeders) > 0 {
		_ = dstDB.Create(&seeders).Error
	}
	var nodeClients []model.NodeClient
	if err := db.Find(&nodeClients).Error; err == nil && len(nodeClients) > 0 {
		_ = dstDB.Create(&nodeClients).Error
	}
	var nodeClientLinks []model.NodeClientLink
	if err := db.Find(&nodeClientLinks).Error; err == nil && len(nodeClientLinks) > 0 {
		_ = dstDB.Create(&nodeClientLinks).Error
	}
	var clientDevices []model.ClientDevice
	if err := db.Find(&clientDevices).Error; err == nil && len(clientDevices) > 0 {
		_ = dstDB.Create(&clientDevices).Error
	}

	_ = dstDB.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error
	return nil
}
