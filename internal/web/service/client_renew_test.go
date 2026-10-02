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
	dbPath := filepath.Join(t.TempDir(), "test_renew.db")
	err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init test db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.CloseDB()
	})
}

func TestUpdateInboundClient_RenewsExpiryTime(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	oldExpiry := time.Now().Unix()*1000 - 3600000 // 1 hour ago (expired)
	newExpiry := time.Now().Unix()*1000 + 86400000 // 1 day in future (renewed)

	// 1. Create an inbound with a client
	inboundSettings := map[string]interface{}{
		"clients": []map[string]interface{}{
			{
				"id":         "client-uuid-1",
				"email":      "renew_user@example.com",
				"expiryTime": oldExpiry,
				"totalGB":    1000,
				"enable":     false,
				"subId":      "sub123",
			},
		},
	}
	settingsBytes, _ := json.Marshal(inboundSettings)

	inbound := &model.Inbound{
		UserId:   1,
		Up:       0,
		Down:     0,
		Total:    0,
		Remark:   "test-vless",
		Enable:   true,
		Protocol: model.VLESS,
		Port:     30001,
		Settings: string(settingsBytes),
		Tag:      "inbound-30001",
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("Failed to create inbound: %v", err)
	}

	// 2. Create the corresponding NodeClient
	nc := &model.NodeClient{
		UUID:       "client-uuid-1",
		Email:      "renew_user@example.com",
		ExpiryTime: oldExpiry,
		TotalGB:    1000,
		Enable:     false,
		SubID:      "sub123",
	}
	if err := db.Create(nc).Error; err != nil {
		t.Fatalf("Failed to create NodeClient: %v", err)
	}

	// 3. Create the corresponding NodeClientLink
	link := &model.NodeClientLink{
		NodeClientId: nc.Id,
		InboundId:    inbound.Id,
	}
	if err := db.Create(link).Error; err != nil {
		t.Fatalf("Failed to create NodeClientLink: %v", err)
	}

	// 4. Create the corresponding ClientTraffic
	ct := &xray.ClientTraffic{
		InboundId:    inbound.Id,
		NodeClientId: &nc.Id,
		Email:        "renew_user@example.com",
		Up:           100,
		Down:         200,
		Total:        1000,
		ExpiryTime:   oldExpiry,
		Enable:       false,
	}
	if err := db.Create(ct).Error; err != nil {
		t.Fatalf("Failed to create ClientTraffic: %v", err)
	}

	// 5. Perform renewal via UpdateInboundClient
	renewSettings := map[string]interface{}{
		"clients": []map[string]interface{}{
			{
				"id":         "client-uuid-1",
				"email":      "renew_user@example.com",
				"expiryTime": newExpiry,
				"totalGB":    5000,
				"enable":     true,
				"subId":      "sub123",
			},
		},
	}
	renewSettingsBytes, _ := json.Marshal(renewSettings)
	updateInboundData := &model.Inbound{
		Id:       inbound.Id,
		Settings: string(renewSettingsBytes),
	}

	inboundService := &InboundService{}
	_, err := inboundService.UpdateInboundClient(updateInboundData, "client-uuid-1")
	if err != nil {
		t.Fatalf("UpdateInboundClient failed: %v", err)
	}

	// 6. Verify NodeClient was updated
	var updatedNC model.NodeClient
	if err := db.First(&updatedNC, nc.Id).Error; err != nil {
		t.Fatalf("Failed to find updated NodeClient: %v", err)
	}
	if updatedNC.ExpiryTime != newExpiry {
		t.Errorf("NodeClient ExpiryTime not updated: got %d, want %d", updatedNC.ExpiryTime, newExpiry)
	}
	if updatedNC.TotalGB != 5000 {
		t.Errorf("NodeClient TotalGB not updated: got %d, want 5000", updatedNC.TotalGB)
	}
	if !updatedNC.Enable {
		t.Errorf("NodeClient Enable not updated to true")
	}

	// 7. Verify ClientTraffic was updated
	var updatedCT xray.ClientTraffic
	if err := db.First(&updatedCT, ct.Id).Error; err != nil {
		t.Fatalf("Failed to find updated ClientTraffic: %v", err)
	}
	if updatedCT.ExpiryTime != newExpiry {
		t.Errorf("ClientTraffic ExpiryTime not updated: got %d, want %d", updatedCT.ExpiryTime, newExpiry)
	}
	if updatedCT.Total != 5000 {
		t.Errorf("ClientTraffic Total not updated: got %d, want 5000", updatedCT.Total)
	}
	if !updatedCT.Enable {
		t.Errorf("ClientTraffic Enable not updated to true")
	}

	// 8. Verify Inbound Settings JSON was updated
	var updatedInbound model.Inbound
	if err := db.First(&updatedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("Failed to find updated Inbound: %v", err)
	}
	var parsedSettings map[string]interface{}
	_ = json.Unmarshal([]byte(updatedInbound.Settings), &parsedSettings)
	clientsList := parsedSettings["clients"].([]interface{})
	client0 := clientsList[0].(map[string]interface{})
	expiryFromJSON := int64(client0["expiryTime"].(float64))
	if expiryFromJSON != newExpiry {
		t.Errorf("Inbound Settings JSON ExpiryTime mismatch: got %d, want %d", expiryFromJSON, newExpiry)
	}
}

