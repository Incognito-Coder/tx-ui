package xray

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTakeUDPHopDropsEmptyContainers(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"quicParams": map[string]any{"udpHop": map[string]any{"ports": "20000-30000"}},
		},
	}
	hop := TakeUDPHop(stream)
	if hop["ports"] != "20000-30000" {
		t.Fatalf("hop = %v", hop)
	}
	if _, ok := stream["finalmask"]; ok {
		t.Fatalf("finalmask was not removed: %v", stream)
	}
}

func TestTakeUDPHopKeepsOtherQuicParams(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"quicParams": map[string]any{"congestion": "bbr", "udpHop": map[string]any{"ports": "20000"}},
		},
	}
	TakeUDPHop(stream)
	fm := stream["finalmask"].(map[string]any)
	qp := fm["quicParams"].(map[string]any)
	if qp["congestion"] != "bbr" || qp["udpHop"] != nil {
		t.Fatalf("quicParams = %v", qp)
	}
}

func TestMoveUDPHopToMaskPutsMaskFirst(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"udp": []any{map[string]any{"type": "salamander"}},
			"quicParams": map[string]any{
				"udpHop": map[string]any{"ports": "20000-30000", "interval": "10s"},
			},
		},
	}
	if !MoveUDPHopToMask(stream) {
		t.Fatal("MoveUDPHopToMask reported no change")
	}
	udp := stream["finalmask"].(map[string]any)["udp"].([]any)
	first := udp[0].(map[string]any)
	if first["type"] != "udphop" {
		t.Fatalf("first mask = %v", first)
	}
	settings := first["settings"].(map[string]any)
	if settings["remotePorts"] != "20000-30000" || settings["interval"] != "10s" {
		t.Fatalf("settings = %v", settings)
	}
}

func TestMoveUDPHopToMaskKeepsExistingMask(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"udp": []any{map[string]any{"type": "udphop", "settings": map[string]any{"remotePorts": "1000"}}},
		},
	}
	MoveUDPHopToMask(stream)
	udp := stream["finalmask"].(map[string]any)["udp"].([]any)
	if len(udp) != 1 {
		t.Fatalf("udp = %v", udp)
	}
}

func TestUpgradeXDNSMasks(t *testing.T) {
	streamJSON := `{
		"finalmask": {
			"udp": [
				{
					"type": "xdns",
					"settings": {
						"domain": "example.com",
						"resolvers": ["udp://1.1.1.1:53"]
					}
				}
			]
		}
	}`
	var stream map[string]any
	json.Unmarshal([]byte(streamJSON), &stream)

	if !UpgradeXDNSMasks(stream) {
		t.Fatal("expected UpgradeXDNSMasks to report changed")
	}

	udp := stream["finalmask"].(map[string]any)["udp"].([]any)
	settings := udp[0].(map[string]any)["settings"].(map[string]any)
	domains := settings["domains"].([]any)
	if len(domains) != 1 {
		t.Fatalf("domains len = %d", len(domains))
	}
	d0 := domains[0].(map[string]any)
	if !reflect.DeepEqual(d0["names"], []any{"example.com"}) {
		t.Fatalf("names = %v", d0["names"])
	}
}

func TestStripFilterMasks(t *testing.T) {
	streamJSON := `{
		"finalmask": {
			"tcp": [
				{"type": "filter", "settings": {"allowedSourceIPs": ["1.1.1.1"]}},
				{"type": "header-custom", "settings": {}}
			],
			"udp": [
				{"type": "filter", "settings": {"blockedSourceIPs": ["2.2.2.2"]}}
			]
		}
	}`
	var stream map[string]any
	json.Unmarshal([]byte(streamJSON), &stream)

	if !StripFilterMasks(stream) {
		t.Fatal("expected StripFilterMasks to report changed")
	}

	fm := stream["finalmask"].(map[string]any)
	tcp := fm["tcp"].([]any)
	if len(tcp) != 1 || tcp[0].(map[string]any)["type"] != "header-custom" {
		t.Fatalf("tcp masks unexpected: %v", tcp)
	}
	if _, ok := fm["udp"]; ok {
		t.Fatalf("expected udp to be deleted when only filter was present, got: %v", fm["udp"])
	}
}

