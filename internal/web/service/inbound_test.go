package service

import (
	"encoding/json"
	"testing"

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
