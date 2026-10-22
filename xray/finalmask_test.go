package xray

import (
	"encoding/json"
	"testing"
)

func TestTakeUDPHop(t *testing.T) {
	t.Run("nil stream", func(t *testing.T) {
		got := TakeUDPHop(nil)
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("no finalmask", func(t *testing.T) {
		stream := map[string]any{"network": "tcp"}
		got := TakeUDPHop(stream)
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("udpHop only in quicParams", func(t *testing.T) {
		stream := map[string]any{
			"network": "hysteria",
			"finalmask": map[string]any{
				"quicParams": map[string]any{
					"udpHop": map[string]any{
						"ports":    "10000-20000",
						"interval": "30",
					},
				},
			},
		}
		got := TakeUDPHop(stream)
		if got == nil {
			t.Fatalf("expected udpHop, got nil")
		}
		if got["ports"] != "10000-20000" || got["interval"] != "30" {
			t.Fatalf("unexpected udpHop content: %v", got)
		}
		// quicParams was only udpHop, so quicParams and finalmask should be deleted
		if _, ok := stream["finalmask"]; ok {
			t.Fatalf("expected finalmask to be removed, but found %v", stream["finalmask"])
		}
	})

	t.Run("udpHop with other quicParams", func(t *testing.T) {
		stream := map[string]any{
			"network": "hysteria",
			"finalmask": map[string]any{
				"quicParams": map[string]any{
					"congestion": "bbr",
					"udpHop": map[string]any{
						"ports": "10000-20000",
					},
				},
			},
		}
		got := TakeUDPHop(stream)
		if got == nil || got["ports"] != "10000-20000" {
			t.Fatalf("expected udpHop ports 10000-20000, got %v", got)
		}
		fm, ok := stream["finalmask"].(map[string]any)
		if !ok {
			t.Fatalf("expected finalmask to remain")
		}
		qp, ok := fm["quicParams"].(map[string]any)
		if !ok || qp["congestion"] != "bbr" {
			t.Fatalf("expected congestion to remain in quicParams, got %v", qp)
		}
		if _, ok := qp["udpHop"]; ok {
			t.Fatalf("expected udpHop to be deleted from quicParams")
		}
	})
}

func TestMoveUDPHopToMask(t *testing.T) {
	t.Run("quicParams.udpHop moved to udphop mask at position 0", func(t *testing.T) {
		stream := map[string]any{
			"finalmask": map[string]any{
				"quicParams": map[string]any{
					"udpHop": map[string]any{
						"ports":    "20000-30000",
						"interval": "15s",
					},
				},
				"udp": []any{
					map[string]any{
						"type":     "salamander",
						"settings": map[string]any{"password": "secret"},
					},
				},
			},
		}

		moved := MoveUDPHopToMask(stream)
		if !moved {
			t.Fatalf("expected MoveUDPHopToMask to return true")
		}

		fm := stream["finalmask"].(map[string]any)
		udp := fm["udp"].([]any)
		if len(udp) != 2 {
			t.Fatalf("expected 2 udp masks, got %d", len(udp))
		}

		first := udp[0].(map[string]any)
		if first["type"] != "udphop" {
			t.Fatalf("expected first mask to be udphop, got %v", first["type"])
		}
		settings := first["settings"].(map[string]any)
		if settings["remotePorts"] != "20000-30000" || settings["interval"] != "15s" {
			t.Fatalf("unexpected settings: %v", settings)
		}
	})

	t.Run("idempotent when udphop already in udp masks", func(t *testing.T) {
		stream := map[string]any{
			"finalmask": map[string]any{
				"quicParams": map[string]any{
					"udpHop": map[string]any{
						"ports": "20000-30000",
					},
				},
				"udp": []any{
					map[string]any{
						"type":     "udphop",
						"settings": map[string]any{"remotePorts": "20000-30000"},
					},
				},
			},
		}

		MoveUDPHopToMask(stream)
		fm := stream["finalmask"].(map[string]any)
		udp := fm["udp"].([]any)
		if len(udp) != 1 {
			t.Fatalf("expected exactly 1 mask, got %d", len(udp))
		}
	})
}

func TestStripFilterMasks(t *testing.T) {
	t.Run("strip filter from both tcp and udp", func(t *testing.T) {
		stream := map[string]any{
			"finalmask": map[string]any{
				"tcp": []any{
					map[string]any{
						"type":     "filter",
						"settings": map[string]any{"allowedSourceIPs": []any{"1.2.3.4"}},
					},
					map[string]any{
						"type":     "fragment",
						"settings": map[string]any{"packets": "tlshello"},
					},
				},
				"udp": []any{
					map[string]any{
						"type":     "filter",
						"settings": map[string]any{"blockedSourceIPs": []any{"5.6.7.8"}},
					},
				},
			},
		}

		changed := StripFilterMasks(stream)
		if !changed {
			t.Fatalf("expected StripFilterMasks to return true")
		}

		fm := stream["finalmask"].(map[string]any)
		tcp := fm["tcp"].([]any)
		if len(tcp) != 1 || tcp[0].(map[string]any)["type"] != "fragment" {
			t.Fatalf("expected only fragment mask in tcp, got %v", tcp)
		}
		if _, ok := fm["udp"]; ok {
			t.Fatalf("expected udp list to be removed entirely, found %v", fm["udp"])
		}
	})

	t.Run("entire finalmask removed when only filters exist", func(t *testing.T) {
		stream := map[string]any{
			"finalmask": map[string]any{
				"tcp": []any{
					map[string]any{"type": "filter"},
				},
			},
		}

		changed := StripFilterMasks(stream)
		if !changed {
			t.Fatalf("expected changed to be true")
		}
		if _, ok := stream["finalmask"]; ok {
			t.Fatalf("expected finalmask to be completely removed")
		}
	})
}

func TestUpgradeXDNSMasks(t *testing.T) {
	t.Run("upgrade legacy string array domains and resolvers", func(t *testing.T) {
		stream := map[string]any{
			"finalmask": map[string]any{
				"udp": []any{
					map[string]any{
						"type": "xdns",
						"settings": map[string]any{
							"domains":   []any{"example.com", "test.org"},
							"resolvers": []any{"8.8.8.8:53", "1.1.1.1:53"},
							"types":     []any{float64(1), float64(28)},
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
		mask := udp[0].(map[string]any)
		settings := mask["settings"].(map[string]any)

		domains := settings["domains"].([]any)
		if len(domains) != 2 {
			t.Fatalf("expected 2 upgraded domains, got %d", len(domains))
		}

		resolvers := settings["resolvers"].([]any)
		if len(resolvers) != 1 {
			t.Fatalf("expected 1 resolver group, got %d", len(resolvers))
		}

		// Ensure output is valid JSON
		raw, err := json.Marshal(stream)
		if err != nil {
			t.Fatalf("failed to marshal upgraded stream: %v", err)
		}
		if len(raw) == 0 {
			t.Fatalf("empty json output")
		}
	})
}
