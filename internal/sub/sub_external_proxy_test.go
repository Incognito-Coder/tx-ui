package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"x-ui/internal/database/model"
	"x-ui/internal/util/wireguard"
)

func TestGenWireguardLinkWithExternalProxy(t *testing.T) {
	sub := NewSubService(false, "-ieo")
	sub.address = "default.server.com"

	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate server keypair: %v", err)
	}

	settingsMap := map[string]interface{}{
		"secretKey": serverPriv,
		"pubKey":    serverPub,
		"mtu":       1420,
		"clients": []map[string]interface{}{
			{
				"email":      "client1@test.com",
				"privateKey": priv,
				"publicKey":  pub,
				"allowedIPs": []string{"10.0.0.2/32"},
			},
		},
	}
	settingsBytes, _ := json.Marshal(settingsMap)

	streamMap := map[string]interface{}{
		"network": "tcp",
		"externalProxy": []map[string]interface{}{
			{
				"dest":   "proxy1.domain.com",
				"port":   51821,
				"remark": "EU-Relay",
			},
			{
				"dest":   "proxy2.domain.com",
				"port":   51822,
				"remark": "US-Relay",
			},
			{
				"dest":   "", // Should fallback to sub.address
				"port":   0,  // Should fallback to inbound.Port
				"remark": "Fallback-Relay",
			},
		},
	}
	streamBytes, _ := json.Marshal(streamMap)

	inbound := &model.Inbound{
		Id:             1,
		Port:           51820,
		Protocol:       model.WireGuard,
		Settings:       string(settingsBytes),
		StreamSettings: string(streamBytes),
		Remark:         "WG-Inbound",
	}

	linkStr := sub.genWireguardLink(inbound, "client1@test.com")
	if linkStr == "" {
		t.Fatalf("expected non-empty wireguard links, got empty")
	}

	links := strings.Split(linkStr, "\n")
	if len(links) != 3 {
		t.Fatalf("expected 3 links for 3 externalProxy items, got %d", len(links))
	}

	// Link 1: proxy1.domain.com:51821
	u1, err := url.Parse(links[0])
	if err != nil {
		t.Fatalf("failed to parse link 1: %v", err)
	}
	if u1.Host != "proxy1.domain.com:51821" {
		t.Errorf("expected host proxy1.domain.com:51821, got %s", u1.Host)
	}
	if !strings.Contains(u1.Fragment, "EU-Relay") {
		t.Errorf("expected fragment to contain EU-Relay, got %s", u1.Fragment)
	}

	// Link 2: proxy2.domain.com:51822
	u2, err := url.Parse(links[1])
	if err != nil {
		t.Fatalf("failed to parse link 2: %v", err)
	}
	if u2.Host != "proxy2.domain.com:51822" {
		t.Errorf("expected host proxy2.domain.com:51822, got %s", u2.Host)
	}
	if !strings.Contains(u2.Fragment, "US-Relay") {
		t.Errorf("expected fragment to contain US-Relay, got %s", u2.Fragment)
	}

	// Link 3: fallback to default.server.com:51820
	u3, err := url.Parse(links[2])
	if err != nil {
		t.Fatalf("failed to parse link 3: %v", err)
	}
	if u3.Host != "default.server.com:51820" {
		t.Errorf("expected host default.server.com:51820, got %s", u3.Host)
	}
}

func TestGenAmneziaWGLinkWithExternalProxy(t *testing.T) {
	sub := NewSubService(false, "-ieo")
	sub.address = "default.server.com"

	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	_, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate server keypair: %v", err)
	}

	settingsMap := map[string]interface{}{
		"server": map[string]interface{}{
			"publicKey": serverPub,
			"mtu":       1420,
			"jc":        4,
			"jmin":      40,
			"jmax":      70,
		},
		"clients": []map[string]interface{}{
			{
				"email":      "awguser@test.com",
				"privateKey": priv,
				"publicKey":  pub,
				"allowedIPs": []string{"10.8.1.2/32"},
			},
		},
	}
	settingsBytes, _ := json.Marshal(settingsMap)

	streamMap := map[string]interface{}{
		"network": "tcp",
		"externalProxy": []map[string]interface{}{
			{
				"dest":   "awgproxy.domain.com",
				"port":   44300,
				"remark": "AWG-Relay",
			},
		},
	}
	streamBytes, _ := json.Marshal(streamMap)

	inbound := &model.Inbound{
		Id:             2,
		Port:           4430,
		Protocol:       model.AmneziaWG,
		Settings:       string(settingsBytes),
		StreamSettings: string(streamBytes),
		Remark:         "AWG-Inbound",
	}

	linkStr := sub.genAmneziaWGLink(inbound, "awguser@test.com")
	if linkStr == "" {
		t.Fatalf("expected non-empty amneziawg link, got empty")
	}

	if !strings.HasPrefix(linkStr, "vpn://") {
		t.Fatalf("expected amneziawg link to start with vpn://, got %s", linkStr)
	}

	rawPart := strings.TrimPrefix(linkStr, "vpn://")
	parts := strings.Split(rawPart, "#")
	decodedBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		// try StdEncoding as fallback
		decodedBytes, err = base64.URLEncoding.DecodeString(parts[0])
		if err != nil {
			t.Fatalf("failed to decode vpn:// base64 payload: %v", err)
		}
	}

	confText := string(decodedBytes)
	expectedEndpoint := "Endpoint = awgproxy.domain.com:44300"
	if !strings.Contains(confText, expectedEndpoint) {
		t.Errorf("amneziawg client conf missing expected endpoint %q, conf was:\n%s", expectedEndpoint, confText)
	}
}

