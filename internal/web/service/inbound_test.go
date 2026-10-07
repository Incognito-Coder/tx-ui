package service

import (
	"encoding/json"
	"testing"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
	"x-ui/xray"
)

func TestBuildInboundConfigExcludesDisabledClients(t *testing.T) {
	inboundService := InboundService{}
	inbound := &model.Inbound{
		Id:       1,
		UserId:   1,
		Tag:      "inbound-1080",
		Port:     1080,
		Protocol: model.VLESS,
		Listen:   "0.0.0.0",
		Enable:   true,
		Settings: `{"clients":[{"id":"uuid-1","email":"user1@example.com"},{"id":"uuid-2","email":"user2@example.com"}]}`,
		ClientStats: []xray.ClientTraffic{
			{InboundId: 1, Email: "user1@example.com", Enable: true},
			{InboundId: 1, Email: "user2@example.com", Enable: false},
		},
	}

	config, err := inboundService.BuildInboundConfig(inbound)
	if err != nil {
		t.Fatalf("BuildInboundConfig returned unexpected error: %v", err)
	}

	if config == nil {
		t.Fatal("expected non-nil config")
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(config.Settings, &settings); err != nil {
		t.Fatalf("failed to unmarshal settings: %v", err)
	}

	clients, ok := settings["clients"].([]interface{})
	if !ok {
		t.Fatal("expected clients array in settings")
	}

	if len(clients) != 1 {
		t.Fatalf("expected 1 active client, got %d", len(clients))
	}

	clientMap, _ := clients[0].(map[string]interface{})
	if email, _ := clientMap["email"].(string); email != "user1@example.com" {
		t.Fatalf("expected active client user1@example.com, got %s", email)
	}
}

func TestHotReloadInboundByTagWhenProcessNotRunning(t *testing.T) {
	inboundService := InboundService{}
	err := inboundService.HotReloadInboundByTag("inbound-1080")
	if err != nil {
		t.Fatalf("expected nil error when Xray process is not running, got %v", err)
	}
}

func TestBuildInboundConfigSanitizesECH(t *testing.T) {
	inboundService := InboundService{}
	inbound := &model.Inbound{
		Id:             2,
		UserId:         1,
		Tag:            "inbound-443",
		Port:           443,
		Protocol:       model.VLESS,
		Listen:         "0.0.0.0",
		Enable:         true,
		Settings:       `{"clients":[{"id":"uuid-1","email":"user1@example.com"}]}`,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"example.com","echServerKeys":"ACC03HO...==\r\n","echConfigList":"AF7+DQBaAA...","echForceQuery":"full","settings":{"fingerprint":"chrome","echConfigList":"AF7+DQBaAA..."}}}`,
	}

	config, err := inboundService.BuildInboundConfig(inbound)
	if err != nil {
		t.Fatalf("BuildInboundConfig returned unexpected error: %v", err)
	}

	var stream map[string]interface{}
	if err := json.Unmarshal(config.StreamSettings, &stream); err != nil {
		t.Fatalf("failed to unmarshal streamSettings: %v", err)
	}

	tlsSettings, ok := stream["tlsSettings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected tlsSettings in streamSettings")
	}

	if echServerKeys, ok := tlsSettings["echServerKeys"].(string); !ok || echServerKeys != "ACC03HO...==" {
		t.Fatalf("expected trimmed echServerKeys 'ACC03HO...==', got '%v'", echServerKeys)
	}

	if _, ok := tlsSettings["settings"]; ok {
		t.Fatal("expected 'settings' to be removed from tlsSettings")
	}

	if _, ok := tlsSettings["echConfigList"]; ok {
		t.Fatal("expected client-only 'echConfigList' to be removed from inbound tlsSettings")
	}

	if _, ok := tlsSettings["echForceQuery"]; ok {
		t.Fatal("expected 'echForceQuery' to be removed from inbound tlsSettings")
	}
}

func TestDelInboundPreservesNodeClientTraffic(t *testing.T) {
	setupTestDB(t)
	db := database.GetDB()
	inboundService := &InboundService{}
	ncService := &NodeClientService{}

	ib1 := &model.Inbound{Id: 101, UserId: 1, Tag: "inbound-101", Port: 1010, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}
	ib2 := &model.Inbound{Id: 102, UserId: 1, Tag: "inbound-102", Port: 1020, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}
	if err := db.Create(ib1).Error; err != nil {
		t.Fatalf("create ib1 failed: %v", err)
	}
	if err := db.Create(ib2).Error; err != nil {
		t.Fatalf("create ib2 failed: %v", err)
	}

	nc := &model.NodeClient{Id: 1, Email: "alice@example.com", SubID: "sub-alice", UUID: "uuid-alice", TotalGB: 100, Enable: true}
	if err := db.Create(nc).Error; err != nil {
		t.Fatalf("create nc failed: %v", err)
	}

	if _, err := ncService.AddLink(1, 101, ""); err != nil {
		t.Fatalf("AddLink ib1 failed: %v", err)
	}
	if _, err := ncService.AddLink(1, 102, ""); err != nil {
		t.Fatalf("AddLink ib2 failed: %v", err)
	}

	// Simulate client usage on inbound 101: Up 100MB, Down 200MB
	if err := db.Model(&xray.ClientTraffic{}).Where("inbound_id = ? AND email = ?", 101, "alice@example.com").
		Updates(map[string]interface{}{"up": int64(104857600), "down": int64(209715200)}).Error; err != nil {
		t.Fatalf("update traffic on ib1 failed: %v", err)
	}

	// Delete inbound 101
	if _, err := inboundService.DelInbound(101); err != nil {
		t.Fatalf("DelInbound failed: %v", err)
	}

	// Verify inbound 102 preserved the traffic
	var ct xray.ClientTraffic
	if err := db.Where("inbound_id = ? AND email = ?", 102, "alice@example.com").First(&ct).Error; err != nil {
		t.Fatalf("failed to query client traffic on ib2: %v", err)
	}
	if ct.Up != 104857600 || ct.Down != 209715200 {
		t.Fatalf("expected preserved traffic Up: 104857600, Down: 209715200, got Up: %d, Down: %d", ct.Up, ct.Down)
	}

	// Verify NodeClient still exists because it's linked to ib2
	var remainingNC model.NodeClient
	if err := db.First(&remainingNC, 1).Error; err != nil {
		t.Fatalf("NodeClient was unexpectedly deleted: %v", err)
	}
}

func TestDelInboundPreservesSameEmailTraffic(t *testing.T) {
	setupTestDB(t)
	db := database.GetDB()
	inboundService := &InboundService{}

	ib1 := &model.Inbound{Id: 201, UserId: 1, Tag: "inbound-201", Port: 2010, Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[{"id":"uuid-1","email":"bob@example.com"}]}`}
	ib2 := &model.Inbound{Id: 202, UserId: 1, Tag: "inbound-202", Port: 2020, Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[{"id":"uuid-2","email":"bob@example.com"}]}`}
	if err := db.Create(ib1).Error; err != nil {
		t.Fatalf("create ib1 failed: %v", err)
	}
	if err := db.Create(ib2).Error; err != nil {
		t.Fatalf("create ib2 failed: %v", err)
	}

	ct1 := &xray.ClientTraffic{InboundId: 201, Email: "bob@example.com", Up: 12000, Down: 34000, Enable: true}
	ct2 := &xray.ClientTraffic{InboundId: 202, Email: "bob@example.com", Up: 1000, Down: 2000, Enable: true}
	if err := db.Create(ct1).Error; err != nil {
		t.Fatalf("create ct1 failed: %v", err)
	}
	if err := db.Create(ct2).Error; err != nil {
		t.Fatalf("create ct2 failed: %v", err)
	}

	if _, err := inboundService.DelInbound(201); err != nil {
		t.Fatalf("DelInbound failed: %v", err)
	}

	var ct2After xray.ClientTraffic
	if err := db.Where("inbound_id = ? AND email = ?", 202, "bob@example.com").First(&ct2After).Error; err != nil {
		t.Fatalf("failed to query client traffic on ib 202: %v", err)
	}
	if ct2After.Up != 12000 || ct2After.Down != 34000 {
		t.Fatalf("expected preserved traffic Up: 12000, Down: 34000, got Up: %d, Down: %d", ct2After.Up, ct2After.Down)
	}
}

func TestDelInboundPreservesSubIDTraffic(t *testing.T) {
	setupTestDB(t)
	db := database.GetDB()
	inboundService := &InboundService{}

	ib1 := &model.Inbound{Id: 301, UserId: 1, Tag: "inbound-301", Port: 3010, Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[{"id":"uuid-1","email":"user1@example.com","subId":"sub-common"}]}`}
	ib2 := &model.Inbound{Id: 302, UserId: 1, Tag: "inbound-302", Port: 3020, Protocol: model.VLESS, Enable: true,
		Settings: `{"clients":[{"id":"uuid-2","email":"user2@example.com","subId":"sub-common"}]}`}
	if err := db.Create(ib1).Error; err != nil {
		t.Fatalf("create ib1 failed: %v", err)
	}
	if err := db.Create(ib2).Error; err != nil {
		t.Fatalf("create ib2 failed: %v", err)
	}

	ct1 := &xray.ClientTraffic{InboundId: 301, Email: "user1@example.com", Up: 5000, Down: 8000, Enable: true}
	ct2 := &xray.ClientTraffic{InboundId: 302, Email: "user2@example.com", Up: 0, Down: 0, Enable: true}
	if err := db.Create(ct1).Error; err != nil {
		t.Fatalf("create ct1 failed: %v", err)
	}
	if err := db.Create(ct2).Error; err != nil {
		t.Fatalf("create ct2 failed: %v", err)
	}

	if _, err := inboundService.DelInbound(301); err != nil {
		t.Fatalf("DelInbound failed: %v", err)
	}

	var ct2After xray.ClientTraffic
	if err := db.Where("inbound_id = ? AND email = ?", 302, "user2@example.com").First(&ct2After).Error; err != nil {
		t.Fatalf("failed to query client traffic on ib 302: %v", err)
	}
	if ct2After.Up != 5000 || ct2After.Down != 8000 {
		t.Fatalf("expected preserved SubID traffic Up: 5000, Down: 8000, got Up: %d, Down: %d", ct2After.Up, ct2After.Down)
	}
}

