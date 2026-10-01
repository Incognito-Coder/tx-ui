package service

import (
	"encoding/json"
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

// ClientDetail bundles a NodeClient with its active links and aggregated traffic.
type ClientDetail struct {
	model.NodeClient
	Links   []model.NodeClientLink `json:"links"`
	Traffic *xray.ClientTraffic   `json:"traffic"`
}

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

// GetAllWithDetails returns all NodeClient records with their links and traffic populated.
func (s *NodeClientService) GetAllWithDetails() ([]*ClientDetail, error) {
	db := database.GetDB()
	var clients []*model.NodeClient
	err := db.Model(model.NodeClient{}).Order("id asc").Find(&clients).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	var allLinks []model.NodeClientLink
	_ = db.Find(&allLinks).Error

	linksByClient := make(map[int][]model.NodeClientLink, len(allLinks))
	for _, l := range allLinks {
		linksByClient[l.NodeClientId] = append(linksByClient[l.NodeClientId], l)
	}

	var allTraffics []xray.ClientTraffic
	_ = db.Find(&allTraffics).Error

	trafficsByNodeClientId := make(map[int][]xray.ClientTraffic)
	trafficsByEmail := make(map[string][]xray.ClientTraffic)
	for _, t := range allTraffics {
		if t.NodeClientId != nil && *t.NodeClientId > 0 {
			trafficsByNodeClientId[*t.NodeClientId] = append(trafficsByNodeClientId[*t.NodeClientId], t)
		}
		if t.Email != "" {
			le := strings.ToLower(t.Email)
			trafficsByEmail[le] = append(trafficsByEmail[le], t)
		}
	}

	results := make([]*ClientDetail, 0, len(clients))
	for _, nc := range clients {
		links := linksByClient[nc.Id]
		if links == nil {
			links = []model.NodeClientLink{}
		}

		var rows []xray.ClientTraffic
		seenTrafficIds := make(map[int]bool)
		if ncRows, ok := trafficsByNodeClientId[nc.Id]; ok {
			for _, r := range ncRows {
				if !seenTrafficIds[r.Id] {
					seenTrafficIds[r.Id] = true
					rows = append(rows, r)
				}
			}
		}
		if nc.Email != "" {
			if emailRows, ok := trafficsByEmail[strings.ToLower(nc.Email)]; ok {
				for _, r := range emailRows {
					if !seenTrafficIds[r.Id] {
						seenTrafficIds[r.Id] = true
						rows = append(rows, r)
					}
				}
			}
		}

		traffic := aggregateTrafficRows(rows, nc.TotalGB, nc.ExpiryTime, nc.Id)
		if traffic.Email == "" {
			traffic.Email = nc.Email
		}

		results = append(results, &ClientDetail{
			NodeClient: *nc,
			Links:      links,
			Traffic:    traffic,
		})
	}
	return results, nil
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
// Uniqueness helpers
// ---------------------------------------------------------------------------

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
// node_clients table (excluding ignoreID > 0).
func (s *NodeClientService) checkEmailUnique(email string, ignoreID int) error {
	if email == "" {
		return nil
	}
	db := database.GetDB()

	query := db.Model(model.NodeClient{}).Where("LOWER(email) = LOWER(?)", email)
	if ignoreID > 0 {
		query = query.Where("id != ?", ignoreID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("email already exists: %s", email)
	}
	return nil
}

// checkSubIDUnique verifies that the given SubID does not already exist in the
// node_clients table (excluding ignoreID > 0).
func (s *NodeClientService) checkSubIDUnique(subId string, ignoreID int) error {
	if subId == "" {
		return nil
	}
	db := database.GetDB()

	query := db.Model(model.NodeClient{}).Where("sub_id = ?", subId)
	if ignoreID > 0 {
		query = query.Where("id != ?", ignoreID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("subId already exists: %s", subId)
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
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := tx.Save(nc).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Keep each linked traffic row's node-client metadata in sync while
	// preserving its accumulated upload and download counters.
	trafficUpdates := map[string]interface{}{
		"email":       nc.Email,
		"total":       nc.TotalGB,
		"expiry_time": nc.ExpiryTime,
		"reset":       nc.Reset,
	}
	now := time.Now().Unix() * 1000
	if !nc.Enable {
		trafficUpdates["enable"] = false
	} else {
		isExhausted := false
		if nc.ExpiryTime > 0 && nc.ExpiryTime <= now {
			isExhausted = true
		}
		if !isExhausted && nc.TotalGB > 0 {
			aggregated, _ := s.GetAggregatedTraffic(nc.Id, tx)
			if aggregated != nil && (aggregated.Up+aggregated.Down) >= nc.TotalGB {
				isExhausted = true
			}
		}
		trafficUpdates["enable"] = !isExhausted
	}

	if err := tx.Model(&xray.ClientTraffic{}).
		Where("node_client_id = ?", nc.Id).
		Updates(trafficUpdates).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
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
		// Row exists — attach it and synchronize the node-client limits while
		// preserving its accumulated upload and download counters.
		if err := tx.Model(ct).Updates(map[string]interface{}{
			"node_client_id": nodeClientId,
			"total":          nc.TotalGB,
			"expiry_time":    nc.ExpiryTime,
			"reset":          nc.Reset,
		}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("updating client traffic for email %s and inbound %d: %w", nc.Email, inboundId, err)
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
	var maxUp, maxDown int64
	var email string
	for _, row := range rows {
		if row.Email != "" && email == "" {
			email = row.Email
		}
		if row.Up > maxUp {
			maxUp = row.Up
		}
		if row.Down > maxDown {
			maxDown = row.Down
		}
	}

	now := time.Now().Unix() * 1000
	isActive := true
	if expiryTime > 0 && expiryTime <= now {
		isActive = false
	}
	if totalGB > 0 && (maxUp+maxDown) >= totalGB {
		isActive = false
	}

	return &xray.ClientTraffic{
		Email:        email,
		Up:           maxUp,
		Down:         maxDown,
		Total:        totalGB,
		ExpiryTime:   expiryTime,
		Enable:       isActive,
		NodeClientId: &nodeClientId,
	}
}

// GetAggregatedTraffic queries all ClientTraffic rows with the given node_client_id or email,
// dedupes rows, and returns a single aggregated *xray.ClientTraffic.
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
	if err := db.Where("node_client_id = ? OR LOWER(email) = LOWER(?)", nodeClientId, nc.Email).Find(&rows).Error; err != nil {
		return nil, err
	}

	traffic := aggregateTrafficRows(rows, nc.TotalGB, nc.ExpiryTime, nodeClientId)
	if traffic.Email == "" {
		traffic.Email = nc.Email
	}
	return traffic, nil
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

	updates := map[string]interface{}{"up": 0, "down": 0}
	var nc model.NodeClient
	if err := tx.First(&nc, nodeClientId).Error; err == nil {
		now := time.Now().Unix() * 1000
		if nc.Enable && (nc.ExpiryTime <= 0 || nc.ExpiryTime > now) {
			updates["enable"] = true
		}
	}

	if err := tx.Model(&xray.ClientTraffic{}).
		Where("node_client_id = ?", nodeClientId).
		Updates(updates).Error; err != nil {
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

	// Only disable linked client_traffics rows so every inbound reflects the exhaustion in Xray.
	// We do NOT update node_clients.enable to false, because the switch represents manual deactivation;
	// exhausted/expired clients are disconnected at the network/core level while keeping the switch on.
	res := db.Model(&xray.ClientTraffic{}).Where("node_client_id IN ? AND enable = ?", idsToDisable, true).Update("enable", false)
	if res.Error != nil {
		return false, res.Error
	}

	logger.Debugf("Marked %d exhausted node client traffic rows disabled", res.RowsAffected)
	return res.RowsAffected > 0, nil
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

		// Zero out Up/Down on all linked ClientTraffic rows and re-enable
		if err := tx.Model(&xray.ClientTraffic{}).
			Where("node_client_id = ?", nc.Id).
			Updates(map[string]interface{}{"up": 0, "down": 0, "expiry_time": newExpiryTime, "enable": true}).Error; err != nil {
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
		LinkFlow   string `gorm:"column:link_flow"`
		ClientID   int    `gorm:"column:client_id"`
		Email      string
		SubID      string
		UUID       string
		Password   string
		Auth       string
		Security   string
		ClientFlow string `gorm:"column:client_flow"`
		TotalGB    int64
		ExpiryTime int64
		LimitIP    int
		TgID       int64
		Enable     bool
		Reset      int
		Comment    string
	}
	if db == nil {
		return existingClients, nil
	}
	var rows []linkWithNC
	err := db.Table("node_client_links").
		Select(`
			node_client_links.flow AS link_flow,
			node_clients.id AS client_id,
			node_clients.email,
			node_clients.sub_id,
			node_clients.uuid,
			node_clients.password,
			node_clients.auth,
			node_clients.security,
			node_clients.flow AS client_flow,
			node_clients.total_gb,
			node_clients.expiry_time,
			node_clients.limit_ip,
			node_clients.tg_id,
			node_clients.enable,
			node_clients.reset,
			node_clients.comment`).
		Joins("JOIN node_clients ON node_clients.id = node_client_links.node_client_id").
		Where("node_client_links.inbound_id = ? AND node_clients.enable = ?", inboundId, true).
		Order("node_clients.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("fetching node client links for inbound %d: %w", inboundId, err)
	}

	merged := make([]model.Client, 0, len(existingClients)+len(rows))
	seenEmails := make(map[string]struct{}, len(existingClients)+len(rows))
	seenIDs := make(map[string]struct{}, len(existingClients)+len(rows))

	now := time.Now().Unix() * 1000
	// 1. Add all active node clients first (NodeClient is primary source of truth)
	for _, row := range rows {
		// Skip expired node clients
		if row.ExpiryTime > 0 && row.ExpiryTime <= now {
			continue
		}

		// Skip traffic-exhausted node clients
		if row.TotalGB > 0 {
			var totalTraffic struct {
				Up   int64
				Down int64
			}
			err := db.Table("client_traffics").
				Select("COALESCE(MAX(up), 0) as up, COALESCE(MAX(down), 0) as down").
				Where("node_client_id = ? OR email = ?", row.ClientID, row.Email).
				Scan(&totalTraffic).Error
			if err == nil && (totalTraffic.Up+totalTraffic.Down) >= row.TotalGB {
				continue
			}
		}

		nc := model.NodeClient{
			Id:         row.ClientID,
			Email:      row.Email,
			SubID:      row.SubID,
			UUID:       row.UUID,
			Password:   row.Password,
			Auth:       row.Auth,
			Security:   row.Security,
			Flow:       row.ClientFlow,
			TotalGB:    row.TotalGB,
			ExpiryTime: row.ExpiryTime,
			LimitIP:    row.LimitIP,
			TgID:       row.TgID,
			Enable:     row.Enable,
			Reset:      row.Reset,
			Comment:    row.Comment,
		}
		link := model.NodeClientLink{Flow: row.LinkFlow}
		client := s.synthesiseClient(&nc, &link)

		emailKey := strings.ToLower(strings.TrimSpace(client.Email))
		idKey := strings.ToLower(strings.TrimSpace(client.ID))
		if emailKey != "" {
			if _, ok := seenEmails[emailKey]; ok {
				continue
			}
			seenEmails[emailKey] = struct{}{}
		}
		if idKey != "" {
			if _, ok := seenIDs[idKey]; ok {
				continue
			}
			seenIDs[idKey] = struct{}{}
		}
		merged = append(merged, client)
	}

	// 2. Add existingClients from settings JSON that are not already covered by node clients
	for _, c := range existingClients {
		emailKey := strings.ToLower(strings.TrimSpace(c.Email))
		idKey := strings.ToLower(strings.TrimSpace(c.ID))
		if emailKey != "" {
			if _, ok := seenEmails[emailKey]; ok {
				continue
			}
		}
		if idKey != "" {
			if _, ok := seenIDs[idKey]; ok {
				continue
			}
		}
		if emailKey != "" {
			seenEmails[emailKey] = struct{}{}
		}
		if idKey != "" {
			seenIDs[idKey] = struct{}{}
		}
		merged = append(merged, c)
	}

	return merged, nil
}

// ---------------------------------------------------------------------------
// Extended Client Operations
// ---------------------------------------------------------------------------

type NodeClientLinkInput struct {
	InboundId int    `json:"inboundId" form:"inboundId"`
	Flow      string `json:"flow"      form:"flow"`
}

// SetLinks replaces all links for a client with the provided list.
func (s *NodeClientService) SetLinks(nodeClientId int, links []NodeClientLinkInput) error {
	db := database.GetDB()
	nc := &model.NodeClient{}
	if err := db.First(nc, nodeClientId).Error; err != nil {
		return fmt.Errorf("client not found (id=%d): %w", nodeClientId, err)
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var currentLinks []model.NodeClientLink
	if err := tx.Where("node_client_id = ?", nodeClientId).Find(&currentLinks).Error; err != nil {
		tx.Rollback()
		return err
	}

	newInboundMap := make(map[int]string, len(links))
	for _, l := range links {
		newInboundMap[l.InboundId] = l.Flow
	}

	// Remove links not in new set
	for _, cl := range currentLinks {
		if _, keep := newInboundMap[cl.InboundId]; !keep {
			if err := tx.Delete(&cl).Error; err != nil {
				tx.Rollback()
				return err
			}
			_ = tx.Model(&xray.ClientTraffic{}).
				Where("LOWER(email) = LOWER(?) AND node_client_id = ? AND inbound_id = ?", nc.Email, nodeClientId, cl.InboundId).
				Update("node_client_id", nil).Error
		}
	}

	// Add or update links
	for _, l := range links {
		var existing model.NodeClientLink
		err := tx.Where("node_client_id = ? AND inbound_id = ?", nodeClientId, l.InboundId).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			newLink := model.NodeClientLink{
				NodeClientId: nodeClientId,
				InboundId:    l.InboundId,
				Flow:         l.Flow,
			}
			if err := tx.Create(&newLink).Error; err != nil {
				tx.Rollback()
				return err
			}

			var ct xray.ClientTraffic
			res := tx.Where("LOWER(email) = LOWER(?) AND inbound_id = ?", nc.Email, l.InboundId).First(&ct)
			if res.Error == gorm.ErrRecordNotFound {
				ct = xray.ClientTraffic{
					InboundId:    l.InboundId,
					Email:        nc.Email,
					Enable:       true,
					Up:           0,
					Down:         0,
					Total:        nc.TotalGB,
					ExpiryTime:   nc.ExpiryTime,
					Reset:        nc.Reset,
					NodeClientId: &nodeClientId,
				}
				_ = tx.Create(&ct).Error
			} else if res.Error == nil {
				_ = tx.Model(&ct).Updates(map[string]interface{}{
					"node_client_id": nodeClientId,
					"total":          nc.TotalGB,
					"expiry_time":    nc.ExpiryTime,
					"reset":          nc.Reset,
				}).Error
			}
		} else if err == nil {
			if existing.Flow != l.Flow {
				existing.Flow = l.Flow
				_ = tx.Save(&existing).Error
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	isNeedXrayRestart.Store(true)
	return nil
}

// BulkCreate creates multiple clients and links each to the specified inbounds.
func (s *NodeClientService) BulkCreate(clients []model.NodeClient, inboundIds []int) error {
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i := range clients {
		nc := &clients[i]
		if err := s.checkEmailUnique(nc.Email, 0); err != nil {
			tx.Rollback()
			return err
		}
		if err := s.checkSubIDUnique(nc.SubID, 0); err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Create(nc).Error; err != nil {
			tx.Rollback()
			return err
		}

		for _, inId := range inboundIds {
			link := model.NodeClientLink{
				NodeClientId: nc.Id,
				InboundId:    inId,
				Flow:         nc.Flow,
			}
			if err := tx.Create(&link).Error; err != nil {
				tx.Rollback()
				return err
			}

			ct := xray.ClientTraffic{
				InboundId:    inId,
				Email:        nc.Email,
				Enable:       true,
				Up:           0,
				Down:         0,
				Total:        nc.TotalGB,
				ExpiryTime:   nc.ExpiryTime,
				Reset:        nc.Reset,
				NodeClientId: &nc.Id,
			}
			if err := tx.Create(&ct).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	isNeedXrayRestart.Store(true)
	return nil
}

// ResetAllTraffics resets upload and download counters on all clients.
func (s *NodeClientService) ResetAllTraffics() error {
	db := database.GetDB()
	if err := db.Model(&xray.ClientTraffic{}).
		Where("node_client_id IS NOT NULL").
		Updates(map[string]interface{}{"up": 0, "down": 0}).Error; err != nil {
		return err
	}
	now := time.Now().Unix() * 1000
	var enabledClientIds []int
	db.Model(&model.NodeClient{}).
		Where("enable = ? AND (expiry_time <= 0 OR expiry_time > ?)", true, now).
		Pluck("id", &enabledClientIds)
	if len(enabledClientIds) > 0 {
		db.Model(&xray.ClientTraffic{}).
			Where("node_client_id IN ?", enabledClientIds).
			Update("enable", true)
	}
	isNeedXrayRestart.Store(true)
	return nil
}

// DeleteDepleted deletes all clients whose quota is exhausted or expiry time has passed.
func (s *NodeClientService) DeleteDepleted() error {
	db := database.GetDB()
	var clients []*model.NodeClient
	if err := db.Find(&clients).Error; err != nil {
		return err
	}
	now := time.Now().Unix() * 1000
	var idsToDelete []int
	for _, nc := range clients {
		if nc.ExpiryTime > 0 && nc.ExpiryTime <= now {
			idsToDelete = append(idsToDelete, nc.Id)
			continue
		}
		if nc.TotalGB > 0 {
			agg, err := s.GetAggregatedTraffic(nc.Id)
			if err == nil && (agg.Up+agg.Down) >= nc.TotalGB {
				idsToDelete = append(idsToDelete, nc.Id)
			}
		}
	}
	if len(idsToDelete) > 0 {
		return s.BulkDelete(idsToDelete)
	}
	return nil
}

// GetLinkedInboundsCounts returns a map of inboundId -> linked clients count.
func (s *NodeClientService) GetLinkedInboundsCounts() (map[int]int, error) {
	db := database.GetDB()
	type countRow struct {
		InboundId int `gorm:"column:inbound_id"`
		Count     int `gorm:"column:count"`
	}
	var rows []countRow
	err := db.Table("node_client_links").
		Select("inbound_id, count(*) as count").
		Group("inbound_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	res := make(map[int]int, len(rows))
	for _, r := range rows {
		res[r.InboundId] = r.Count
	}
	return res, nil
}

// MigrateLegacyClients scans all inbounds for clients embedded in settings JSON.
// Any client that does not yet exist in node_clients is imported into node_clients,
// linked to its inbound in node_client_links, and its client_traffics row is associated.
func (s *NodeClientService) MigrateLegacyClients() error {
	db := database.GetDB()
	if db == nil {
		return nil
	}

	var inbounds []*model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}

	for _, inbound := range inbounds {
		if inbound.Settings == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &raw); err != nil {
			continue
		}
		clientsRaw, ok := raw["clients"]
		if !ok || clientsRaw == nil {
			continue
		}
		clientsList, ok := clientsRaw.([]interface{})
		if !ok || len(clientsList) == 0 {
			continue
		}

		for _, item := range clientsList {
			cMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			email, _ := cMap["email"].(string)
			if email == "" {
				continue
			}

			// Check if NodeClient with this email already exists
			var existingNC model.NodeClient
			err := db.Where("LOWER(email) = LOWER(?)", email).First(&existingNC).Error
			if err == gorm.ErrRecordNotFound {
				subId, _ := cMap["subId"].(string)
				id, _ := cMap["id"].(string)
				password, _ := cMap["password"].(string)
				auth, _ := cMap["auth"].(string)
				security, _ := cMap["security"].(string)
				flow, _ := cMap["flow"].(string)
				comment, _ := cMap["comment"].(string)

				var totalGB int64
				if v, ok := cMap["totalGB"].(float64); ok {
					totalGB = int64(v)
				}
				var expiryTime int64
				if v, ok := cMap["expiryTime"].(float64); ok {
					expiryTime = int64(v)
				}
				var limitIp int
				if v, ok := cMap["limitIp"].(float64); ok {
					limitIp = int(v)
				}
				var tgId int64
				if v, ok := cMap["tgId"].(float64); ok {
					tgId = int64(v)
				}
				var reset int
				if v, ok := cMap["reset"].(float64); ok {
					reset = int(v)
				}
				enable := true
				if v, ok := cMap["enable"].(bool); ok {
					enable = v
				}

				newNC := model.NodeClient{
					Email:      email,
					SubID:      subId,
					UUID:       id,
					Password:   password,
					Auth:       auth,
					Security:   security,
					Flow:       flow,
					TotalGB:    totalGB,
					ExpiryTime: expiryTime,
					LimitIP:    limitIp,
					TgID:       tgId,
					Enable:     enable,
					Reset:      reset,
					Comment:    comment,
				}
				if err := db.Create(&newNC).Error; err != nil {
					logger.Warningf("MigrateLegacyClients: failed to create client %s: %v", email, err)
					continue
				}
				existingNC = newNC
			} else if err != nil {
				continue
			}

			// Ensure link exists
			var linkCount int64
			db.Model(&model.NodeClientLink{}).
				Where("node_client_id = ? AND inbound_id = ?", existingNC.Id, inbound.Id).
				Count(&linkCount)
			if linkCount == 0 {
				flow, _ := cMap["flow"].(string)
				link := model.NodeClientLink{
					NodeClientId: existingNC.Id,
					InboundId:    inbound.Id,
					Flow:         flow,
				}
				_ = db.Create(&link).Error
			}

			// Associate existing ClientTraffic row with node_client_id
			_ = db.Model(&xray.ClientTraffic{}).
				Where("email = ? AND inbound_id = ? AND (node_client_id IS NULL OR node_client_id = 0)", email, inbound.Id).
				Update("node_client_id", existingNC.Id).Error
		}
	}
	return nil
}
