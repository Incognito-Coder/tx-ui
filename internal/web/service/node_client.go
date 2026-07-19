package service

import (
	"fmt"
	"strings"
	"time"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
	"x-ui/internal/logger"
	"x-ui/xray"

	"gorm.io/gorm"
)

// NodeClientService handles CRUD operations for NodeClient records.
// It works exclusively against the SQLite DB via GORM; no direct xray API calls
// are made from this service layer — only the xray restart flag is set.
type NodeClientService struct{}

// ---------------------------------------------------------------------------
// Read helpers
// ---------------------------------------------------------------------------

// GetAll returns all NodeClient records.
func (s *NodeClientService) GetAll() ([]*model.NodeClient, error) {
	db := database.GetDB()
	var clients []*model.NodeClient
	err := db.Model(model.NodeClient{}).Find(&clients).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return clients, nil
}

// GetByID returns a single NodeClient by primary key.
func (s *NodeClientService) GetByID(id int) (*model.NodeClient, error) {
	db := database.GetDB()
	nc := &model.NodeClient{}
	err := db.Model(model.NodeClient{}).First(nc, id).Error
	if err != nil {
		return nil, err
	}
	return nc, nil
}

// GetByEmail returns a single NodeClient looked up by email.
func (s *NodeClientService) GetByEmail(email string) (*model.NodeClient, error) {
	db := database.GetDB()
	nc := &model.NodeClient{}
	err := db.Model(model.NodeClient{}).Where("email = ?", email).First(nc).Error
	if err != nil {
		return nil, err
	}
	return nc, nil
}

// GetBySubID returns a single NodeClient looked up by SubID.
func (s *NodeClientService) GetBySubID(subId string) (*model.NodeClient, error) {
	db := database.GetDB()
	nc := &model.NodeClient{}
	err := db.Model(model.NodeClient{}).Where("sub_id = ?", subId).First(nc).Error
	if err != nil {
		return nil, err
	}
	return nc, nil
}

// ---------------------------------------------------------------------------
// Uniqueness helpers (scan both node_clients table and inbound settings JSON)
// ---------------------------------------------------------------------------

// getAllInboundEmails returns all client emails embedded in any inbound's
// settings.clients JSON array — mirrors InboundService.getAllEmails.
func (s *NodeClientService) getAllInboundEmails() ([]string, error) {
	db := database.GetDB()
	var emails []string
	err := db.Raw(`
		SELECT JSON_EXTRACT(client.value, '$.email')
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE inbounds.settings IS NOT NULL
		  AND inbounds.settings != ''
		  AND JSON_TYPE(inbounds.settings, '$.clients') = 'array'
	`).Scan(&emails).Error
	if err != nil {
		return nil, err
	}
	return emails, nil
}

// getAllInboundSubIDs returns all client subId values embedded in any inbound's
// settings.clients JSON array.
func (s *NodeClientService) getAllInboundSubIDs() ([]string, error) {
	db := database.GetDB()
	var subIds []string
	err := db.Raw(`
		SELECT JSON_EXTRACT(client.value, '$.subId')
		FROM inbounds,
			JSON_EACH(JSON_EXTRACT(inbounds.settings, '$.clients')) AS client
		WHERE inbounds.settings IS NOT NULL
		  AND inbounds.settings != ''
		  AND JSON_TYPE(inbounds.settings, '$.clients') = 'array'
		  AND JSON_EXTRACT(client.value, '$.subId') IS NOT NULL
		  AND JSON_EXTRACT(client.value, '$.subId') != ''
	`).Scan(&subIds).Error
	if err != nil {
		return nil, err
	}
	return subIds, nil
}

func containsIgnoreCase(slice []string, s string) bool {
	lower := strings.ToLower(s)
	for _, v := range slice {
		if strings.ToLower(v) == lower {
			return true
		}
	}
	return false
}

