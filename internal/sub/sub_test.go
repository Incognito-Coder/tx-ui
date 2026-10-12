package sub

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"x-ui/internal/database/model"

	"github.com/goccy/go-json"
)

func TestSubServiceECHLinks(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	subService.address = "server.example.com"

	echConfig := "AF7+DQBaAAAgACBxcShGBgh+STt1uGtX0BeCVKwYwFD59mU9lE8eUZBfMwAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAAtleGFtcGxlLmNvbQAA"
	streamSettings := `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"example.com","settings":{"fingerprint":"chrome","echConfigList":"` + echConfig + `"}}}`

	// Test VLESS
	vlessInbound := &model.Inbound{
		Id:             1,
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"id":"b831381d-6324-4d53-ad4f-8cda48b30811","email":"user1@example.com"}]}`,
		StreamSettings: streamSettings,
	}
	vlessLink := subService.genVlessLink(vlessInbound, "user1@example.com")
	u, err := url.Parse(vlessLink)
	if err != nil {
		t.Fatalf("failed to parse vless link: %v", err)
	}
	if !strings.Contains(u.RawQuery, "ech="+echConfig) {
		t.Fatalf("expected ech param '%s' in vless link, got '%s'", echConfig, u.RawQuery)
	}

	// Test VMess
	vmessInbound := &model.Inbound{
		Id:             2,
		Port:           443,
		Protocol:       model.VMESS,
		Settings:       `{"clients":[{"id":"b831381d-6324-4d53-ad4f-8cda48b30811","email":"user1@example.com","security":"auto"}]}`,
		StreamSettings: streamSettings,
	}
	vmessLink := subService.genVmessLink(vmessInbound, "user1@example.com")
	if !strings.HasPrefix(vmessLink, "vmess://") {
		t.Fatalf("expected vmess link to start with vmess://, got %s", vmessLink)
	}
	vmessB64 := strings.TrimPrefix(vmessLink, "vmess://")
	vmessJSON, err := base64.StdEncoding.DecodeString(vmessB64)
	if err != nil {
		t.Fatalf("failed to decode vmess base64: %v", err)
	}
	var vmessObj map[string]interface{}
	if err := json.Unmarshal(vmessJSON, &vmessObj); err != nil {
		t.Fatalf("failed to unmarshal vmess JSON: %v", err)
	}
	if vmessObj["ech"] != echConfig {
		t.Fatalf("expected ech '%s' in vmess object, got '%v'", echConfig, vmessObj["ech"])
	}

	// Test Trojan
	trojanInbound := &model.Inbound{
		Id:             3,
		Port:           443,
		Protocol:       model.Trojan,
		Settings:       `{"clients":[{"password":"trojan-pass","email":"user1@example.com"}]}`,
		StreamSettings: streamSettings,
	}
	trojanLink := subService.genTrojanLink(trojanInbound, "user1@example.com")
	uTrojan, err := url.Parse(trojanLink)
	if err != nil {
		t.Fatalf("failed to parse trojan link: %v", err)
	}
	if !strings.Contains(uTrojan.RawQuery, "ech="+echConfig) {
		t.Fatalf("expected ech param '%s' in trojan link, got '%s'", echConfig, uTrojan.RawQuery)
	}

	// Test Shadowsocks
	ssInbound := &model.Inbound{
		Id:             4,
		Port:           443,
		Protocol:       model.Shadowsocks,
		Settings:       `{"method":"aes-256-gcm","password":"ss-server-pass","clients":[{"password":"ss-client-pass","email":"user1@example.com"}]}`,
		StreamSettings: streamSettings,
	}
	ssLink := subService.genShadowsocksLink(ssInbound, "user1@example.com")
	uSS, err := url.Parse(ssLink)
	if err != nil {
		t.Fatalf("failed to parse shadowsocks link: %v", err)
	}
	if !strings.Contains(uSS.RawQuery, "ech="+echConfig) {
		t.Fatalf("expected ech param '%s' in shadowsocks link, got '%s'", echConfig, uSS.RawQuery)
	}

	// Test Hysteria
	hyInbound := &model.Inbound{
		Id:             5,
		Port:           443,
		Protocol:       model.Hysteria,
		Settings:       `{"version":2,"clients":[{"auth":"hy-auth","email":"user1@example.com"}]}`,
		StreamSettings: streamSettings,
	}
	hyLink := subService.genHysteriaLink(hyInbound, "user1@example.com")
	uHy, err := url.Parse(hyLink)
	if err != nil {
		t.Fatalf("failed to parse hysteria link: %v", err)
	}
	if !strings.Contains(uHy.RawQuery, "ech="+echConfig) {
		t.Fatalf("expected ech param '%s' in hysteria link, got '%s'", echConfig, uHy.RawQuery)
	}
}

func TestSubJsonServiceECH(t *testing.T) {
	subJsonService := NewSubJsonService("", "", "", "", nil)
	echConfig := "AF7+DQBaAAAgACBxcShGBgh+STt1uGtX0BeCVKwYwFD59mU9lE8eUZBfMwAkAAEAAQABAAIAAQADAAIAAQACAAIAAgADAAMAAQADAAIAAwADAAtleGFtcGxlLmNvbQAA"

	tData := map[string]interface{}{
		"serverName": "example.com",
		"alpn":       []string{"h2", "http/1.1"},
		"settings": map[string]interface{}{
			"fingerprint":   "chrome",
			"echConfigList": echConfig,
		},
	}

	result := subJsonService.tlsData(tData)
	if result["echConfigList"] != echConfig {
		t.Fatalf("expected echConfigList '%s' in tlsData result, got '%v'", echConfig, result["echConfigList"])
	}

	tDataWithForce := map[string]interface{}{
		"serverName":    "example.com",
		"echForceQuery": "full",
		"settings": map[string]interface{}{
			"echConfigList": "cloudflare-ech.com+udp://1.1.1.1",
		},
	}
	resultForce := subJsonService.tlsData(tDataWithForce)
	if resultForce["echForceQuery"] != "full" {
		t.Fatalf("expected echForceQuery 'full', got '%v'", resultForce["echForceQuery"])
	}
	if resultForce["echConfigList"] != "cloudflare-ech.com+udp://1.1.1.1" {
		t.Fatalf("expected echConfigList 'cloudflare-ech.com+udp://1.1.1.1', got '%v'", resultForce["echConfigList"])
	}
}

func TestSubServiceVlessTrueConfigECH(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	subService.address = "cookiebot.com"

	echConfig := "cloudflare-ech.com+udp://1.1.1.1"
	encryption := "mlkem768x25519plus.native.0rtt.testkey"
	streamSettings := `{"network":"xhttp","xhttpSettings":{"path":"/","host":"host.ahmand.online","mode":"auto"},"security":"tls","tlsSettings":{"serverName":"host.ahmand.online","alpn":["h2","http/1.1","h3"],"settings":{"fingerprint":"chrome","echConfigList":"` + echConfig + `"}}}`

	vlessInbound := &model.Inbound{
		Id:             1,
		Port:           2083,
		Protocol:       model.VLESS,
		Settings:       `{"decryption":"` + encryption + `","clients":[{"id":"38e6216f-0bf1-417c-bd57-3ac0678e94a0","email":"user1@example.com"}]}`,
		StreamSettings: streamSettings,
	}

	link := subService.genVlessLink(vlessInbound, "user1@example.com")
	if !strings.Contains(link, "ech=cloudflare-ech.com+udp://1.1.1.1") {
		t.Fatalf("expected link to contain unescaped 'ech=cloudflare-ech.com+udp://1.1.1.1', got %s", link)
	}
	if !strings.Contains(link, "alpn=h2,http/1.1,h3") {
		t.Fatalf("expected link to contain unescaped 'alpn=h2,http/1.1,h3', got %s", link)
	}
	if !strings.Contains(link, "path=/") {
		t.Fatalf("expected link to contain unescaped 'path=/', got %s", link)
	}
	if !strings.Contains(link, "encryption="+encryption) {
		t.Fatalf("expected link to contain encryption from decryption setting, got %s", link)
	}
}

func TestHysteriaSubService_UDPFinalMaskAndExternalProxy(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	subService.address = "panel.example.com"

	streamSettings := `{
		"network": "hysteria",
		"security": "tls",
		"tlsSettings": {
			"serverName": "hy.example.com",
			"alpn": ["h3"],
			"settings": {
				"allowInsecure": true
			}
		},
		"finalmask": {
			"udp": [
				{
					"type": "salamander",
					"settings": {
						"password": "salamander-secret"
					}
				},
				{
					"type": "udpHop",
					"settings": {
						"ports": "20000-40000"
					}
				}
			]
		},
		"externalProxy": [
			{
				"dest": "ep1.example.com",
				"port": 8443,
				"remark": "node1"
			},
			{
				"dest": "ep2.example.com",
				"port": 9443,
				"remark": "node2"
			}
		]
	}`

	hyInbound := &model.Inbound{
		Id:             10,
		Port:           443,
		Protocol:       model.Hysteria,
		Settings:       `{"version":2,"clients":[{"auth":"my-hy-auth","email":"test@example.com"}]}`,
		StreamSettings: streamSettings,
	}

	links := subService.genHysteriaLink(hyInbound, "test@example.com")
	lines := strings.Split(links, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 links for 2 external proxies, got %d: %s", len(lines), links)
	}

	for _, l := range lines {
		u, err := url.Parse(l)
		if err != nil {
			t.Fatalf("failed to parse hysteria link: %v", err)
		}
		if u.Scheme != "hysteria2" {
			t.Fatalf("expected scheme hysteria2, got %s", u.Scheme)
		}
		if u.User.Username() != "my-hy-auth" {
			t.Fatalf("expected user my-hy-auth, got %s", u.User.Username())
		}
		q := u.Query()
		if q.Get("obfs") != "salamander" {
			t.Fatalf("expected obfs=salamander, got %s", q.Get("obfs"))
		}
		if q.Get("obfs-password") != "salamander-secret" {
			t.Fatalf("expected obfs-password=salamander-secret, got %s", q.Get("obfs-password"))
		}
		if q.Get("mport") != "20000-40000" {
			t.Fatalf("expected mport=20000-40000, got %s", q.Get("mport"))
		}
		if q.Get("insecure") != "1" {
			t.Fatalf("expected insecure=1, got %s", q.Get("insecure"))
		}
		if q.Get("sni") != "hy.example.com" {
			t.Fatalf("expected sni=hy.example.com, got %s", q.Get("sni"))
		}
	}
}

func TestHysteriaSubJsonService_Finalmask(t *testing.T) {
	subJsonService := NewSubJsonService("", "", "", "", nil)

	hyInbound := &model.Inbound{
		Id:       11,
		Listen:   "0.0.0.0",
		Port:     443,
		Protocol: model.Hysteria,
		Settings: `{"version":2}`,
		StreamSettings: `{
			"network": "hysteria",
			"security": "tls",
			"hysteriaSettings": {
				"auth": "server-fallback-auth",
				"udpIdleTimeout": 60
			},
			"finalmask": {
				"udp": [
					{
						"type": "salamander",
						"settings": {"password": "pwd"}
					}
				]
			}
		}`,
	}

	client := model.Client{
		Email: "jsonuser@example.com",
		// auth left empty to test fallback
	}

	cfgs := subJsonService.getConfig(hyInbound, client, "hy.domain.com")
	if len(cfgs) == 0 {
		t.Fatalf("expected at least 1 json config, got 0")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(cfgs[0]), &parsed); err != nil {
		t.Fatalf("failed to parse output config: %v", err)
	}

	outbounds, ok := parsed["outbounds"].([]interface{})
	if !ok || len(outbounds) == 0 {
		t.Fatalf("expected outbounds array in config")
	}

	proxyOutbound := outbounds[0].(map[string]interface{})
	streamSettings := proxyOutbound["streamSettings"].(map[string]interface{})

	// Check finalmask was retained in streamSettings
	finalmask, ok := streamSettings["finalmask"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected finalmask in streamSettings, got: %v", streamSettings)
	}
	if finalmask["udp"] == nil {
		t.Fatalf("expected udp masks in finalmask")
	}

	// Check fallback auth
	hySettings := streamSettings["hysteriaSettings"].(map[string]interface{})
	if hySettings["auth"] != "server-fallback-auth" {
		t.Fatalf("expected auth fallback to server-fallback-auth, got %v", hySettings["auth"])
	}
}


