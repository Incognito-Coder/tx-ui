package database

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"x-ui/internal/database/model"
	"x-ui/xray"

	"gorm.io/gorm"
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

	// Checkpoint WAL after cleanup
	_ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error

	return result, nil
}
