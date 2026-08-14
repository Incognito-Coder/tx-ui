package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
	"x-ui/internal/util/json_util"
)

func TestNormalizeLegacyVMessUser(t *testing.T) {
	for _, legacy := range []string{"none", "zero", "plain", ""} {
		user := map[string]interface{}{"security": legacy}
		normalizeLegacyVMessUser(user)
		if user["security"] != "auto" {
			t.Fatalf("security %q was not normalized: %#v", legacy, user)
		}
	}

	user := map[string]interface{}{"security": "aes-128-gcm"}
	normalizeLegacyVMessUser(user)
	if user["security"] != "aes-128-gcm" {
		t.Fatalf("supported security was changed: %#v", user)
	}
}

func TestNormalizeLegacyVMessOutbounds(t *testing.T) {
	raw := json_util.RawMessage(`[
		{"protocol":"vmess","settings":{"vnext":[{"users":[{"id":"1","security":"zero"}]}]}},
		{"protocol":"trojan","settings":{"servers":[{"security":"none"}]}}
	]`)

	normalized, err := normalizeLegacyVMessOutbounds(raw)
	if err != nil {
		t.Fatalf("normalize outbounds: %v", err)
	}

	var outbounds []map[string]interface{}
	if err := json.Unmarshal(normalized, &outbounds); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	vmessUser := outbounds[0]["settings"].(map[string]interface{})["vnext"].([]interface{})[0].(map[string]interface{})["users"].([]interface{})[0].(map[string]interface{})
	if vmessUser["security"] != "auto" {
		t.Fatalf("legacy VMess security was not normalized: %#v", vmessUser)
	}
	trojanServer := outbounds[1]["settings"].(map[string]interface{})["servers"].([]interface{})[0].(map[string]interface{})
	if trojanServer["security"] != "none" {
		t.Fatalf("non-VMess outbound was changed: %#v", trojanServer)
	}
}

func TestMergeIntoInboundConfigKeepsClientAndLinkFieldsSeparate(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "node-client.db")); err != nil {
		t.Fatalf("initialize test database: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	nodeClient := model.NodeClient{
		Email:    "node@example.com",
		SubID:    "node-sub-id",
		UUID:     "00112233-4455-6677-8899-aabbccddeeff",
		Security: "aes-128-gcm",
		Flow:     "client-flow",
		Enable:   true,
	}
	if err := database.GetDB().Create(&nodeClient).Error; err != nil {
		t.Fatalf("create node client: %v", err)
	}
	link := model.NodeClientLink{NodeClientId: nodeClient.Id, InboundId: 42, Flow: "link-flow"}
	if err := database.GetDB().Create(&link).Error; err != nil {
		t.Fatalf("create node client link: %v", err)
	}

	merged, err := (&NodeClientService{}).MergeIntoInboundConfig(42, nil)
	if err != nil {
		t.Fatalf("merge node client: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("expected one merged client, got %d", len(merged))
	}
	if merged[0].ID != nodeClient.UUID || merged[0].Security != nodeClient.Security || merged[0].Flow != link.Flow {
		t.Fatalf("joined fields were mixed up: %#v", merged[0])
	}
}