func TestNodeClientServiceUpdate_RenewsExpiryTimeAndSyncsInbound(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	oldExpiry := time.Now().Unix()*1000 - 3600000 // 1 hour ago (expired)
	newExpiry := time.Now().Unix()*1000 + 86400000 // 1 day in future (renewed)

	// 1. Create an inbound with a client
	inboundSettings := map[string]interface{}{
		"clients": []map[string]interface{}{
			{
				"id":         "client-uuid-2",
				"email":      "renew_nc@example.com",
				"expiryTime": oldExpiry,
				"totalGB":    1000,
				"enable":     false,
				"subId":      "sub456",
			},
		},
	}
	settingsBytes, _ := json.Marshal(inboundSettings)

	inbound := &model.Inbound{
		UserId:   1,
		Up:       0,
		Down:     0,
		Total:    0,
		Remark:   "test-vless-2",
		Enable:   true,
		Protocol: model.VLESS,
		Port:     30002,
		Settings: string(settingsBytes),
		Tag:      "inbound-30002",
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("Failed to create inbound: %v", err)
	}

	// 2. Create the corresponding NodeClient
	nc := &model.NodeClient{
		UUID:       "client-uuid-2",
		Email:      "renew_nc@example.com",
		ExpiryTime: oldExpiry,
		TotalGB:    1000,
		Enable:     false,
		SubID:      "sub456",
	}
	if err := db.Create(nc).Error; err != nil {
		t.Fatalf("Failed to create NodeClient: %v", err)
	}

	// 3. Create the corresponding NodeClientLink
	link := &model.NodeClientLink{
		NodeClientId: nc.Id,
		InboundId:    inbound.Id,
	}
	if err := db.Create(link).Error; err != nil {
		t.Fatalf("Failed to create NodeClientLink: %v", err)
	}

	// 4. Create the corresponding ClientTraffic
	ct := &xray.ClientTraffic{
		InboundId:    inbound.Id,
		NodeClientId: &nc.Id,
		Email:        "renew_nc@example.com",
		Up:           10,
		Down:         20,
		Total:        1000,
		ExpiryTime:   oldExpiry,
		Enable:       false,
	}
	if err := db.Create(ct).Error; err != nil {
		t.Fatalf("Failed to create ClientTraffic: %v", err)
	}

	// 5. Update NodeClient via NodeClientService.Update (Renewal)
	nc.ExpiryTime = newExpiry
	nc.Enable = true
	nc.TotalGB = 6000

	ncService := &NodeClientService{}
	_, err := ncService.Update(nc)
	if err != nil {
		t.Fatalf("NodeClientService.Update failed: %v", err)
	}

	// 6. Verify NodeClient in DB
	var updatedNC model.NodeClient
	if err := db.First(&updatedNC, nc.Id).Error; err != nil {
		t.Fatalf("Failed to find updated NodeClient: %v", err)
	}
	if updatedNC.ExpiryTime != newExpiry {
		t.Errorf("NodeClient ExpiryTime not updated: got %d, want %d", updatedNC.ExpiryTime, newExpiry)
	}
	if updatedNC.TotalGB != 6000 {
		t.Errorf("NodeClient TotalGB not updated: got %d, want 6000", updatedNC.TotalGB)
	}
	if !updatedNC.Enable {
		t.Errorf("NodeClient Enable not updated to true")
	}

	// 7. Verify ClientTraffic in DB
	var updatedCT xray.ClientTraffic
	if err := db.First(&updatedCT, ct.Id).Error; err != nil {
		t.Fatalf("Failed to find updated ClientTraffic: %v", err)
	}
	if updatedCT.ExpiryTime != newExpiry {
		t.Errorf("ClientTraffic ExpiryTime not updated: got %d, want %d", updatedCT.ExpiryTime, newExpiry)
	}
	if updatedCT.Total != 6000 {
		t.Errorf("ClientTraffic Total not updated: got %d, want 6000", updatedCT.Total)
	}
	if !updatedCT.Enable {
		t.Errorf("ClientTraffic Enable not updated to true")
	}

	// 8. Verify Inbound Settings JSON has updated expiryTime
	var updatedInbound model.Inbound
	if err := db.First(&updatedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("Failed to find updated Inbound: %v", err)
	}
	var parsedSettings map[string]interface{}
	_ = json.Unmarshal([]byte(updatedInbound.Settings), &parsedSettings)
	clientsList := parsedSettings["clients"].([]interface{})
	if len(clientsList) == 0 {
		t.Fatalf("No clients found in updated inbound settings")
	}
	client0 := clientsList[0].(map[string]interface{})
	expiryFromJSON := int64(client0["expiryTime"].(float64))
	if expiryFromJSON != newExpiry {
		t.Errorf("Inbound Settings JSON ExpiryTime mismatch: got %d, want %d", expiryFromJSON, newExpiry)
	}
}
