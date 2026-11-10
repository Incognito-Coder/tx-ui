package service

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
	"x-ui/xray"
)

func setupTestDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() {
		if db := database.GetDB(); db != nil {
			if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
	})
}

func TestNodeClientUpdateDoesNotRemoveExhaustedClientFromInboundSettings(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	ncService := &NodeClientService{}

	// Create inbound with client in Settings
	initialSettings := map[string]interface{}{
		"clients": []interface{}{
			map[string]interface{}{
				"id":    "550e8400-e29b-41d4-a716-446655440000",
				"email": "exhausted@example.com",
				"flow":  "",
			},
		},
	}
	settingsBytes, _ := json.Marshal(initialSettings)

	inbound := &model.Inbound{
		UserId:   1,
		Tag:      "inbound-test-1",
		Port:     10001,
		Protocol: model.VLESS,
		Settings: string(settingsBytes),
		Enable:   true,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("failed to create inbound: %v", err)
	}

	totalQuota := int64(50 * 1024 * 1024 * 1024) // 50GB
	nc := &model.NodeClient{
		Email:      "exhausted@example.com",
		UUID:       "550e8400-e29b-41d4-a716-446655440000",
		TotalGB:    totalQuota,
		ExpiryTime: time.Now().Add(24 * time.Hour).Unix() * 1000,
		Enable:     true,
	}
	if err := db.Create(nc).Error; err != nil {
		t.Fatalf("failed to create node client: %v", err)
	}

	link := &model.NodeClientLink{
		NodeClientId: nc.Id,
		InboundId:    inbound.Id,
	}
	if err := db.Create(link).Error; err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	// Create exhausted traffic row: 50GB used of 50GB, enable=false
	ct := &xray.ClientTraffic{
		InboundId:    inbound.Id,
		Enable:       false,
		Email:        nc.Email,
		Up:           totalQuota / 2,
		Down:         totalQuota / 2,
		Total:        totalQuota,
		NodeClientId: &nc.Id,
	}
	if err := db.Create(ct).Error; err != nil {
		t.Fatalf("failed to create client traffic: %v", err)
	}

	// Now update node client (e.g. extending expiry time or comment, while traffic is still exhausted)
	nc.ExpiryTime = time.Now().Add(72 * time.Hour).Unix() * 1000
	nc.Comment = "renewed"
	_, err := ncService.Update(nc)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify inbound.Settings: the client MUST NOT be removed from inbound.Settings!
	var updatedInbound model.Inbound
	if err := db.First(&updatedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("failed to fetch updated inbound: %v", err)
	}

	var parsedSettings map[string]interface{}
	if err := json.Unmarshal([]byte(updatedInbound.Settings), &parsedSettings); err != nil {
		t.Fatalf("failed to parse updated inbound settings: %v", err)
	}
	clients, ok := parsedSettings["clients"].([]interface{})
	if !ok || len(clients) == 0 {
		t.Fatalf("expected client to remain in inbound.Settings, but clients list is empty or missing: %s", updatedInbound.Settings)
	}

	found := false
	for _, c := range clients {
		cm, ok := c.(map[string]interface{})
		if ok && cm["email"] == "exhausted@example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("exhausted client was removed from inbound.Settings! Settings: %s", updatedInbound.Settings)
	}
}

func TestNodeClientResetTrafficRestoresSettingsAndEnablesTraffic(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	ncService := &NodeClientService{}

	// Inbound where client was missing from Settings
	emptySettings := map[string]interface{}{
		"clients": []interface{}{},
	}
	settingsBytes, _ := json.Marshal(emptySettings)

	inbound := &model.Inbound{
		UserId:   1,
		Tag:      "inbound-test-2",
		Port:     10002,
		Protocol: model.VLESS,
		Settings: string(settingsBytes),
		Enable:   true,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("failed to create inbound: %v", err)
	}

	totalQuota := int64(100 * 1024 * 1024 * 1024)
	nc := &model.NodeClient{
		Email:      "client2@example.com",
		UUID:       "660e8400-e29b-41d4-a716-446655440001",
		TotalGB:    totalQuota,
		ExpiryTime: time.Now().Add(24 * time.Hour).Unix() * 1000,
		Enable:     true,
	}
	if err := db.Create(nc).Error; err != nil {
		t.Fatalf("failed to create node client: %v", err)
	}

	link := &model.NodeClientLink{
		NodeClientId: nc.Id,
		InboundId:    inbound.Id,
	}
	if err := db.Create(link).Error; err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	ct := &xray.ClientTraffic{
		InboundId:    inbound.Id,
		Enable:       false,
		Email:        nc.Email,
		Up:           totalQuota,
		Down:         0,
		Total:        totalQuota,
		NodeClientId: &nc.Id,
	}
	if err := db.Create(ct).Error; err != nil {
		t.Fatalf("failed to create client traffic: %v", err)
	}

	// Reset traffic
	_, err := ncService.ResetTraffic(nc.Id)
	if err != nil {
		t.Fatalf("ResetTraffic failed: %v", err)
	}

	// Verify ClientTraffic
	var updatedCT xray.ClientTraffic
	if err := db.First(&updatedCT, ct.Id).Error; err != nil {
		t.Fatalf("failed to fetch updated client traffic: %v", err)
	}
	if updatedCT.Up != 0 || updatedCT.Down != 0 {
		t.Fatalf("expected up=0, down=0, got up=%d, down=%d", updatedCT.Up, updatedCT.Down)
	}
	if !updatedCT.Enable {
		t.Fatalf("expected enable=true after traffic reset, got enable=false")
	}

	// Verify inbound.Settings: client was missing, should now be restored!
	var updatedInbound model.Inbound
	if err := db.First(&updatedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("failed to fetch updated inbound: %v", err)
	}
	var parsedSettings map[string]interface{}
	_ = json.Unmarshal([]byte(updatedInbound.Settings), &parsedSettings)
	clients := parsedSettings["clients"].([]interface{})
	if len(clients) != 1 {
		t.Fatalf("expected client to be restored to inbound.Settings, got %d clients", len(clients))
	}
}

