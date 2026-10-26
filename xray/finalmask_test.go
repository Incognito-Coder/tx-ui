package xray

import (
	"reflect"
	"testing"
)

func TestTakeUDPHop(t *testing.T) {
	// Case 1: No finalmask
	stream := map[string]any{}
	if got := TakeUDPHop(stream); got != nil {
		t.Fatalf("expected nil for empty stream, got %v", got)
	}

	// Case 2: finalmask with quicParams.udpHop
	udpHopData := map[string]any{"ports": "10000-20000", "interval": "30s"}
	stream = map[string]any{
		"finalmask": map[string]any{
			"quicParams": map[string]any{
				"udpHop": udpHopData,
			},
		},
	}
	got := TakeUDPHop(stream)
	if !reflect.DeepEqual(got, udpHopData) {
		t.Fatalf("expected %v, got %v", udpHopData, got)
	}
	// quicParams and finalmask should be deleted when empty
	if _, ok := stream["finalmask"]; ok {
		t.Fatalf("expected finalmask to be deleted from stream")
	}

	// Case 3: quicParams has other fields
	stream = map[string]any{
		"finalmask": map[string]any{
			"quicParams": map[string]any{
				"udpHop": udpHopData,
				"other":  "value",
			},
		},
	}
	TakeUDPHop(stream)
	fm := stream["finalmask"].(map[string]any)
	qp := fm["quicParams"].(map[string]any)
	if _, ok := qp["udpHop"]; ok {
		t.Fatalf("expected udpHop to be deleted")
	}
	if qp["other"] != "value" {
		t.Fatalf("expected other to be preserved")
	}
}

func TestMoveUDPHopToMask(t *testing.T) {
	udpHopData := map[string]any{"ports": "1000-2000", "interval": "10s"}
	stream := map[string]any{
		"finalmask": map[string]any{
			"quicParams": map[string]any{
				"udpHop": udpHopData,
			},
		},
	}

	if !MoveUDPHopToMask(stream) {
		t.Fatalf("expected MoveUDPHopToMask to return true")
	}

	fm := stream["finalmask"].(map[string]any)
	udp := fm["udp"].([]any)
	if len(udp) != 1 {
		t.Fatalf("expected 1 udp mask, got %d", len(udp))
	}
	mask := udp[0].(map[string]any)
	if mask["type"] != "udphop" {
		t.Fatalf("expected type udphop, got %v", mask["type"])
	}
	settings := mask["settings"].(map[string]any)
	if settings["remotePorts"] != "1000-2000" || settings["interval"] != "10s" {
		t.Fatalf("unexpected settings: %v", settings)
	}

	// Calling again should not duplicate
	MoveUDPHopToMask(stream)
	udp = fm["udp"].([]any)
	if len(udp) != 1 {
		t.Fatalf("expected still 1 udp mask after second call, got %d", len(udp))
	}
}

func TestUpgradeXDNSMasks(t *testing.T) {
	stream := map[string]any{
		"finalmask": map[string]any{
			"udp": []any{
				map[string]any{
					"type": "xdns",
					"settings": map[string]any{
						"domain": "example.com",
						"resolvers": []any{
							"1.1.1.1:53",
							"example.org:A+8.8.8.8:53",
						},
					},
				},
			},
		},
	}

	changed := UpgradeXDNSMasks(stream)
	if !changed {
		t.Fatalf("expected UpgradeXDNSMasks to report changed=true")
	}

	fm := stream["finalmask"].(map[string]any)
	udp := fm["udp"].([]any)
	settings := udp[0].(map[string]any)["settings"].(map[string]any)

	// Old "domain" key should be deleted
	if _, ok := settings["domain"]; ok {
		t.Fatalf("expected old domain key to be deleted")
	}

	domains, ok := settings["domains"].([]any)
	if !ok || len(domains) == 0 {
		t.Fatalf("expected domains list to be populated")
	}

	resolvers, ok := settings["resolvers"].([]any)
	if !ok || len(resolvers) == 0 {
		t.Fatalf("expected resolvers list to be populated")
	}
}

func TestStripFilterMasks(t *testing.T) {
	// Nil stream
	if StripFilterMasks(nil) {
		t.Fatalf("expected false for nil stream")
	}

	stream := map[string]any{
		"finalmask": map[string]any{
			"tcp": []any{
				map[string]any{"type": "filter", "settings": map[string]any{"blockedSourceIPs": []any{"1.2.3.4"}}},
				map[string]any{"type": "other", "settings": map[string]any{}},
			},
			"udp": []any{
				map[string]any{"type": "filter", "settings": map[string]any{"allowedSourceIPs": []any{"5.6.7.8"}}},
			},
		},
	}

	changed := StripFilterMasks(stream)
	if !changed {
		t.Fatalf("expected StripFilterMasks to return true")
	}

	fm := stream["finalmask"].(map[string]any)
	tcp := fm["tcp"].([]any)
	if len(tcp) != 1 || tcp[0].(map[string]any)["type"] != "other" {
		t.Fatalf("expected only 'other' in tcp, got %v", tcp)
	}
	if _, ok := fm["udp"]; ok {
		t.Fatalf("expected udp to be completely deleted since only filter mask existed")
	}

	// Now strip when all masks are filter: finalmask should be deleted
	stream = map[string]any{
		"finalmask": map[string]any{
			"tcp": []any{
				map[string]any{"type": "filter"},
			},
		},
	}
	StripFilterMasks(stream)
	if _, ok := stream["finalmask"]; ok {
		t.Fatalf("expected finalmask to be deleted from stream when empty")
	}
}
