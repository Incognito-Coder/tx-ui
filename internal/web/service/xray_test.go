package service

import (
	"encoding/json"
	"testing"
)

func TestNormalizeStreamSettings_XDNS(t *testing.T) {
	streamJSON := `{
		"network": "tcp",
		"finalmask": {
			"udp": [
				{
					"type": "xdns",
					"settings": {
						"domains": ["example.com", "test.org"],
						"resolvers": ["udp://8.8.8.8:53"],
						"extraPoll": 1
					}
				}
			]
		}
	}`

	var stream map[string]interface{}
	if err := json.Unmarshal([]byte(streamJSON), &stream); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	changed := NormalizeStreamSettings(stream)
	if !changed {
		t.Fatalf("expected NormalizeStreamSettings to report changed = true")
	}

	finalmask := stream["finalmask"].(map[string]interface{})
	udpMasks := finalmask["udp"].([]interface{})
	xdnsMask := udpMasks[0].(map[string]interface{})
	settings := xdnsMask["settings"].(map[string]interface{})

	domains, ok := settings["domains"].([]map[string]interface{})
	if !ok || len(domains) != 1 {
		t.Fatalf("expected normalized domains array of length 1, got %v", settings["domains"])
	}
	names := domains[0]["names"].([]string)
	if len(names) != 2 || names[0] != "example.com" || names[1] != "test.org" {
		t.Fatalf("expected normalized names [example.com, test.org], got %v", names)
	}

	resolvers, ok := settings["resolvers"].([]map[string]interface{})
	if !ok || len(resolvers) != 1 {
		t.Fatalf("expected normalized resolvers array of length 1, got %v", settings["resolvers"])
	}
	addrs := resolvers[0]["addrs"].([]string)
	if len(addrs) != 1 || addrs[0] != "udp://8.8.8.8:53" {
		t.Fatalf("expected normalized addrs [udp://8.8.8.8:53], got %v", addrs)
	}
}

func TestNormalizeStreamSettings_NilAndEmpty(t *testing.T) {
	if NormalizeStreamSettings(nil) {
		t.Fatalf("expected false for nil stream")
	}
	empty := map[string]interface{}{}
	if NormalizeStreamSettings(empty) {
		t.Fatalf("expected false for empty stream")
	}
}