func TestInboundResetClientTraffic(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	inboundService := &InboundService{}

	clientSettings := map[string]interface{}{
		"clients": []interface{}{
			map[string]interface{}{
				"id":     "770e8400-e29b-41d4-a716-446655440002",
				"email":  "standard@example.com",
				"flow":   "",
				"enable": true,
			},
		},
	}
	settingsBytes, _ := json.Marshal(clientSettings)

	inbound := &model.Inbound{
		UserId:   1,
		Tag:      "inbound-test-3",
		Port:     10003,
		Protocol: model.VLESS,
		Settings: string(settingsBytes),
		Enable:   true,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("failed to create inbound: %v", err)
	}

	totalQuota := int64(10 * 1024 * 1024 * 1024)
	ct := &xray.ClientTraffic{
		InboundId: inbound.Id,
		Enable:    false,
		Email:     "standard@example.com",
		Up:        totalQuota,
		Down:      0,
		Total:     totalQuota,
	}
	if err := db.Create(ct).Error; err != nil {
		t.Fatalf("failed to create client traffic: %v", err)
	}

	// Reset traffic with mixed-case email
	_, err := inboundService.ResetClientTraffic(inbound.Id, "Standard@example.com")
	if err != nil {
		t.Fatalf("ResetClientTraffic failed: %v", err)
	}

	var updatedCT xray.ClientTraffic
	if err := db.First(&updatedCT, ct.Id).Error; err != nil {
		t.Fatalf("failed to fetch updated client traffic: %v", err)
	}
	if updatedCT.Up != 0 || updatedCT.Down != 0 {
		t.Fatalf("expected up=0, down=0, got up=%d, down=%d", updatedCT.Up, updatedCT.Down)
	}
	if !updatedCT.Enable {
		t.Fatalf("expected enable=true after traffic reset, got enable=false")
	}
}

func TestExhaustedUserUpdateThenResetTrafficSequence(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	ncService := &NodeClientService{}
	inboundService := &InboundService{}

	initialSettings := map[string]interface{}{
		"clients": []interface{}{
			map[string]interface{}{
				"id":    "880e8400-e29b-41d4-a716-446655440003",
				"email": "sequence@example.com",
				"flow":  "",
			},
		},
	}
	settingsBytes, _ := json.Marshal(initialSettings)

	inbound := &model.Inbound{
		UserId:   1,
		Tag:      "inbound-test-4",
		Port:     10004,
		Protocol: model.VLESS,
		Settings: string(settingsBytes),
		Enable:   true,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("failed to create inbound: %v", err)
	}

	quota := int64(20 * 1024 * 1024 * 1024)
	nc := &model.NodeClient{
		Email:      "sequence@example.com",
		UUID:       "880e8400-e29b-41d4-a716-446655440003",
		TotalGB:    quota,
		ExpiryTime: time.Now().Add(12 * time.Hour).Unix() * 1000,
		Enable:     true,
	}
	if err := db.Create(nc).Error; err != nil {
		t.Fatalf("failed to create node client: %v", err)
	}

	link := &model.NodeClientLink{
		NodeClientId: nc.Id,
		InboundId:    inbound.Id,
	}
	if err := db.Create(link).Error; err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	// Client has used full quota
	ct := &xray.ClientTraffic{
		InboundId:    inbound.Id,
		Enable:       false,
		Email:        nc.Email,
		Up:           quota / 2,
		Down:         quota / 2,
		Total:        quota,
		NodeClientId: &nc.Id,
	}
	if err := db.Create(ct).Error; err != nil {
		t.Fatalf("failed to create client traffic: %v", err)
	}

	// 1. Admin calls Update client details (e.g. extending expiry to +30 days)
	nc.ExpiryTime = time.Now().Add(30 * 24 * time.Hour).Unix() * 1000
	nc.Comment = "monthly renewal"
	if _, err := ncService.Update(nc); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 2. Admin calls ResetTraffic
	if _, err := ncService.ResetTraffic(nc.Id); err != nil {
		t.Fatalf("ResetTraffic failed: %v", err)
	}

	// 3. Verify ClientTraffic is active
	var updatedCT xray.ClientTraffic
	if err := db.First(&updatedCT, ct.Id).Error; err != nil {
		t.Fatalf("failed to fetch updated client traffic: %v", err)
	}
	if updatedCT.Up != 0 || updatedCT.Down != 0 {
		t.Fatalf("expected up=0, down=0, got up=%d, down=%d", updatedCT.Up, updatedCT.Down)
	}
	if !updatedCT.Enable {
		t.Fatalf("expected client_traffics.enable=true, got false")
	}

	// 4. Verify inbound.Settings contains client
	var updatedInbound model.Inbound
	if err := db.First(&updatedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("failed to fetch updated inbound: %v", err)
	}
	var parsedSettings map[string]interface{}
	_ = json.Unmarshal([]byte(updatedInbound.Settings), &parsedSettings)
	clients := parsedSettings["clients"].([]interface{})
	if len(clients) == 0 {
		t.Fatalf("inbound.Settings is missing clients!")
	}

	// 5. Verify BuildInboundConfig includes this client!
	cfg, err := inboundService.BuildInboundConfig(&updatedInbound)
	if err != nil {
		t.Fatalf("BuildInboundConfig failed: %v", err)
	}
	var builtSettings map[string]interface{}
	if err := json.Unmarshal(cfg.Settings, &builtSettings); err != nil {
		t.Fatalf("failed to parse built settings: %v", err)
	}
	builtClients, ok := builtSettings["clients"].([]interface{})
	if !ok || len(builtClients) == 0 {
		t.Fatalf("BuildInboundConfig did not include client! settings=%s", string(cfg.Settings))
	}
	clientFound := false
	for _, bc := range builtClients {
		if bcm, ok := bc.(map[string]interface{}); ok {
			if bcm["email"] == "sequence@example.com" {
				clientFound = true
				break
			}
		}
	}
	if !clientFound {
		t.Fatalf("BuildInboundConfig excluded the reset client! settings=%s", string(cfg.Settings))
	}
}