// checkEmailUnique verifies that the given email does not already exist in the
// node_clients table (excluding ignoreID > 0) or in any inbound's JSON clients.
func (s *NodeClientService) checkEmailUnique(email string, ignoreID int) error {
	if email == "" {
		return nil
	}
	db := database.GetDB()

	// Check node_clients table
	query := db.Model(model.NodeClient{}).Where("email = ?", email)
	if ignoreID > 0 {
		query = query.Where("id != ?", ignoreID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("email already exists in node clients: %s", email)
	}

	// Check inbound settings JSON arrays
	inboundEmails, err := s.getAllInboundEmails()
	if err != nil {
		return err
	}
	if containsIgnoreCase(inboundEmails, email) {
		return fmt.Errorf("email already exists in inbound clients: %s", email)
	}
	return nil
}

// checkSubIDUnique verifies that the given SubID does not already exist in the
// node_clients table (excluding ignoreID > 0) or in any inbound's JSON clients.
func (s *NodeClientService) checkSubIDUnique(subId string, ignoreID int) error {
	if subId == "" {
		return nil
	}
	db := database.GetDB()

	// Check node_clients table
	query := db.Model(model.NodeClient{}).Where("sub_id = ?", subId)
	if ignoreID > 0 {
		query = query.Where("id != ?", ignoreID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("subId already exists in node clients: %s", subId)
	}

	// Check inbound settings JSON arrays
	inboundSubIDs, err := s.getAllInboundSubIDs()
	if err != nil {
		return err
	}
	if containsIgnoreCase(inboundSubIDs, subId) {
		return fmt.Errorf("subId already exists in inbound clients: %s", subId)
	}
	return nil
}

// ---------------------------------------------------------------------------
// CRUD — Create
// ---------------------------------------------------------------------------

// Create validates uniqueness and persists a new NodeClient record.
// Requirements: 1.2, 1.3, 1.4, 1.5
func (s *NodeClientService) Create(nc *model.NodeClient) error {
	if err := s.checkEmailUnique(nc.Email, 0); err != nil {
		return err
	}
	if err := s.checkSubIDUnique(nc.SubID, 0); err != nil {
		return err
	}

	db := database.GetDB()
	return db.Create(nc).Error
}

// ---------------------------------------------------------------------------
// CRUD — Update
// ---------------------------------------------------------------------------

// Update validates uniqueness (excluding the current record) and persists
// changes. Flags xray for restart.
// Requirements: 8.1, 8.2
func (s *NodeClientService) Update(nc *model.NodeClient) error {
	if err := s.checkEmailUnique(nc.Email, nc.Id); err != nil {
		return err
	}
	if err := s.checkSubIDUnique(nc.SubID, nc.Id); err != nil {
		return err
	}

	db := database.GetDB()
	if err := db.Save(nc).Error; err != nil {
		return err
	}

	// Flag xray restart so updated credentials are reflected at next restart.
	isNeedXrayRestart.Store(true)
	return nil
}

// ---------------------------------------------------------------------------
// CRUD — Delete (single)
// ---------------------------------------------------------------------------

// Delete removes a NodeClient, all its NodeClientLink records, and NULL-outs
// the node_client_id column on associated ClientTraffic rows — all in a single
// transaction. Flags xray restart.
// Requirements: 8.3, 3.4
func (s *NodeClientService) Delete(id int) error {
	db := database.GetDB()

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := s.deleteInTx(tx, id); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	isNeedXrayRestart.Store(true)
	return nil
}

// deleteInTx performs the three-step deletion within a provided transaction.
// It is reused by both Delete and BulkDelete.
func (s *NodeClientService) deleteInTx(tx *gorm.DB, id int) error {
	// 1. Verify the NodeClient exists.
	nc := &model.NodeClient{}
	if err := tx.First(nc, id).Error; err != nil {
		return err
	}

	// 2. Delete all NodeClientLink rows for this NodeClient.
	if err := tx.Where("node_client_id = ?", id).Delete(&model.NodeClientLink{}).Error; err != nil {
		return fmt.Errorf("deleting node client links for id %d: %w", id, err)
	}

	// 3. NULL-out node_client_id on all associated ClientTraffic rows.
	if err := tx.Model(&xray.ClientTraffic{}).
		Where("node_client_id = ?", id).
		Update("node_client_id", nil).Error; err != nil {
		return fmt.Errorf("nulling node_client_id on client_traffics for id %d: %w", id, err)
	}

	// 4. Delete the NodeClient record itself.
	if err := tx.Delete(&model.NodeClient{}, id).Error; err != nil {
		return fmt.Errorf("deleting node client id %d: %w", id, err)
	}

	logger.Debugf("NodeClient %d deleted (links and traffic references cleaned up)", id)
	return nil
}

// ---------------------------------------------------------------------------
// CRUD — BulkDelete
// ---------------------------------------------------------------------------

// BulkDelete wraps individual delete logic in a single transaction. If any
// deletion fails, the entire operation is rolled back.
// Requirements: 8.4
func (s *NodeClientService) BulkDelete(ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, id := range ids {
		if err := s.deleteInTx(tx, id); err != nil {
			tx.Rollback()
			return fmt.Errorf("bulk delete failed at id %d: %w", id, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	isNeedXrayRestart.Store(true)
	return nil
}

// ---------------------------------------------------------------------------
// Link management
// ---------------------------------------------------------------------------

// AddLink creates a NodeClientLink between the given node client and inbound.
// It also ensures a ClientTraffic row exists for the (email, inboundId) pair
// with NodeClientId set. Flags xray restart on success.
// Requirements: 2.1, 2.2, 2.3, 4.1
func (s *NodeClientService) AddLink(nodeClientId, inboundId int, flow string) error {
	db := database.GetDB()

	// 1. Fetch the NodeClient — error if not found.
	nc := &model.NodeClient{}
	if err := db.First(nc, nodeClientId).Error; err != nil {
		return fmt.Errorf("node client not found (id=%d): %w", nodeClientId, err)
	}

	// 2. Check for duplicate (nodeClientId, inboundId) pair.
	var linkCount int64
	if err := db.Model(&model.NodeClientLink{}).
		Where("node_client_id = ? AND inbound_id = ?", nodeClientId, inboundId).
		Count(&linkCount).Error; err != nil {
		return err
	}
	if linkCount > 0 {
		return fmt.Errorf("link already exists for node client %d and inbound %d", nodeClientId, inboundId)
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 3. Create the NodeClientLink record.
	link := &model.NodeClientLink{
		NodeClientId: nodeClientId,
		InboundId:    inboundId,
		Flow:         flow,
	}
	if err := tx.Create(link).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("creating node client link: %w", err)
	}

	// 4. Upsert the ClientTraffic row.
	//    ClientTraffic is keyed by (email, inbound_id), so use FirstOrCreate on both.
	//    If the row already exists, update NodeClientId; otherwise create it fresh.
	ct := &xray.ClientTraffic{}
	result := tx.Where("email = ? AND inbound_id = ?", nc.Email, inboundId).First(ct)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		tx.Rollback()
		return fmt.Errorf("querying client traffic for email %s and inbound %d: %w", nc.Email, inboundId, result.Error)
	}

	if result.Error == gorm.ErrRecordNotFound {
		// Row does not exist — create it.
		ct = &xray.ClientTraffic{
			InboundId:    inboundId,
			Email:        nc.Email,
			Enable:       true,
			Up:           0,
			Down:         0,
			Total:        nc.TotalGB,
			ExpiryTime:   nc.ExpiryTime,
			Reset:        nc.Reset,
			NodeClientId: &nodeClientId,
		}
		if err := tx.Create(ct).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("creating client traffic for email %s and inbound %d: %w", nc.Email, inboundId, err)
		}
	} else {
		// Row exists — update NodeClientId if it is not already set.
		if ct.NodeClientId == nil {
			if err := tx.Model(ct).Update("node_client_id", nodeClientId).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("updating node_client_id on client traffic for email %s and inbound %d: %w", nc.Email, inboundId, err)
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// 5. Flag xray restart.
	isNeedXrayRestart.Store(true)
	return nil
}

// RemoveLink deletes the NodeClientLink for the given (nodeClientId, inboundId)
// pair and NULL-outs node_client_id on the corresponding ClientTraffic row
// (preserving the row for historical data). Flags xray restart on success.
// Requirements: 2.5, 4.4
func (s *NodeClientService) RemoveLink(nodeClientId, inboundId int) error {
	db := database.GetDB()

	// 1. Find the NodeClientLink.
	link := &model.NodeClientLink{}
	if err := db.Where("node_client_id = ? AND inbound_id = ?", nodeClientId, inboundId).
		First(link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("link not found for node client %d and inbound %d", nodeClientId, inboundId)
		}
		return err
	}

	// 2. Fetch the NodeClient to get its email.
	nc := &model.NodeClient{}
	if err := db.First(nc, nodeClientId).Error; err != nil {
		return fmt.Errorf("node client not found (id=%d): %w", nodeClientId, err)
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 3. Delete the NodeClientLink record.
	if err := tx.Delete(link).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("deleting node client link: %w", err)
	}

	// 4. NULL-out node_client_id on the ClientTraffic row (preserve the row).
	if err := tx.Model(&xray.ClientTraffic{}).
		Where("email = ? AND node_client_id = ?", nc.Email, nodeClientId).
		Update("node_client_id", nil).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("nulling node_client_id on client traffic for email %s: %w", nc.Email, err)
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// 5. Flag xray restart.
	isNeedXrayRestart.Store(true)
	return nil
}

// ---------------------------------------------------------------------------
// Traffic aggregation
// ---------------------------------------------------------------------------

func aggregateTrafficRows(rows []xray.ClientTraffic, totalGB int64, expiryTime int64, nodeClientId int) *xray.ClientTraffic {
	trafficByEmail := make(map[string]*xray.ClientTraffic, len(rows))
	for _, row := range rows {
		if row.Email == "" {
			continue
		}
		existing, ok := trafficByEmail[row.Email]
		if !ok || row.Up+row.Down > existing.Up+existing.Down {
			copied := row
			trafficByEmail[row.Email] = &copied
		}
	}

	var totalUp, totalDown int64
	var email string
	for _, row := range trafficByEmail {
		totalUp += row.Up
		totalDown += row.Down
		if email == "" {
			email = row.Email
		}
	}

	return &xray.ClientTraffic{
		Email:        email,
		Up:           totalUp,
		Down:         totalDown,
		Total:        totalGB,
		ExpiryTime:   expiryTime,
		NodeClientId: &nodeClientId,
	}
}

// GetAggregatedTraffic queries all ClientTraffic rows with the given node_client_id,
// dedupes rows by email, and returns a single aggregated *xray.ClientTraffic.
// If the node client is linked to multiple inbounds, the same email may appear
// across multiple rows in the DB, so only one representative row per email is
// used when calculating used traffic.
// Requirements: 4.5, 6.3
func (s *NodeClientService) GetAggregatedTraffic(nodeClientId int, txs ...*gorm.DB) (*xray.ClientTraffic, error) {
	db := database.GetDB()
	if len(txs) > 0 && txs[0] != nil {
		db = txs[0]
	}

	// Fetch the NodeClient to get its ExpiryTime and TotalGB
	nc := &model.NodeClient{}
	if err := db.First(nc, nodeClientId).Error; err != nil {
		return nil, err
	}

	var rows []xray.ClientTraffic
	if err := db.Where("node_client_id = ?", nodeClientId).Find(&rows).Error; err != nil {
		return nil, err
	}

	return aggregateTrafficRows(rows, nc.TotalGB, nc.ExpiryTime, nodeClientId), nil
}

// ResetTraffic zeros out Up and Down on all ClientTraffic rows for the given node client
// inside a single transaction. Sets isNeedXrayRestart to true on success.
// Returns (needsRestart=true, nil) on success.
// Requirements: 4.5, 6.3
func (s *NodeClientService) ResetTraffic(nodeClientId int) (bool, error) {
	db := database.GetDB()

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&xray.ClientTraffic{}).
		Where("node_client_id = ?", nodeClientId).
		Updates(map[string]interface{}{"up": 0, "down": 0}).Error; err != nil {
		tx.Rollback()
		return false, err
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	isNeedXrayRestart.Store(true)
	return true, nil
}

// IsNodeClientEmail queries the node_clients table to determine whether the given
// email belongs to a node client. Returns (true, nil) if a matching record exists.
// Requirements: 6.3
func (s *NodeClientService) IsNodeClientEmail(email string) (bool, error) {
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.NodeClient{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ---------------------------------------------------------------------------
// Link queries
// ---------------------------------------------------------------------------

// GetLinks returns all NodeClientLink records for the given node client.
// Requirements: 2.2
func (s *NodeClientService) GetLinks(nodeClientId int) ([]*model.NodeClientLink, error) {
	db := database.GetDB()
	var links []*model.NodeClientLink
	err := db.Where("node_client_id = ?", nodeClientId).Find(&links).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return links, nil
}

// GetLinkedInbounds returns all Inbound records linked to the given node client,
// joining through the node_client_links table.
// Requirements: 2.7
func (s *NodeClientService) GetLinkedInbounds(nodeClientId int) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(&model.Inbound{}).
		Joins("JOIN node_client_links ON node_client_links.inbound_id = inbounds.id").
		Where("node_client_links.node_client_id = ?", nodeClientId).
		Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

// ---------------------------------------------------------------------------
// Lifecycle hooks (called by InboundService jobs)
// ---------------------------------------------------------------------------

// DisableExhausted evaluates all enabled NodeClients and disables those whose
// aggregated traffic exceeds their TotalGB quota or whose ExpiryTime has passed.
// Returns (changed=true, nil) if any node client was updated.
// Requirements: 7.1
func (s *NodeClientService) DisableExhausted(txs ...*gorm.DB) (bool, error) {
	db := database.GetDB()
	if len(txs) > 0 && txs[0] != nil {
		db = txs[0]
	}

	// Get all enabled NodeClients
	var nodeClients []*model.NodeClient
	if err := db.Where("enable = ?", true).Find(&nodeClients).Error; err != nil {
		return false, err
	}

	if len(nodeClients) == 0 {
		return false, nil
	}

	now := time.Now().Unix() * 1000
	var idsToDisable []int

	for _, nc := range nodeClients {
		shouldDisable := false

		// Check expiry
		if nc.ExpiryTime > 0 && nc.ExpiryTime <= now {
			shouldDisable = true
			logger.Debugf("NodeClient %d (%s) expired at %d (now=%d)", nc.Id, nc.Email, nc.ExpiryTime, now)
		}

		// Check traffic quota
		if !shouldDisable && nc.TotalGB > 0 {
			aggregated, err := s.GetAggregatedTraffic(nc.Id, txs...)
			if err != nil {
				logger.Warningf("Failed to get aggregated traffic for NodeClient %d: %v", nc.Id, err)
				continue
			}
			totalUsed := aggregated.Up + aggregated.Down
			if totalUsed >= nc.TotalGB {
				shouldDisable = true
				logger.Debugf("NodeClient %d (%s) exhausted: %d >= %d", nc.Id, nc.Email, totalUsed, nc.TotalGB)
			}
		}

		if shouldDisable {
			idsToDisable = append(idsToDisable, nc.Id)
		}
	}

	if len(idsToDisable) == 0 {
		return false, nil
	}

	// Disable all exhausted node clients in one query
	if err := db.Model(&model.NodeClient{}).Where("id IN ?", idsToDisable).Update("enable", false).Error; err != nil {
		return false, err
	}

	logger.Debugf("Disabled %d exhausted node clients", len(idsToDisable))
	return true, nil
}

// AutoRenew processes all enabled NodeClients with Reset > 0. For each client whose
// reset interval has elapsed (ExpiryTime <= now), it advances the ExpiryTime by the
// reset interval and zeros out Up/Down on all linked ClientTraffic rows.
// Mirrors the logic from InboundService.autoRenewClients.
// Requirements: 7.2, 7.3
func (s *NodeClientService) AutoRenew(txs ...*gorm.DB) error {
	db := database.GetDB()
	if len(txs) > 0 && txs[0] != nil {
		db = txs[0]
	}
	now := time.Now().Unix() * 1000

	// Find all enabled NodeClients with Reset > 0 and ExpiryTime <= now
	var nodeClients []*model.NodeClient
	err := db.Where("enable = ? AND reset > 0 AND expiry_time > 0 AND expiry_time <= ?", true, now).Find(&nodeClients).Error
	if err != nil {
		return err
	}

	if len(nodeClients) == 0 {
		return nil
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, nc := range nodeClients {
		// Advance ExpiryTime by the reset interval until it's in the future
		newExpiryTime := nc.ExpiryTime
		resetInterval := int64(nc.Reset) * 86400000 // days to milliseconds
		for newExpiryTime < now {
			newExpiryTime += resetInterval
		}

		// Update NodeClient ExpiryTime
		if err := tx.Model(&model.NodeClient{}).Where("id = ?", nc.Id).Update("expiry_time", newExpiryTime).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("updating expiry_time for NodeClient %d: %w", nc.Id, err)
		}

		// Zero out Up/Down on all linked ClientTraffic rows
		if err := tx.Model(&xray.ClientTraffic{}).
			Where("node_client_id = ?", nc.Id).
			Updates(map[string]interface{}{"up": 0, "down": 0, "expiry_time": newExpiryTime}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("resetting traffic for NodeClient %d: %w", nc.Id, err)
		}

		logger.Debugf("AutoRenewed NodeClient %d (%s): new expiry=%d", nc.Id, nc.Email, newExpiryTime)
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	logger.Debugf("AutoRenewed %d node clients", len(nodeClients))
	return nil
}

// ---------------------------------------------------------------------------
// Xray config synthesis
// ---------------------------------------------------------------------------

// synthesiseClient maps a NodeClient + NodeClientLink into a model.Client that
// can be embedded in an inbound's settings.clients array.
//
// Field mapping:
//   - Email      → nc.Email
//   - ID         → nc.UUID        (used for vmess/vless)
//   - Password   → nc.Password    (used for trojan/shadowsocks)
//   - Auth       → nc.Auth        (used for hysteria)
//   - Security   → nc.Security
//   - Flow       → link.Flow if non-empty, else nc.Flow
//   - TotalGB    → nc.TotalGB
//   - ExpiryTime → nc.ExpiryTime
//   - LimitIP    → nc.LimitIP
//   - TgID       → nc.TgID
//   - Enable     → nc.Enable
//   - Reset      → nc.Reset
//   - Comment    → nc.Comment
//   - SubID      → nc.SubID
//
// Requirements: 3.2, 8.5 (Property 5)
func (s *NodeClientService) synthesiseClient(nc *model.NodeClient, link *model.NodeClientLink) model.Client {
	flow := link.Flow
	if flow == "" {
		flow = nc.Flow
	}

	return model.Client{
		ID:         nc.UUID,
		Password:   nc.Password,
		Auth:       nc.Auth,
		Security:   nc.Security,
		Flow:       flow,
		Email:      nc.Email,
		TotalGB:    nc.TotalGB,
		ExpiryTime: nc.ExpiryTime,
		LimitIP:    nc.LimitIP,
		TgID:       nc.TgID,
		Enable:     nc.Enable,
		Reset:      nc.Reset,
		Comment:    nc.Comment,
		SubID:      nc.SubID,
	}
}

// MergeIntoInboundConfig fetches all enabled NodeClientLinks for the given
// inboundId, synthesises a Client entry for each, detects email collisions
// (logs a warning and skips the offending link), and returns the merged slice.
//
// An "email collision" occurs when a synthesised client's email already appears
// in existingClients OR has already been produced by a previous synthesised
// client in this same call (duplicate node-client emails on the same inbound).
//
// The returned slice is existingClients extended with the synthesised entries;
// the original existingClients slice is never mutated.
//
// Requirements: 3.1, 3.2, 3.3, 3.5 (Properties 3, 5, 6)
func (s *NodeClientService) MergeIntoInboundConfig(inboundId int, existingClients []model.Client) ([]model.Client, error) {
	db := database.GetDB()

	// Fetch all NodeClientLinks for this inbound that belong to an enabled NodeClient.
	// We join node_clients to filter on enable = true in one query.
	type linkWithNC struct {
		model.NodeClientLink
		model.NodeClient
	}

	var rows []linkWithNC
	err := db.Table("node_client_links").
		Select("node_client_links.*, node_clients.*").
		Joins("JOIN node_clients ON node_clients.id = node_client_links.node_client_id").
		Where("node_client_links.inbound_id = ? AND node_clients.enable = ?", inboundId, true).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("fetching node client links for inbound %d: %w", inboundId, err)
	}

	// Build a set of emails already present in existingClients for fast collision detection.
	seenEmails := make(map[string]struct{}, len(existingClients))
	for _, c := range existingClients {
		if c.Email != "" {
			seenEmails[strings.ToLower(c.Email)] = struct{}{}
		}
	}

	// Start with a copy of existingClients so we never mutate the caller's slice.
	merged := make([]model.Client, len(existingClients), len(existingClients)+len(rows))
	copy(merged, existingClients)

	for _, row := range rows {
		nc := row.NodeClient
		link := row.NodeClientLink

		emailKey := strings.ToLower(nc.Email)
		if _, collision := seenEmails[emailKey]; collision {
			logger.Warningf(
				"NodeClientService.MergeIntoInboundConfig: email collision for %q "+
					"(node_client_id=%d, inbound_id=%d) — skipping link",
				nc.Email, link.NodeClientId, inboundId,
			)
			continue
		}

		seenEmails[emailKey] = struct{}{}
		merged = append(merged, s.synthesiseClient(&nc, &link))
	}

	return merged, nil
}
