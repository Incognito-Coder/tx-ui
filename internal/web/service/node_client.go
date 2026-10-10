package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
	_ = db.Table("node_client_links").
		Joins("JOIN inbounds ON inbounds.id = node_client_links.inbound_id").
		Select("node_client_links.*").
		Find(&allLinks).Error

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
// changes. If credentials or enable status changed, it hot-syncs with running Xray
// without restarting Xray unless WireGuard or API error.
// Requirements: 8.1, 8.2
func (s *NodeClientService) Update(nc *model.NodeClient) (bool, error) {
	if err := s.checkEmailUnique(nc.Email, nc.Id); err != nil {
		return false, err
	}
	if err := s.checkSubIDUnique(nc.SubID, nc.Id); err != nil {
		return false, err
	}

	db := database.GetDB()

	// Capture previous state to check if credentials or enable status changed
	oldNC := &model.NodeClient{}
	hasOld := false
	if err := db.First(oldNC, nc.Id).Error; err == nil {
		hasOld = true
	}

	tx := db.Begin()
	if tx.Error != nil {
		return false, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := tx.Save(nc).Error; err != nil {
		tx.Rollback()
		return false, err
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
		return false, err
	}

	// Keep settings JSON in sync for linked inbounds if credentials, enable, or metadata (expiry, quota, etc.) changed
	credsChanged := hasOld && (oldNC.Email != nc.Email || oldNC.UUID != nc.UUID || oldNC.Password != nc.Password || oldNC.Auth != nc.Auth || oldNC.Flow != nc.Flow)
	enableChanged := hasOld && (oldNC.Enable != nc.Enable)
	metaChanged := hasOld && (oldNC.ExpiryTime != nc.ExpiryTime || oldNC.TotalGB != nc.TotalGB || oldNC.Reset != nc.Reset || oldNC.SubID != nc.SubID || oldNC.LimitIP != nc.LimitIP || oldNC.TgID != nc.TgID || oldNC.Comment != nc.Comment)

	links, _ := s.GetLinks(nc.Id)
	if credsChanged || enableChanged || metaChanged || !hasOld {
		for _, l := range links {
			var inbound model.Inbound
			if err := tx.First(&inbound, l.InboundId).Error; err == nil {
				flow := l.Flow
				if flow == "" {
					flow = nc.Flow
				}
				removeClientFromInboundSettings(&inbound, oldNC.Email, oldNC.UUID, oldNC.Password, oldNC.Auth)
				if nc.Enable {
					addClientToInboundSettings(&inbound, nc, flow)
				}
				_ = tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	// If credentials or enable state changed, hot-sync with running Xray without dropping other users' connections.
	needRestart := false
	if hasOld && (credsChanged || enableChanged) {
		if p != nil && p.IsRunning() {
			var xrayApi xray.XrayAPI
			if err := xrayApi.Init(p.GetAPIPort()); err != nil {
				needRestart = true
			} else {
				defer xrayApi.Close()
				for _, l := range links {
					var inbound model.Inbound
					if err := db.First(&inbound, l.InboundId).Error; err == nil {
						if s.hotRemoveUserFromInbound(&xrayApi, &inbound, oldNC.Email) {
							needRestart = true
						}
						if nc.Enable {
							flow := l.Flow
							if flow == "" {
								flow = nc.Flow
							}
							if s.hotAddUserToInbound(&xrayApi, &inbound, nc, flow) {
								needRestart = true
							}
						}
					}
				}
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
}

// ---------------------------------------------------------------------------
// CRUD — Delete (single)
// ---------------------------------------------------------------------------

// Delete removes a NodeClient, all its NodeClientLink records, and NULL-outs
// the node_client_id column on associated ClientTraffic rows — all in a single
// transaction. Hot-removes users from running Xray if active.
// Requirements: 8.3, 3.4
func (s *NodeClientService) Delete(id int) (bool, error) {
	db := database.GetDB()

	// 1. Find all links before deletion so we can hot-remove from Xray
	var links []model.NodeClientLink
	_ = db.Where("node_client_id = ?", id).Find(&links).Error

	nc := &model.NodeClient{}
	_ = db.First(nc, id).Error

	err := database.ExecWithRetry(5, func() error {
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

		return tx.Commit().Error
	})
	if err != nil {
		return false, err
	}

	needRestart := false
	if p != nil && p.IsRunning() && nc.Email != "" && len(links) > 0 {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()
			for _, l := range links {
				var inbound model.Inbound
				if err := db.First(&inbound, l.InboundId).Error; err == nil {
					if s.hotRemoveUserFromInbound(&xrayApi, &inbound, nc.Email) {
						needRestart = true
					}
				}
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
}

// removeClientFromInboundSettings removes any matching client from an inbound's settings JSON ("clients" and "peers").
// Returns true if any client was removed.
func removeClientFromInboundSettings(inbound *model.Inbound, email, uuid, password, auth string) bool {
	if inbound.Settings == "" {
		return false
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return false
	}
	modified := false
	for _, key := range []string{"clients", "peers"} {
		raw, ok := settings[key]
		if !ok || raw == nil {
			continue
		}
		list, ok := raw.([]interface{})
		if !ok || len(list) == 0 {
			continue
		}
		newList := make([]interface{}, 0, len(list))
		for _, item := range list {
			c, ok := item.(map[string]interface{})
			if !ok {
				newList = append(newList, item)
				continue
			}
			cEmail, _ := c["email"].(string)
			cId, _ := c["id"].(string)
			cPass, _ := c["password"].(string)
			cAuth, _ := c["auth"].(string)
			cKey, _ := c["publicKey"].(string)

			isMatch := false
			if email != "" && cEmail != "" && strings.EqualFold(strings.TrimSpace(cEmail), strings.TrimSpace(email)) {
				isMatch = true
			} else if uuid != "" && cId != "" && strings.EqualFold(strings.TrimSpace(cId), strings.TrimSpace(uuid)) {
				isMatch = true
			} else if password != "" && cPass != "" && cPass == password {
				isMatch = true
			} else if auth != "" && cAuth != "" && cAuth == auth {
				isMatch = true
			} else if uuid != "" && cKey != "" && strings.EqualFold(strings.TrimSpace(cKey), strings.TrimSpace(uuid)) {
				isMatch = true
			}

			if isMatch {
				modified = true
			} else {
				newList = append(newList, item)
			}
		}
		settings[key] = newList
	}

	if modified {
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err == nil {
			inbound.Settings = string(newSettings)
			return true
		}
	}
	return false
}

// addClientToInboundSettings adds a client to an inbound's settings JSON ("clients" or "peers") if not present.
// Returns true if modified.
func addClientToInboundSettings(inbound *model.Inbound, nc *model.NodeClient, flow string) bool {
	if inbound.Settings == "" {
		return false
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return false
	}

	key := "clients"
	if inbound.Protocol == model.WireGuard {
		key = "peers"
	}

	var list []interface{}
	if raw, ok := settings[key]; ok && raw != nil {
		if l, ok := raw.([]interface{}); ok {
			list = l
		}
	}

	// Check if already present
	for _, item := range list {
		c, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cEmail, _ := c["email"].(string)
		cId, _ := c["id"].(string)
		if (nc.Email != "" && cEmail != "" && strings.EqualFold(strings.TrimSpace(cEmail), strings.TrimSpace(nc.Email))) ||
			(nc.UUID != "" && cId != "" && strings.EqualFold(strings.TrimSpace(cId), strings.TrimSpace(nc.UUID))) {
			return false
		}
	}

	clientFlow := flow
	if clientFlow == "" {
		clientFlow = nc.Flow
	}

	newClientMap := map[string]interface{}{
		"email":      nc.Email,
		"enable":     nc.Enable,
		"expiryTime": nc.ExpiryTime,
		"limitIp":    nc.LimitIP,
		"reset":      nc.Reset,
		"subId":      nc.SubID,
		"tgId":       nc.TgID,
		"totalGB":    nc.TotalGB,
		"comment":    nc.Comment,
	}

	switch inbound.Protocol {
	case "trojan":
		newClientMap["password"] = nc.Password
	case "shadowsocks":
		newClientMap["password"] = nc.Password
		if m, ok := settings["method"]; ok {
			newClientMap["method"] = m
		}
	case "hysteria":
		newClientMap["auth"] = nc.Auth
	case "wireguard":
		newClientMap["publicKey"] = nc.UUID
		newClientMap["privateKey"] = nc.Password
	default:
		newClientMap["id"] = nc.UUID
		newClientMap["flow"] = clientFlow
	}

	list = append(list, newClientMap)
	settings[key] = list

	newSettings, err := json.MarshalIndent(settings, "", "  ")
	if err == nil {
		inbound.Settings = string(newSettings)
		return true
	}
	return false
}

// updateClientFlowInInboundSettings updates client flow in an inbound's settings JSON.
func updateClientFlowInInboundSettings(inbound *model.Inbound, email, uuid, newFlow string) bool {
	if inbound.Settings == "" {
		return false
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return false
	}
	raw, ok := settings["clients"]
	if !ok || raw == nil {
		return false
	}
	list, ok := raw.([]interface{})
	if !ok || len(list) == 0 {
		return false
	}
	modified := false
	for _, item := range list {
		c, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cEmail, _ := c["email"].(string)
		cId, _ := c["id"].(string)
		if (email != "" && cEmail != "" && strings.EqualFold(strings.TrimSpace(cEmail), strings.TrimSpace(email))) ||
			(uuid != "" && cId != "" && strings.EqualFold(strings.TrimSpace(cId), strings.TrimSpace(uuid))) {
			c["flow"] = newFlow
			modified = true
		}
	}
	if modified {
		newSettings, err := json.MarshalIndent(settings, "", "  ")
		if err == nil {
			inbound.Settings = string(newSettings)
			return true
		}
	}
	return false
}

// hotAddUserToInbound dynamically adds a client to a running inbound using Xray's HandlerService API.
// Returns true if Xray needs to be restarted (e.g. wireguard or API error).
func (s *NodeClientService) hotAddUserToInbound(xrayApi *xray.XrayAPI, inbound *model.Inbound, nc *model.NodeClient, flow string) bool {
	if inbound.Protocol == model.WireGuard {
		return true
	}
	if !inbound.Enable || !nc.Enable {
		return false
	}
	now := time.Now().Unix() * 1000
	if nc.ExpiryTime > 0 && nc.ExpiryTime <= now {
		return false
	}
	if nc.TotalGB > 0 {
		db := database.GetDB()
		var totalTraffic struct {
			Up   int64
			Down int64
		}
		q := db.Table("client_traffics").Select("COALESCE(MAX(up), 0) as up, COALESCE(MAX(down), 0) as down")
		if nc.Email != "" {
			q = q.Where("node_client_id = ? OR LOWER(email) = LOWER(?)", nc.Id, nc.Email)
		} else {
			q = q.Where("node_client_id = ?", nc.Id)
		}
		_ = q.Scan(&totalTraffic).Error
		if (totalTraffic.Up + totalTraffic.Down) >= nc.TotalGB {
			return false
		}
	}

	cipher := ""
	if inbound.Protocol == "shadowsocks" {
		var settings map[string]interface{}
		_ = json.Unmarshal([]byte(inbound.Settings), &settings)
		if m, ok := settings["method"].(string); ok {
			cipher = m
		}
	}

	clientFlow := flow
	if clientFlow == "" {
		clientFlow = nc.Flow
	}

	err := xrayApi.AddUser(string(inbound.Protocol), inbound.Tag, map[string]interface{}{
		"email":    nc.Email,
		"id":       nc.UUID,
		"auth":     nc.Auth,
		"security": nc.Security,
		"flow":     clientFlow,
		"password": nc.Password,
		"cipher":   cipher,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return false
		}
		logger.Warningf("hotAddUserToInbound: failed to add %s to %s: %v", nc.Email, inbound.Tag, err)
		return true
	}
	return false
}

// hotRemoveUserFromInbound dynamically removes a client from a running inbound using Xray's HandlerService API.
// Returns true if Xray needs to be restarted (e.g. wireguard or API error).
func (s *NodeClientService) hotRemoveUserFromInbound(xrayApi *xray.XrayAPI, inbound *model.Inbound, email string) bool {
	if inbound.Protocol == model.WireGuard {
		return true
	}
	err := xrayApi.RemoveUser(inbound.Tag, email)
	if err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf("User %s not found.", email)) ||
			strings.Contains(err.Error(), "not found") {
			return false
		}
		logger.Warningf("hotRemoveUserFromInbound: failed to remove %s from %s: %v", email, inbound.Tag, err)
		return true
	}
	return false
}

// deleteInTx performs the deletion within a provided transaction.
// It removes the client from inbounds settings JSON, deletes links,
// cleans client traffic records, and deletes the NodeClient record.
// It is reused by both Delete and BulkDelete.
func (s *NodeClientService) deleteInTx(tx *gorm.DB, id int) error {
	// 1. Verify the NodeClient exists.
	nc := &model.NodeClient{}
	if err := tx.First(nc, id).Error; err != nil {
		return err
	}

	// 2. Remove this client from all inbounds' settings JSON so MigrateLegacyClients doesn't resurrect it on startup.
	var inbounds []*model.Inbound
	if err := tx.Find(&inbounds).Error; err == nil {
		for _, inbound := range inbounds {
			if removeClientFromInboundSettings(inbound, nc.Email, nc.UUID, nc.Password, nc.Auth) {
				if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error; err != nil {
					logger.Warningf("deleteInTx: failed to update inbound %d settings: %v", inbound.Id, err)
				}
			}
		}
	}

	// 3. Delete all NodeClientLink rows for this NodeClient.
	if err := tx.Where("node_client_id = ?", id).Delete(&model.NodeClientLink{}).Error; err != nil {
		return fmt.Errorf("deleting node client links for id %d: %w", id, err)
	}

	// 4. Delete associated ClientTraffic rows.
	if err := tx.Where("node_client_id = ? OR (email != '' AND LOWER(email) = LOWER(?))", id, nc.Email).
		Delete(&xray.ClientTraffic{}).Error; err != nil {
		return fmt.Errorf("deleting client_traffics for id %d: %w", id, err)
	}

	// 5. Delete the NodeClient record itself.
	if err := tx.Delete(&model.NodeClient{}, id).Error; err != nil {
		return fmt.Errorf("deleting node client id %d: %w", id, err)
	}

	logger.Debugf("NodeClient %d deleted (inbounds, links, and traffic cleaned up)", id)
	return nil
}

// ---------------------------------------------------------------------------
// CRUD — BulkDelete
// ---------------------------------------------------------------------------

// BulkDelete wraps individual delete logic in a single transaction. If any
// deletion fails, the entire operation is rolled back. Hot-removes users from
// running inbounds via Xray API if active.
// Requirements: 8.4
func (s *NodeClientService) BulkDelete(ids []int) (bool, error) {
	if len(ids) == 0 {
		return false, nil
	}

	nodeClientOpMutex.Lock()
	defer nodeClientOpMutex.Unlock()

	return s.bulkDeleteUnlocked(ids)
}

func (s *NodeClientService) bulkDeleteUnlocked(ids []int) (bool, error) {
	if len(ids) == 0 {
		return false, nil
	}

	db := database.GetDB()

	type linkInfo struct {
		inboundId int
		email     string
	}
	var toRemove []linkInfo
	for _, id := range ids {
		var nc model.NodeClient
		if err := db.First(&nc, id).Error; err == nil && nc.Email != "" {
			var links []model.NodeClientLink
			if err := db.Where("node_client_id = ?", id).Find(&links).Error; err == nil {
				for _, l := range links {
					toRemove = append(toRemove, linkInfo{inboundId: l.InboundId, email: nc.Email})
				}
			}
		}
	}

	err := database.ExecWithRetry(5, func() error {
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

		return tx.Commit().Error
	})
	if err != nil {
		return false, err
	}

	needRestart := false
	if p != nil && p.IsRunning() && len(toRemove) > 0 {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()
			for _, item := range toRemove {
				var inbound model.Inbound
				if err := db.First(&inbound, item.inboundId).Error; err == nil {
					if s.hotRemoveUserFromInbound(&xrayApi, &inbound, item.email) {
						needRestart = true
					}
				}
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
}

// ---------------------------------------------------------------------------
// Link management
// ---------------------------------------------------------------------------

// AddLink creates a NodeClientLink between the given node client and inbound.
// It also ensures a ClientTraffic row exists for the (email, inboundId) pair
// with NodeClientId set, and syncs settings JSON.
// Dynamically adds user to running Xray via API without restarting Xray.
// Requirements: 2.1, 2.2, 2.3, 4.1
func (s *NodeClientService) AddLink(nodeClientId, inboundId int, flow string) (bool, error) {
	db := database.GetDB()

	// 1. Fetch the NodeClient — error if not found.
	nc := &model.NodeClient{}
	if err := db.First(nc, nodeClientId).Error; err != nil {
		return false, fmt.Errorf("node client not found (id=%d): %w", nodeClientId, err)
	}

	// 2. Fetch the Inbound — error if not found.
	var inbound model.Inbound
	if err := db.First(&inbound, inboundId).Error; err != nil {
		return false, fmt.Errorf("inbound not found (id=%d): %w", inboundId, err)
	}

	// 3. Check for duplicate (nodeClientId, inboundId) pair.
	var linkCount int64
	if err := db.Model(&model.NodeClientLink{}).
		Where("node_client_id = ? AND inbound_id = ?", nodeClientId, inboundId).
		Count(&linkCount).Error; err != nil {
		return false, err
	}
	if linkCount > 0 {
		return false, fmt.Errorf("link already exists for node client %d and inbound %d", nodeClientId, inboundId)
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 4. Create the NodeClientLink record.
	link := &model.NodeClientLink{
		NodeClientId: nodeClientId,
		InboundId:    inboundId,
		Flow:         flow,
	}
	if err := tx.Create(link).Error; err != nil {
		tx.Rollback()
		return false, fmt.Errorf("creating node client link: %w", err)
	}

	// 5. Add to inbound.Settings if not already present.
	if addClientToInboundSettings(&inbound, nc, flow) {
		_ = tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error
	}

	// 6. Upsert the ClientTraffic row.
	ct := &xray.ClientTraffic{}
	result := tx.Where("email = ? AND inbound_id = ?", nc.Email, inboundId).First(ct)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		tx.Rollback()
		return false, fmt.Errorf("querying client traffic for email %s and inbound %d: %w", nc.Email, inboundId, result.Error)
	}

	if result.Error == gorm.ErrRecordNotFound {
		var existingTraffics []xray.ClientTraffic
		_ = tx.Where("node_client_id = ? OR LOWER(email) = LOWER(?)", nodeClientId, nc.Email).Find(&existingTraffics).Error
		var maxUp, maxDown int64
		for _, et := range existingTraffics {
			if et.Up > maxUp {
				maxUp = et.Up
			}
			if et.Down > maxDown {
				maxDown = et.Down
			}
		}

		ct = &xray.ClientTraffic{
			InboundId:    inboundId,
			Email:        nc.Email,
			Enable:       true,
			Up:           maxUp,
			Down:         maxDown,
			Total:        nc.TotalGB,
			ExpiryTime:   nc.ExpiryTime,
			Reset:        nc.Reset,
			NodeClientId: &nodeClientId,
		}
		if err := tx.Create(ct).Error; err != nil {
			tx.Rollback()
			return false, fmt.Errorf("creating client traffic for email %s and inbound %d: %w", nc.Email, inboundId, err)
		}
	} else {
		if err := tx.Model(ct).Updates(map[string]interface{}{
			"node_client_id": nodeClientId,
			"total":          nc.TotalGB,
			"expiry_time":    nc.ExpiryTime,
			"reset":          nc.Reset,
		}).Error; err != nil {
			tx.Rollback()
			return false, fmt.Errorf("updating client traffic for email %s and inbound %d: %w", nc.Email, inboundId, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	// 7. Hot-add user via Xray API if Xray is running, same as AddInboundClient.
	needRestart := false
	if p != nil && p.IsRunning() {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()
			if s.hotAddUserToInbound(&xrayApi, &inbound, nc, flow) {
				needRestart = true
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
}

// RemoveLink deletes the NodeClientLink for the given (nodeClientId, inboundId)
// pair and NULL-outs node_client_id on the corresponding ClientTraffic row
// (preserving the row for historical data), and strips from inbound.Settings.
// Dynamically removes user from running Xray via API without restarting Xray.
// Requirements: 2.5, 4.4
func (s *NodeClientService) RemoveLink(nodeClientId, inboundId int) (bool, error) {
	db := database.GetDB()

	// 1. Find the NodeClientLink.
	link := &model.NodeClientLink{}
	if err := db.Where("node_client_id = ? AND inbound_id = ?", nodeClientId, inboundId).
		First(link).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, fmt.Errorf("link not found for node client %d and inbound %d", nodeClientId, inboundId)
		}
		return false, err
	}

	// 2. Fetch the NodeClient to get its email.
	nc := &model.NodeClient{}
	if err := db.First(nc, nodeClientId).Error; err != nil {
		return false, fmt.Errorf("node client not found (id=%d): %w", nodeClientId, err)
	}

	// 3. Fetch the Inbound.
	var inbound model.Inbound
	if err := db.First(&inbound, inboundId).Error; err != nil {
		return false, fmt.Errorf("inbound not found (id=%d): %w", inboundId, err)
	}

	err := database.ExecWithRetry(5, func() error {
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 4. Delete the NodeClientLink record.
		if err := tx.Delete(link).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("deleting node client link: %w", err)
		}

		// 5. Remove from inbound.Settings for this specific inbound so MigrateLegacyClients won't resurrect the link.
		if removeClientFromInboundSettings(&inbound, nc.Email, nc.UUID, nc.Password, nc.Auth) {
			_ = tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error
		}

		// 6. Preserve traffic on remaining links if this inbound had higher usage, then NULL-out node_client_id for this inbound only.
		var thisTraffic xray.ClientTraffic
		if err := tx.Where("email = ? AND inbound_id = ?", nc.Email, inboundId).First(&thisTraffic).Error; err == nil {
			if thisTraffic.Up > 0 || thisTraffic.Down > 0 {
				var otherTraffics []xray.ClientTraffic
				if err := tx.Where("(node_client_id = ? OR LOWER(email) = LOWER(?)) AND inbound_id != ?", nodeClientId, nc.Email, inboundId).Find(&otherTraffics).Error; err == nil {
					for _, other := range otherTraffics {
						updates := make(map[string]interface{})
						if thisTraffic.Up > other.Up {
							updates["up"] = thisTraffic.Up
						}
						if thisTraffic.Down > other.Down {
							updates["down"] = thisTraffic.Down
						}
						if len(updates) > 0 {
							_ = tx.Model(&xray.ClientTraffic{}).Where("id = ?", other.Id).Updates(updates).Error
						}
					}
				}
			}
		}

		if err := tx.Model(&xray.ClientTraffic{}).
			Where("email = ? AND node_client_id = ? AND inbound_id = ?", nc.Email, nodeClientId, inboundId).
			Update("node_client_id", nil).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("nulling node_client_id on client traffic for email %s and inbound %d: %w", nc.Email, inboundId, err)
		}

		return tx.Commit().Error
	})
	if err != nil {
		return false, err
	}

	// 7. Hot-remove user via Xray API if Xray is running, same as DelInboundClient.
	needRestart := false
	if p != nil && p.IsRunning() {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()
			if s.hotRemoveUserFromInbound(&xrayApi, &inbound, nc.Email) {
				needRestart = true
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
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
// inside a single transaction. Hot-adds the user to running Xray via API if re-enabled.
// Requirements: 4.5, 6.3
func (s *NodeClientService) ResetTraffic(nodeClientId int) (bool, error) {
	nodeClientOpMutex.Lock()
	defer nodeClientOpMutex.Unlock()

	return s.resetTrafficUnlocked(nodeClientId)
}

func (s *NodeClientService) resetTrafficUnlocked(nodeClientId int) (bool, error) {
	db := database.GetDB()

	var nc model.NodeClient
	var reEnabled bool
	var siblingNCs []model.NodeClient
	var syncClients bool

	err := database.ExecWithRetry(5, func() error {
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		if err := tx.First(&nc, nodeClientId).Error; err != nil {
			tx.Rollback()
			return err
		}

		updates := map[string]interface{}{
			"up":             0,
			"down":           0,
			"node_client_id": nodeClientId,
		}

		now := time.Now().Unix() * 1000
		reEnabled = false
		if nc.ExpiryTime <= 0 || nc.ExpiryTime > now {
			updates["enable"] = true
			reEnabled = true
			if !nc.Enable {
				_ = tx.Model(&model.NodeClient{}).Where("id = ?", nodeClientId).Update("enable", true).Error
				nc.Enable = true
			}
		}

		whereClause := "node_client_id = ?"
		whereArgs := []interface{}{nodeClientId}
		if nc.Email != "" {
			whereClause += " OR LOWER(email) = LOWER(?)"
			whereArgs = append(whereArgs, nc.Email)
		}

		if err := tx.Model(&xray.ClientTraffic{}).
			Where(whereClause, whereArgs...).
			Updates(updates).Error; err != nil {
			tx.Rollback()
			return err
		}

		// If client sync is enabled and client has subId, sync reset across all clients with same subId
		settingService := &SettingService{}
		var serr error
		syncClients, serr = settingService.GetSyncClients()
		if serr == nil && syncClients && nc.SubID != "" {
			siblingNCs = nil
			_ = tx.Where("sub_id = ? AND id != ?", nc.SubID, nodeClientId).Find(&siblingNCs).Error
			var siblingEmails []string
			var siblingIds []int
			for _, snc := range siblingNCs {
				siblingIds = append(siblingIds, snc.Id)
				if snc.Email != "" {
					siblingEmails = append(siblingEmails, snc.Email)
				}
			}
			if len(siblingIds) > 0 && reEnabled {
				_ = tx.Model(&model.NodeClient{}).Where("id IN ?", siblingIds).Update("enable", true).Error
			}
			if len(siblingEmails) > 0 || len(siblingIds) > 0 {
				sWhere := "node_client_id IN ?"
				sArgs := []interface{}{siblingIds}
				if len(siblingEmails) > 0 {
					sWhere += " OR LOWER(email) IN ?"
					sArgs = append(sArgs, siblingEmails)
				}
				sUpdates := map[string]interface{}{"up": 0, "down": 0}
				if reEnabled {
					sUpdates["enable"] = true
				}
				_ = tx.Model(&xray.ClientTraffic{}).Where(sWhere, sArgs...).Updates(sUpdates).Error
			}
		}

		return tx.Commit().Error
	})
	if err != nil {
		return false, err
	}

	needRestart := false
	if reEnabled && p != nil && p.IsRunning() {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()
			links, _ := s.GetLinks(nodeClientId)
			for _, l := range links {
				var inbound model.Inbound
				if err := db.First(&inbound, l.InboundId).Error; err == nil {
					flow := l.Flow
					if flow == "" {
						flow = nc.Flow
					}
					if s.hotAddUserToInbound(&xrayApi, &inbound, &nc, flow) {
						needRestart = true
					}
				}
			}
			if syncClients && len(siblingNCs) > 0 {
				for _, snc := range siblingNCs {
					slinks, _ := s.GetLinks(snc.Id)
					for _, sl := range slinks {
						var inbound model.Inbound
						if err := db.First(&inbound, sl.InboundId).Error; err == nil {
							flow := sl.Flow
							if flow == "" {
								flow = snc.Flow
							}
							if s.hotAddUserToInbound(&xrayApi, &inbound, &snc, flow) {
								needRestart = true
							}
						}
					}
				}
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
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
	err := db.Table("node_client_links").
		Joins("JOIN inbounds ON inbounds.id = node_client_links.inbound_id").
		Where("node_client_links.node_client_id = ?", nodeClientId).
		Select("node_client_links.*").
		Find(&links).Error
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
	var clientsToDisable []*model.NodeClient

	for _, nc := range nodeClients {
		shouldDisable := false

		// Check expiry
		if nc.ExpiryTime > 0 && nc.ExpiryTime <= now {
			shouldDisable = true
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
			}
		}

		if shouldDisable {
			idsToDisable = append(idsToDisable, nc.Id)
			clientsToDisable = append(clientsToDisable, nc)
		}
	}

	if len(idsToDisable) == 0 {
		return false, nil
	}

	// Only disable linked client_traffics rows so every inbound reflects the exhaustion in Xray.
	// We do NOT update node_clients.enable to false, because the switch represents manual deactivation;
	// exhausted/expired clients are disconnected at the network/core level while keeping the switch on.
	//
	// We must track which node clients actually had rows newly disabled so that we only trigger the
	// Xray hot-reload once. Without this guard the hot-reload (and its log spam) would fire every
	// second because the NodeClient's own enable flag is intentionally never cleared.
	newlyDisabledIDs := make(map[int]bool)
	for _, id := range idsToDisable {
		var count int64
		if err := db.Model(&xray.ClientTraffic{}).Where("node_client_id = ? AND enable = ?", id, true).Count(&count).Error; err == nil && count > 0 {
			newlyDisabledIDs[id] = true
		}
	}

	res := db.Model(&xray.ClientTraffic{}).Where("node_client_id IN ? AND enable = ?", idsToDisable, true).Update("enable", false)
	if res.Error != nil {
		return false, res.Error
	}


	// Dynamically remove exhausted users from running inbounds via API so no full Xray restart is needed.
	// Only act on clients that actually had rows newly disabled this iteration.
	if p != nil && p.IsRunning() && len(newlyDisabledIDs) > 0 {
		logger.Debugf("Disabled %d exhausted node client traffic rows", res.RowsAffected)
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err == nil {
			defer xrayApi.Close()
			for _, nc := range clientsToDisable {
				if !newlyDisabledIDs[nc.Id] {
					continue
				}
				if nc.ExpiryTime > 0 && nc.ExpiryTime <= now {
					logger.Debugf("NodeClient %d (%s) expired, removing from active inbounds", nc.Id, nc.Email)
				} else {
					logger.Debugf("NodeClient %d (%s) traffic exhausted, removing from active inbounds", nc.Id, nc.Email)
				}
				links, _ := s.GetLinks(nc.Id)
				for _, l := range links {
					var inbound model.Inbound
					if err := db.First(&inbound, l.InboundId).Error; err == nil {
						s.hotRemoveUserFromInbound(&xrayApi, &inbound, nc.Email)
						inboundService := InboundService{}
						inboundService.HotReloadInboundByTag(inbound.Tag, db)
					}
				}
			}
		}
	}

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

		// Also update settings JSON for linked inbounds so the new expiry is reflected everywhere
		links, _ := s.GetLinks(nc.Id)
		for _, l := range links {
			var inbound model.Inbound
			if err := tx.First(&inbound, l.InboundId).Error; err == nil {
				flow := l.Flow
				if flow == "" {
					flow = nc.Flow
				}
				removeClientFromInboundSettings(&inbound, nc.Email, nc.UUID, nc.Password, nc.Auth)
				ncWithNewExpiry := *nc
				ncWithNewExpiry.ExpiryTime = newExpiryTime
				ncWithNewExpiry.Enable = true
				addClientToInboundSettings(&inbound, &ncWithNewExpiry, flow)
				_ = tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", inbound.Settings).Error
			}
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
		Joins("LEFT JOIN client_traffics ON client_traffics.node_client_id = node_clients.id AND client_traffics.inbound_id = node_client_links.inbound_id").
		Where("node_client_links.inbound_id = ? AND node_clients.enable = ? AND (client_traffics.enable IS NULL OR client_traffics.enable = ?)", inboundId, true, true).
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

var nodeClientOpMutex sync.Mutex

// SetLinks replaces all links for a client with the provided list.
// Dynamically adds and removes users from running Xray via API without restarting Xray.
func (s *NodeClientService) SetLinks(nodeClientId int, links []NodeClientLinkInput) (bool, error) {
	nodeClientOpMutex.Lock()
	defer nodeClientOpMutex.Unlock()

	db := database.GetDB()
	nc := &model.NodeClient{}
	if err := db.First(nc, nodeClientId).Error; err != nil {
		return false, fmt.Errorf("client not found (id=%d): %w", nodeClientId, err)
	}

	var allInbounds []model.Inbound
	if err := db.Find(&allInbounds).Error; err != nil {
		return false, err
	}
	inboundMap := make(map[int]*model.Inbound, len(allInbounds))
	for i := range allInbounds {
		inboundMap[allInbounds[i].Id] = &allInbounds[i]
	}
	modifiedInbounds := make(map[int]bool)

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var currentLinks []model.NodeClientLink
	if err := tx.Where("node_client_id = ?", nodeClientId).Find(&currentLinks).Error; err != nil {
		tx.Rollback()
		return false, err
	}

	newInboundMap := make(map[int]string, len(links))
	for _, l := range links {
		newInboundMap[l.InboundId] = l.Flow
	}

	var removedInboundIds []int
	var addedLinks []NodeClientLinkInput
	var updatedFlowLinks []NodeClientLinkInput

	// Remove links not in new set
	for _, cl := range currentLinks {
		if _, keep := newInboundMap[cl.InboundId]; !keep {
			removedInboundIds = append(removedInboundIds, cl.InboundId)
			if err := tx.Delete(&cl).Error; err != nil {
				tx.Rollback()
				return false, err
			}
			_ = tx.Model(&xray.ClientTraffic{}).
				Where("LOWER(email) = LOWER(?) AND node_client_id = ? AND inbound_id = ?", nc.Email, nodeClientId, cl.InboundId).
				Update("node_client_id", nil).Error

			if ib, ok := inboundMap[cl.InboundId]; ok {
				if removeClientFromInboundSettings(ib, nc.Email, nc.UUID, nc.Password, nc.Auth) {
					modifiedInbounds[ib.Id] = true
				}
			}
		}
	}

	// Add or update links
	for _, l := range links {
		var existing model.NodeClientLink
		err := tx.Where("node_client_id = ? AND inbound_id = ?", nodeClientId, l.InboundId).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			addedLinks = append(addedLinks, l)
			newLink := model.NodeClientLink{
				NodeClientId: nodeClientId,
				InboundId:    l.InboundId,
				Flow:         l.Flow,
			}
			if err := tx.Create(&newLink).Error; err != nil {
				tx.Rollback()
				return false, err
			}

			if ib, ok := inboundMap[l.InboundId]; ok {
				if addClientToInboundSettings(ib, nc, l.Flow) {
					modifiedInbounds[ib.Id] = true
				}
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
				updatedFlowLinks = append(updatedFlowLinks, l)

				if ib, ok := inboundMap[l.InboundId]; ok {
					if updateClientFlowInInboundSettings(ib, nc.Email, nc.UUID, l.Flow) {
						modifiedInbounds[ib.Id] = true
					}
				}
			}
		}
	}

	// Flush modified inbound settings once per inbound
	for inId := range modifiedInbounds {
		if ib, ok := inboundMap[inId]; ok {
			if err := tx.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", ib.Settings).Error; err != nil {
				tx.Rollback()
				return false, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	// Hot-sync changes with running Xray
	needRestart := false
	if p != nil && p.IsRunning() {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()

			// 1. Remove users from unlinked inbounds
			for _, inId := range removedInboundIds {
				if ib, ok := inboundMap[inId]; ok {
					if s.hotRemoveUserFromInbound(&xrayApi, ib, nc.Email) {
						needRestart = true
					}
				}
			}

			// 2. For updated flow links: remove and re-add with new flow
			for _, l := range updatedFlowLinks {
				if ib, ok := inboundMap[l.InboundId]; ok {
					s.hotRemoveUserFromInbound(&xrayApi, ib, nc.Email)
					if s.hotAddUserToInbound(&xrayApi, ib, nc, l.Flow) {
						needRestart = true
					}
				}
			}

			// 3. For newly added links: add user
			for _, l := range addedLinks {
				if ib, ok := inboundMap[l.InboundId]; ok {
					if s.hotAddUserToInbound(&xrayApi, ib, nc, l.Flow) {
						needRestart = true
					}
				}
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
}

// BulkSetLinks updates links for multiple clients according to the specified action.
// Supported actions:
//   - "set" / "replace": replaces all links of each client with inboundIds
//   - "add": preserves existing links and adds inboundIds (optionally updating flow)
//   - "remove": removes inboundIds from each client's existing links
func (s *NodeClientService) BulkSetLinks(clientIds []int, inboundIds []int, action string, flow string) (bool, error) {
	if len(clientIds) == 0 {
		return false, nil
	}

	nodeClientOpMutex.Lock()
	defer nodeClientOpMutex.Unlock()

	db := database.GetDB()

	// 1. Fetch all requested clients
	var clients []model.NodeClient
	if err := db.Where("id IN ?", clientIds).Find(&clients).Error; err != nil {
		return false, err
	}
	if len(clients) == 0 {
		return false, nil
	}

	// 2. Fetch all current links for these clients
	var allCurrentLinks []model.NodeClientLink
	if err := db.Where("node_client_id IN ?", clientIds).Find(&allCurrentLinks).Error; err != nil {
		return false, err
	}
	currentLinksByClient := make(map[int][]model.NodeClientLink, len(clients))
	for _, l := range allCurrentLinks {
		currentLinksByClient[l.NodeClientId] = append(currentLinksByClient[l.NodeClientId], l)
	}

	// 3. Cache all inbounds that could be affected
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return false, err
	}
	inboundMap := make(map[int]*model.Inbound, len(inbounds))
	for i := range inbounds {
		inboundMap[inbounds[i].Id] = &inbounds[i]
	}

	type hotSyncAction struct {
		inboundId int
		email     string
		nc        *model.NodeClient
		flow      string
		isRemove  bool
	}
	var hotSyncItems []hotSyncAction
	modifiedInbounds := make(map[int]bool)

	// 4. Perform all database changes inside a SINGLE atomic transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i := range clients {
		nc := &clients[i]
		curLinks := currentLinksByClient[nc.Id]

		// Determine target links for this client
		var targetLinks []NodeClientLinkInput
		switch action {
		case "add":
			linkMap := make(map[int]string, len(curLinks)+len(inboundIds))
			for _, l := range curLinks {
				linkMap[l.InboundId] = l.Flow
			}
			for _, inId := range inboundIds {
				if flow != "" {
					linkMap[inId] = flow
				} else if _, ok := linkMap[inId]; !ok {
					linkMap[inId] = ""
				}
			}
			targetLinks = make([]NodeClientLinkInput, 0, len(linkMap))
			for inId, fl := range linkMap {
				targetLinks = append(targetLinks, NodeClientLinkInput{
					InboundId: inId,
					Flow:      fl,
				})
			}

		case "remove":
			removeSet := make(map[int]bool, len(inboundIds))
			for _, inId := range inboundIds {
				removeSet[inId] = true
			}
			targetLinks = make([]NodeClientLinkInput, 0, len(curLinks))
			for _, l := range curLinks {
				if !removeSet[l.InboundId] {
					targetLinks = append(targetLinks, NodeClientLinkInput{
						InboundId: l.InboundId,
						Flow:      l.Flow,
					})
				}
			}

		default: // "set" or "replace"
			targetLinks = make([]NodeClientLinkInput, 0, len(inboundIds))
			for _, inId := range inboundIds {
				targetLinks = append(targetLinks, NodeClientLinkInput{
					InboundId: inId,
					Flow:      flow,
				})
			}
		}

		newInboundMap := make(map[int]string, len(targetLinks))
		for _, l := range targetLinks {
			newInboundMap[l.InboundId] = l.Flow
		}

		// Remove links not in target set
		for _, cl := range curLinks {
			if _, keep := newInboundMap[cl.InboundId]; !keep {
				if err := tx.Delete(&cl).Error; err != nil {
					tx.Rollback()
					return false, err
				}
				_ = tx.Model(&xray.ClientTraffic{}).
					Where("LOWER(email) = LOWER(?) AND node_client_id = ? AND inbound_id = ?", nc.Email, nc.Id, cl.InboundId).
					Update("node_client_id", nil).Error

				if ib, ok := inboundMap[cl.InboundId]; ok {
					if removeClientFromInboundSettings(ib, nc.Email, nc.UUID, nc.Password, nc.Auth) {
						modifiedInbounds[ib.Id] = true
					}
				}
				hotSyncItems = append(hotSyncItems, hotSyncAction{
					inboundId: cl.InboundId,
					email:     nc.Email,
					isRemove:  true,
				})
			}
		}

		// Add or update links
		curMap := make(map[int]model.NodeClientLink, len(curLinks))
		for _, cl := range curLinks {
			curMap[cl.InboundId] = cl
		}

		for _, l := range targetLinks {
			if existing, exists := curMap[l.InboundId]; !exists {
				newLink := model.NodeClientLink{
					NodeClientId: nc.Id,
					InboundId:    l.InboundId,
					Flow:         l.Flow,
				}
				if err := tx.Create(&newLink).Error; err != nil {
					tx.Rollback()
					return false, err
				}

				if ib, ok := inboundMap[l.InboundId]; ok {
					if addClientToInboundSettings(ib, nc, l.Flow) {
						modifiedInbounds[ib.Id] = true
					}
				}

				var ct xray.ClientTraffic
				res := tx.Where("LOWER(email) = LOWER(?) AND inbound_id = ?", nc.Email, l.InboundId).First(&ct)
				if res.Error == gorm.ErrRecordNotFound {
					var existingTraffics []xray.ClientTraffic
					_ = tx.Where("node_client_id = ? OR LOWER(email) = LOWER(?)", nc.Id, nc.Email).Find(&existingTraffics).Error
					var maxUp, maxDown int64
					for _, et := range existingTraffics {
						if et.Up > maxUp {
							maxUp = et.Up
						}
						if et.Down > maxDown {
							maxDown = et.Down
						}
					}

					ct = xray.ClientTraffic{
						InboundId:    l.InboundId,
						Email:        nc.Email,
						Enable:       true,
						Up:           maxUp,
						Down:         maxDown,
						Total:        nc.TotalGB,
						ExpiryTime:   nc.ExpiryTime,
						Reset:        nc.Reset,
						NodeClientId: &nc.Id,
					}
					_ = tx.Create(&ct).Error
				} else if res.Error == nil {
					_ = tx.Model(&ct).Updates(map[string]interface{}{
						"node_client_id": nc.Id,
						"total":          nc.TotalGB,
						"expiry_time":    nc.ExpiryTime,
						"reset":          nc.Reset,
					}).Error
				}

				hotSyncItems = append(hotSyncItems, hotSyncAction{
					inboundId: l.InboundId,
					email:     nc.Email,
					nc:        nc,
					flow:      l.Flow,
					isRemove:  false,
				})
			} else {
				if existing.Flow != l.Flow {
					existing.Flow = l.Flow
					_ = tx.Save(&existing).Error

					if ib, ok := inboundMap[l.InboundId]; ok {
						if updateClientFlowInInboundSettings(ib, nc.Email, nc.UUID, l.Flow) {
							modifiedInbounds[ib.Id] = true
						}
					}
					hotSyncItems = append(hotSyncItems, hotSyncAction{
						inboundId: l.InboundId,
						email:     nc.Email,
						isRemove:  true,
					})
					hotSyncItems = append(hotSyncItems, hotSyncAction{
						inboundId: l.InboundId,
						email:     nc.Email,
						nc:        nc,
						flow:      l.Flow,
						isRemove:  false,
					})
				}
			}
		}
	}

	// Flush modified inbound settings once per inbound
	for inId := range modifiedInbounds {
		if ib, ok := inboundMap[inId]; ok {
			if err := tx.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", ib.Settings).Error; err != nil {
				tx.Rollback()
				return false, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	// 5. Hot-sync changes with running Xray outside of the DB transaction
	needRestart := false
	if p != nil && p.IsRunning() && len(hotSyncItems) > 0 {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()
			for _, item := range hotSyncItems {
				if ib, ok := inboundMap[item.inboundId]; ok {
					if item.isRemove {
						if s.hotRemoveUserFromInbound(&xrayApi, ib, item.email) {
							needRestart = true
						}
					} else {
						if s.hotAddUserToInbound(&xrayApi, ib, item.nc, item.flow) {
							needRestart = true
						}
					}
				}
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
}

// BulkCreate creates multiple clients and links each to the specified inbounds.
// Hot-adds users to running Xray via API without restarting Xray.
func (s *NodeClientService) BulkCreate(clients []model.NodeClient, inboundIds []int) (bool, error) {
	nodeClientOpMutex.Lock()
	defer nodeClientOpMutex.Unlock()

	db := database.GetDB()

	// Pre-fetch inbounds
	var inbounds []model.Inbound
	if len(inboundIds) > 0 {
		if err := db.Where("id IN ?", inboundIds).Find(&inbounds).Error; err != nil {
			return false, err
		}
	}
	inboundMap := make(map[int]*model.Inbound, len(inbounds))
	for i := range inbounds {
		inboundMap[inbounds[i].Id] = &inbounds[i]
	}
	modifiedInbounds := make(map[int]bool)

	// Validate emails and subIDs before opening write transaction
	for i := range clients {
		nc := &clients[i]
		if err := s.checkEmailUnique(nc.Email, 0); err != nil {
			return false, err
		}
		if err := s.checkSubIDUnique(nc.SubID, 0); err != nil {
			return false, err
		}
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for i := range clients {
		nc := &clients[i]
		if err := tx.Create(nc).Error; err != nil {
			tx.Rollback()
			return false, err
		}

		for _, inId := range inboundIds {
			link := model.NodeClientLink{
				NodeClientId: nc.Id,
				InboundId:    inId,
				Flow:         nc.Flow,
			}
			if err := tx.Create(&link).Error; err != nil {
				tx.Rollback()
				return false, err
			}

			if ib, ok := inboundMap[inId]; ok {
				if addClientToInboundSettings(ib, nc, nc.Flow) {
					modifiedInbounds[ib.Id] = true
				}
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
				return false, err
			}
		}
	}

	// Flush modified inbound settings once per inbound
	for inId := range modifiedInbounds {
		if ib, ok := inboundMap[inId]; ok {
			if err := tx.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("settings", ib.Settings).Error; err != nil {
				tx.Rollback()
				return false, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	needRestart := false
	if p != nil && p.IsRunning() && len(inboundIds) > 0 {
		var xrayApi xray.XrayAPI
		if err := xrayApi.Init(p.GetAPIPort()); err != nil {
			needRestart = true
		} else {
			defer xrayApi.Close()
			for _, inId := range inboundIds {
				if ib, ok := inboundMap[inId]; ok {
					for i := range clients {
						if s.hotAddUserToInbound(&xrayApi, ib, &clients[i], clients[i].Flow) {
							needRestart = true
						}
					}
				}
			}
		}
	}

	if needRestart {
		isNeedXrayRestart.Store(true)
	}
	return needRestart, nil
}

// Toggle toggles the enable flag of a NodeClient and hot-adds/removes it from Xray.
func (s *NodeClientService) Toggle(id int) (bool, error) {
	nc, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	nc.Enable = !nc.Enable
	return s.Update(nc)
}

// ResetAllTraffics resets upload and download counters on all clients.
func (s *NodeClientService) ResetAllTraffics() error {
	nodeClientOpMutex.Lock()
	defer nodeClientOpMutex.Unlock()

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&xray.ClientTraffic{}).
		Where("node_client_id IS NOT NULL").
		Updates(map[string]interface{}{"up": 0, "down": 0}).Error; err != nil {
		tx.Rollback()
		return err
	}
	now := time.Now().Unix() * 1000
	var enabledClientIds []int
	tx.Model(&model.NodeClient{}).
		Where("enable = ? AND (expiry_time <= 0 OR expiry_time > ?)", true, now).
		Pluck("id", &enabledClientIds)
	if len(enabledClientIds) > 0 {
		if err := tx.Model(&xray.ClientTraffic{}).
			Where("node_client_id IN ?", enabledClientIds).
			Update("enable", true).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	isNeedXrayRestart.Store(true)
	return nil
}

// DeleteDepleted deletes all clients whose quota is exhausted or expiry time has passed.
func (s *NodeClientService) DeleteDepleted() (bool, error) {
	nodeClientOpMutex.Lock()
	defer nodeClientOpMutex.Unlock()

	return s.deleteDepletedUnlocked()
}

func (s *NodeClientService) deleteDepletedUnlocked() (bool, error) {
	db := database.GetDB()
	var clients []*model.NodeClient
	if err := db.Find(&clients).Error; err != nil {
		return false, err
	}
	now := time.Now().Unix() * 1000

	type trafficSum struct {
		NodeClientId int   `gorm:"column:node_client_id"`
		TotalUp      int64 `gorm:"column:total_up"`
		TotalDown    int64 `gorm:"column:total_down"`
	}
	var sums []trafficSum
	_ = db.Model(&xray.ClientTraffic{}).
		Where("node_client_id IS NOT NULL").
		Select("node_client_id, SUM(up) as total_up, SUM(down) as total_down").
		Group("node_client_id").
		Scan(&sums).Error

	trafficByNc := make(map[int]int64, len(sums))
	for _, sum := range sums {
		trafficByNc[sum.NodeClientId] = sum.TotalUp + sum.TotalDown
	}

	var idsToDelete []int
	for _, nc := range clients {
		if nc.ExpiryTime > 0 && nc.ExpiryTime <= now {
			idsToDelete = append(idsToDelete, nc.Id)
			continue
		}
		used := trafficByNc[nc.Id]
		if nc.TotalGB > 0 && used >= nc.TotalGB {
			idsToDelete = append(idsToDelete, nc.Id)
			continue
		}
		if !nc.Enable && nc.TotalGB > 0 && used >= nc.TotalGB {
			idsToDelete = append(idsToDelete, nc.Id)
		}
	}
	if len(idsToDelete) > 0 {
		return s.bulkDeleteUnlocked(idsToDelete)
	}
	return false, nil
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