func TestNormalizeLegacyOutboundsFreedomNoises(t *testing.T) {
	// 1. Empty noises array is stripped
	inputEmpty := []byte(`[{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"AsIs","noises":[],"fragment":{}}}]`)
	normalized, err := normalizeLegacyOutbounds(inputEmpty)
	if err != nil {
		t.Fatalf("normalizeLegacyOutbounds failed: %v", err)
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(normalized, &out); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	settings := out[0]["settings"].(map[string]interface{})
	if _, exists := settings["noises"]; exists {
		t.Errorf("expected 'noises' to be removed from freedom settings")
	}
	if _, exists := settings["fragment"]; exists {
		t.Errorf("expected 'fragment' to be removed from freedom settings")
	}

	// 2. Active noises are migrated to streamSettings.finalmask.udp
	inputActive := []byte(`[{"protocol":"freedom","tag":"direct","settings":{"noises":[{"type":"rand","packet":"10-20","delay":"10-16"}]}}]`)
	normActive, err := normalizeLegacyOutbounds(inputActive)
	if err != nil {
		t.Fatalf("normalizeLegacyOutbounds active failed: %v", err)
	}
	var outActive []map[string]interface{}
	if err := json.Unmarshal(normActive, &outActive); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	activeSettings := outActive[0]["settings"].(map[string]interface{})
	if _, exists := activeSettings["noises"]; exists {
		t.Errorf("expected 'noises' to be deleted after migration")
	}
	streamSettings := outActive[0]["streamSettings"].(map[string]interface{})
	finalmask := streamSettings["finalmask"].(map[string]interface{})
	udp := finalmask["udp"].([]interface{})
	if len(udp) != 1 {
		t.Fatalf("expected 1 udp mask, got %d", len(udp))
	}
	mask := udp[0].(map[string]interface{})
	if mask["type"] != "noise" {
		t.Errorf("expected mask type 'noise', got %v", mask["type"])
	}
}

func TestTgBotCpuNotifySetting(t *testing.T) {
	setupTestDB(t)

	settingService := &SettingService{}

	// Default should be true
	enabled, err := settingService.GetTgBotCpuNotify()
	if err != nil {
		t.Fatalf("GetTgBotCpuNotify failed: %v", err)
	}
	if !enabled {
		t.Errorf("expected default TgBotCpuNotify to be true, got false")
	}

	// Disable it
	if err := settingService.SetTgBotCpuNotify(false); err != nil {
		t.Fatalf("SetTgBotCpuNotify failed: %v", err)
	}

	// Verify it returns false
	enabled, err = settingService.GetTgBotCpuNotify()
	if err != nil {
		t.Fatalf("GetTgBotCpuNotify after disable failed: %v", err)
	}
	if enabled {
		t.Errorf("expected TgBotCpuNotify to be false, got true")
	}

	// Verify GetAllSetting reflects the disabled state
	allSetting, err := settingService.GetAllSetting()
	if err != nil {
		t.Fatalf("GetAllSetting failed: %v", err)
	}
	if allSetting.TgBotCpuNotify {
		t.Errorf("expected allSetting.TgBotCpuNotify to be false, got true")
	}
}