func TestRemoveLinkPreservesNodeClientIdOnOtherInbounds(t *testing.T) {
	setupTestDB(t)
	db := database.GetDB()
	ncService := &NodeClientService{}

	ib1 := &model.Inbound{Id: 401, UserId: 1, Tag: "inbound-401", Port: 4010, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}
	ib2 := &model.Inbound{Id: 402, UserId: 1, Tag: "inbound-402", Port: 4020, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}
	_ = db.Create(ib1)
	_ = db.Create(ib2)

	nc := &model.NodeClient{Id: 2, Email: "carol@example.com", SubID: "sub-carol", UUID: "uuid-carol", Enable: true}
	_ = db.Create(nc)

	_, _ = ncService.AddLink(2, 401, "")
	_, _ = ncService.AddLink(2, 402, "")

	// Set traffic on inbound 401
	_ = db.Model(&xray.ClientTraffic{}).Where("inbound_id = ? AND email = ?", 401, "carol@example.com").
		Updates(map[string]interface{}{"up": int64(1000), "down": int64(2000)}).Error

	// Remove link to inbound 401
	if _, err := ncService.RemoveLink(2, 401); err != nil {
		t.Fatalf("RemoveLink failed: %v", err)
	}

	var ct402 xray.ClientTraffic
	if err := db.Where("inbound_id = ? AND email = ?", 402, "carol@example.com").First(&ct402).Error; err != nil {
		t.Fatalf("traffic row for ib 402 missing: %v", err)
	}
	if ct402.NodeClientId == nil || *ct402.NodeClientId != 2 {
		t.Fatalf("node_client_id on ib 402 was improperly nulled, got: %v", ct402.NodeClientId)
	}
	if ct402.Up != 1000 || ct402.Down != 2000 {
		t.Fatalf("expected preserved traffic Up: 1000, Down: 2000 on ib 402, got Up: %d, Down: %d", ct402.Up, ct402.Down)
	}
}

func TestDelClientStatScopedToInbound(t *testing.T) {
	setupTestDB(t)
	db := database.GetDB()
	inboundService := &InboundService{}

	ct1 := &xray.ClientTraffic{InboundId: 501, Email: "dave@example.com", Up: 100, Down: 200, Enable: true}
	ct2 := &xray.ClientTraffic{InboundId: 502, Email: "dave@example.com", Up: 300, Down: 400, Enable: true}
	_ = db.Create(ct1)
	_ = db.Create(ct2)

	if err := inboundService.DelClientStat(db, "dave@example.com", 501); err != nil {
		t.Fatalf("DelClientStat failed: %v", err)
	}

	var count int64
	db.Model(&xray.ClientTraffic{}).Where("inbound_id = ? AND email = ?", 501, "dave@example.com").Count(&count)
	if count != 0 {
		t.Fatalf("expected 501 row deleted, count = %d", count)
	}

	db.Model(&xray.ClientTraffic{}).Where("inbound_id = ? AND email = ?", 502, "dave@example.com").Count(&count)
	if count != 1 {
		t.Fatalf("expected 502 row preserved, count = %d", count)
	}
}

