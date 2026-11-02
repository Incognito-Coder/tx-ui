package sub

import (
	"encoding/json"
	"testing"
	"x-ui/internal/database/model"
)

func TestSubJsonService_GenWireguard_PresharedKey(t *testing.T) {
	service := &SubJsonService{
		SubService: &SubService{
			address: "127.0.0.1",
		},
	}

	inbound := &model.Inbound{
		Protocol: model.WireGuard,
		Port:     51820,
		Listen:   "0.0.0.0",
		Settings: `{"secretKey":"aW52YWxpZHNlY3JldGtleWZvcmhlbGxvMTIzNDU2Nzg5MDE=","pubKey":"YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY="}`,
	}

	pskVal := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTg="

	// Test 1: client with PreSharedKey
	client1 := model.Client{
		Email:        "client1@test.com",
		PrivateKey:   "cHJpdmF0ZWtleWZvcmNsaWVudDEyMzQ1Njc4OTAxMjM0NTY=",
		PublicKey:    "cHVibGlja2V5Zm9yY2xpZW50MTIzNDU2Nzg5MDEyMzQ1Njc=",
		PreSharedKey: pskVal,
	}

	rawMsg1 := service.genWireguard(inbound, client1)
	var outbound1 map[string]any
	if err := json.Unmarshal([]byte(rawMsg1), &outbound1); err != nil {
		t.Fatalf("failed to unmarshal outbound: %v", err)
	}

	settings1, ok := outbound1["settings"].(map[string]any)
	if !ok {
		t.Fatalf("expected settings map, got %v", outbound1["settings"])
	}
	peers1, ok := settings1["peers"].([]any)
	if !ok || len(peers1) == 0 {
		t.Fatalf("expected peers list, got %v", settings1["peers"])
	}
	peer1 := peers1[0].(map[string]any)
	if peer1["preSharedKey"] != pskVal {
		t.Errorf("expected peer preSharedKey %q, got %q", pskVal, peer1["preSharedKey"])
	}
	if peer1["psk"] != pskVal {
		t.Errorf("expected peer psk %q, got %q", pskVal, peer1["psk"])
	}

	// Test 2: client with Psk (empty PreSharedKey)
	client2 := model.Client{
		Email:      "client2@test.com",
		PrivateKey: "cHJpdmF0ZWtleWZvcmNsaWVudDEyMzQ1Njc4OTAxMjM0NTY=",
		PublicKey:  "cHVibGlja2V5Zm9yY2xpZW50MTIzNDU2Nzg5MDEyMzQ1Njc=",
		Psk:        pskVal,
	}

	rawMsg2 := service.genWireguard(inbound, client2)
	var outbound2 map[string]any
	if err := json.Unmarshal([]byte(rawMsg2), &outbound2); err != nil {
		t.Fatalf("failed to unmarshal outbound: %v", err)
	}
	settings2 := outbound2["settings"].(map[string]any)
	peer2 := settings2["peers"].([]any)[0].(map[string]any)
	if peer2["preSharedKey"] != pskVal {
		t.Errorf("expected peer preSharedKey %q, got %q", pskVal, peer2["preSharedKey"])
	}
	if peer2["psk"] != pskVal {
		t.Errorf("expected peer psk %q, got %q", pskVal, peer2["psk"])
	}
}