func TestAmneziaWG31ConfAndLinkFeatures(t *testing.T) {
	sub := NewSubService(false, "-ieo")
	sub.address = "vpn.example.com"

	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	_, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate server keypair: %v", err)
	}

	settingsMap := map[string]interface{}{
		"server": map[string]interface{}{
			"publicKey":              serverPub,
			"mtu":                    1360,
			"primaryDns":             "1.1.1.1",
			"secondaryDns":           "8.8.8.8",
			"jc":                     5,
			"jmin":                   40,
			"jmax":                   120,
			"s1":                     32,
			"s2":                     64,
			"s3":                     24,
			"s4":                     18,
			"h1":                     "1000",
			"h2":                     "2000",
			"h3":                     "3000",
			"h4":                     "4000",
			"i1":                     "<r 50>",
			"i2":                     "<b 0x01020304>",
			"i3":                     "<t>",
			"i4":                     "<rc 8>",
			"i5":                     "<rd 4>",
			"headerProtectionKey":    "QU1ORVpJQVdHMzFQUk9URUNUSU9OS0VZWkFCQ0RFRg==",
			"contentPaddingAddition": "8-48",
			"rekeyAfterTime":         "105-125",
			"rekeyTimeout":           "4-8",
			"rejectAfterTime":        "160-200",
			"keepaliveTimeout":       "10-15",
			"maxHandshakeAttempts":   "20-30",
			"randomTrailers":         true,
			"disableCookies":         true,
		},
		"clients": []map[string]interface{}{
			{
				"email":      "awg31@test.com",
				"privateKey": priv,
				"publicKey":  pub,
				"allowedIPs": []string{"10.8.1.2/32", "fd00::2/128"},
			},
		},
	}
	settingsBytes, _ := json.Marshal(settingsMap)

	inbound := &model.Inbound{
		Id:       99,
		Port:     51820,
		Protocol: model.AmneziaWG,
		Settings: string(settingsBytes),
		Remark:   "AWG-31-Full",
	}

	linkStr := sub.genAmneziaWGLink(inbound, "awg31@test.com")
	if linkStr == "" {
		t.Fatalf("expected non-empty amneziawg link, got empty")
	}
	if !strings.HasPrefix(linkStr, "vpn://") {
		t.Fatalf("expected link starting with vpn://, got %s", linkStr)
	}

	rawPart := strings.TrimPrefix(linkStr, "vpn://")
	parts := strings.Split(rawPart, "#")
	decodedBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		decodedBytes, err = base64.URLEncoding.DecodeString(parts[0])
		if err != nil {
			t.Fatalf("failed to decode vpn:// base64 payload: %v", err)
		}
	}
	conf := string(decodedBytes)

	requiredSnippets := []string{
		"Address = 10.8.1.2/32, fd00::2/128",
		"DNS = 1.1.1.1, 8.8.8.8",
		"MTU = 1360",
		"Jc = 5",
		"Jmin = 40",
		"Jmax = 120",
		"S1 = 32",
		"S2 = 64",
		"S3 = 24",
		"S4 = 18",
		"H1 = 1000",
		"H2 = 2000",
		"H3 = 3000",
		"H4 = 4000",
		"I1 = <r 50>",
		"I2 = <b 0x01020304>",
		"I3 = <t>",
		"I4 = <rc 8>",
		"I5 = <rd 4>",
		"HeaderProtectionKey = QU1ORVpJQVdHMzFQUk9URUNUSU9OS0VZWkFCQ0RFRg==",
		"ContentPaddingAddition = 8-48",
		"RekeyAfterTime = 105-125",
		"RekeyTimeout = 4-8",
		"RejectAfterTime = 160-200",
		"KeepaliveTimeout = 10-15",
		"MaxHandshakeAttempts = 20-30",
		"RandomTrailers = on",
		"DisableCookies = on",
		"Endpoint = vpn.example.com:51820",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(conf, snippet) {
			t.Errorf("rendered AWG 3.1 client conf missing expected snippet: %q\nFull conf:\n%s", snippet, conf)
		}
	}
}

