package model

import (
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
)

func TestWireguardClientsAsPeers(t *testing.T) {
	inputSettings := `{
		"secretKey": "aW52YWxpZHNlY3JldGtleWZvcmhlbGxvMTIzNDU2Nzg5MDE=",
		"address": "10.0.0.1/24",
		"clients": [
			{
				"email": "user1@test.com",
				"publicKey": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
				"allowedIPs": ["10.0.0.2"],
				"enable": true
			},
			{
				"email": "user2@test.com",
				"id": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTc=",
				"allowedIPs": "10.0.0.3",
				"psk": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTg=",
				"enable": true
			},
			{
				"email": "disabled@test.com",
				"publicKey": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTk=",
				"allowedIPs": ["10.0.0.4"],
				"enable": false
			}
		]
	}`

	result := wireguardClientsAsPeers(inputSettings)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Address must be a slice of strings
	addresses, ok := parsed["address"].([]any)
	if !ok || len(addresses) != 1 || addresses[0] != "10.0.0.1/24" {
		t.Errorf("expected address to be ['10.0.0.1/24'], got %v", parsed["address"])
	}

	// Peers should have 2 active peers (disabled filtered out)
	peers, ok := parsed["peers"].([]any)
	if !ok || len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %v", len(peers))
	}

	// First peer allowedIPs must have /32
	p1 := peers[0].(map[string]any)
	aips1, ok := p1["allowedIPs"].([]any)
	if !ok || len(aips1) != 1 || aips1[0] != "10.0.0.2/32" {
		t.Errorf("expected peer 1 allowedIPs to be ['10.0.0.2/32'], got %v", p1["allowedIPs"])
	}

	// Second peer allowedIPs must have /32 and preSharedKey mapped from psk
	p2 := peers[1].(map[string]any)
	aips2, ok := p2["allowedIPs"].([]any)
	if !ok || len(aips2) != 1 || aips2[0] != "10.0.0.3/32" {
		t.Errorf("expected peer 2 allowedIPs to be ['10.0.0.3/32'], got %v", p2["allowedIPs"])
	}
	if p2["preSharedKey"] != "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTg=" {
		t.Errorf("expected peer 2 preSharedKey to be mapped from psk")
	}
	if p2["psk"] != "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTg=" {
		t.Errorf("expected peer 2 psk to be mapped from psk")
	}

	// Verify Xray infra/conf accepts this JSON without error
	inboundDetourJSON := `{
		"protocol": "wireguard",
		"port": 51820,
		"tag": "inbound-51820",
		"settings": ` + result + `
	}`

	confDetour := new(conf.InboundDetourConfig)
	if err := json.Unmarshal([]byte(inboundDetourJSON), confDetour); err != nil {
		t.Fatalf("failed to unmarshal into Xray InboundDetourConfig: %v", err)
	}
	if _, err := confDetour.Build(); err != nil {
		t.Fatalf("Xray confDetour.Build() failed: %v", err)
	}
}