func TestSubJsonWireguardWithExternalProxy(t *testing.T) {
	sub := NewSubService(false, "-ieo")
	sub.address = "default.server.com"
	subJson := NewSubJsonService("", "", "", "", sub)

	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("failed to generate server keypair: %v", err)
	}

	settingsMap := map[string]interface{}{
		"secretKey": serverPriv,
		"pubKey":    serverPub,
		"mtu":       1420,
		"clients": []map[string]interface{}{
			{
				"email":      "client1@test.com",
				"privateKey": priv,
				"publicKey":  pub,
				"allowedIPs": []string{"10.0.0.2/32"},
			},
		},
	}
	settingsBytes, _ := json.Marshal(settingsMap)

	streamMap := map[string]interface{}{
		"network": "tcp",
		"externalProxy": []map[string]interface{}{
			{
				"dest":   "wgproxy.domain.com",
				"port":   51825,
				"remark": "WG-EP",
			},
		},
	}
	streamBytes, _ := json.Marshal(streamMap)

	inbound := &model.Inbound{
		Id:             3,
		Port:           51820,
		Protocol:       model.WireGuard,
		Settings:       string(settingsBytes),
		StreamSettings: string(streamBytes),
		Remark:         "WG-Inbound",
	}

	client := model.Client{
		Email:      "client1@test.com",
		PrivateKey: priv,
		PublicKey:  pub,
		AllowedIPs: []string{"10.0.0.2/32"},
	}

	rawConfigs := subJson.getConfig(inbound, client, "default.server.com")
	if len(rawConfigs) != 1 {
		t.Fatalf("expected 1 config for 1 externalProxy item, got %d", len(rawConfigs))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(rawConfigs[0]), &parsed); err != nil {
		t.Fatalf("failed to parse config JSON: %v", err)
	}

	outbounds, ok := parsed["outbounds"].([]interface{})
	if !ok || len(outbounds) == 0 {
		t.Fatalf("expected non-empty outbounds in config JSON")
	}

	foundWg := false
	for _, ob := range outbounds {
		obMap, ok := ob.(map[string]interface{})
		if !ok || obMap["protocol"] != "wireguard" {
			continue
		}
		foundWg = true
		settings, _ := obMap["settings"].(map[string]interface{})
		peers, _ := settings["peers"].([]interface{})
		if len(peers) == 0 {
			t.Fatalf("wireguard settings missing peers")
		}
		peer, _ := peers[0].(map[string]interface{})
		endpoint, _ := peer["endpoint"].(string)
		if endpoint != "wgproxy.domain.com:51825" {
			t.Errorf("expected wireguard outbound endpoint wgproxy.domain.com:51825, got %s", endpoint)
		}
	}

	if !foundWg {
		t.Errorf("did not find wireguard outbound in config JSON")
	}
}

func TestGenMtprotoLinkWithExternalProxy(t *testing.T) {
	sub := NewSubService(false, "-ieo")
	sub.address = "default.server.com"

	secret := model.GenerateFakeTLSSecret("google.com")

	settingsMap := map[string]interface{}{
		"clients": []map[string]interface{}{
			{
				"email":  "mtpuser@test.com",
				"secret": secret,
			},
		},
	}
	settingsBytes, _ := json.Marshal(settingsMap)

	streamMap := map[string]interface{}{
		"network": "tcp",
		"externalProxy": []map[string]interface{}{
			{
				"dest":   "mtpproxy1.domain.com",
				"port":   8443,
				"remark": "MTP-Relay1",
			},
			{
				"dest":   "mtpproxy2.domain.com",
				"port":   9443,
				"remark": "MTP-Relay2",
			},
		},
	}
	streamBytes, _ := json.Marshal(streamMap)

	inbound := &model.Inbound{
		Id:             4,
		Port:           443,
		Protocol:       model.MTProto,
		Settings:       string(settingsBytes),
		StreamSettings: string(streamBytes),
		Remark:         "MTProto-Inbound",
	}

	linkStr := sub.genMtprotoLink(inbound, "mtpuser@test.com")
	if linkStr == "" {
		t.Fatalf("expected non-empty mtproto link, got empty")
	}

	links := strings.Split(linkStr, "\n")
	if len(links) != 2 {
		t.Fatalf("expected 2 links for 2 externalProxy items, got %d", len(links))
	}

	expectedPrefix1 := "tg://proxy?server=mtpproxy1.domain.com&port=8443&secret=" + url.QueryEscape(secret)
	if !strings.HasPrefix(links[0], expectedPrefix1) {
		t.Errorf("expected link 1 to start with %q, got %q", expectedPrefix1, links[0])
	}
	if !strings.Contains(links[0], "MTP-Relay1") {
		t.Errorf("expected link 1 to contain remark MTP-Relay1, got %q", links[0])
	}

	expectedPrefix2 := "tg://proxy?server=mtpproxy2.domain.com&port=9443&secret=" + url.QueryEscape(secret)
	if !strings.HasPrefix(links[1], expectedPrefix2) {
		t.Errorf("expected link 2 to start with %q, got %q", expectedPrefix2, links[1])
	}
	if !strings.Contains(links[1], "MTP-Relay2") {
		t.Errorf("expected link 2 to contain remark MTP-Relay2, got %q", links[1])
	}
}
