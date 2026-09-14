package sub

import (
	"crypto/ecdh"
	"encoding/base64"
	"net/url"
	"testing"
	"x-ui/internal/database/model"
)

func TestGetWireguardPubKey(t *testing.T) {
	// Generate a valid X25519 private key
	priv, err := ecdh.X25519().GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	privB64 := base64.StdEncoding.EncodeToString(priv.Bytes())
	expectedPubB64 := base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())

	pubB64 := getWireguardPubKey(privB64)
	if pubB64 != expectedPubB64 {
		t.Errorf("Expected pubkey %s, got %s", expectedPubB64, pubB64)
	}
}

func TestGenWireguardLink(t *testing.T) {
	priv, _ := ecdh.X25519().GenerateKey(nil)
	privB64 := base64.StdEncoding.EncodeToString(priv.Bytes())
	pubB64 := base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())

	clientPriv, _ := ecdh.X25519().GenerateKey(nil)
	clientPrivB64 := base64.StdEncoding.EncodeToString(clientPriv.Bytes())

	subSvc := NewSubService(false, "-ieo")
	subSvc.address = "127.0.0.1"

	inbound := &model.Inbound{
		Protocol: model.WireGuard,
		Port:     51820,
		Settings: `{"secretKey":"` + privB64 + `","pubKey":"` + pubB64 + `","mtu":1420,"peers":[{"email":"user1@test.com","privateKey":"` + clientPrivB64 + `","allowedIPs":["10.0.0.2/32","fd00::2/128"],"psk":"testpsk123=","keepAlive":25}]}`,
	}

	link := subSvc.genWireguardLink(inbound, "user1@test.com")
	if link == "" {
		t.Fatalf("Expected non-empty link, got empty")
	}

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("Failed to parse generated link %s: %v", link, err)
	}

	if u.Scheme != "wireguard" {
		t.Errorf("Expected scheme wireguard, got %s", u.Scheme)
	}

	q := u.Query()
	if q.Get("peer_public_key") != pubB64 {
		t.Errorf("Peer public key mismatch. peer_public_key=%s", q.Get("peer_public_key"))
	}
	if q.Get("public_key") != pubB64 {
		t.Errorf("Public key mismatch. public_key=%s", q.Get("public_key"))
	}
	if q.Get("private_key") != clientPrivB64 {
		t.Errorf("Private key mismatch. private_key=%s", q.Get("private_key"))
	}
	if q.Get("address") != "10.0.0.2/32,fd00::2/128" {
		t.Errorf("Address mismatch: address=%s", q.Get("address"))
	}
	if q.Get("dns") != "1.1.1.1,1.0.0.1" {
		t.Errorf("Expected dns=1.1.1.1,1.0.0.1, got %s", q.Get("dns"))
	}
	if q.Get("mtu") != "1420" {
		t.Errorf("Expected mtu=1420, got %s", q.Get("mtu"))
	}
	if q.Get("pre_shared_key") != "testpsk123=" {
		t.Errorf("PSK mismatch: pre_shared_key=%s", q.Get("pre_shared_key"))
	}
	if q.Get("keepalive") != "25" {
		t.Errorf("Keepalive mismatch: keepalive=%s", q.Get("keepalive"))
	}
	if q.Get("allowed_ips") != "0.0.0.0/0,::/0" {
		t.Errorf("AllowedIPs mismatch: allowed_ips=%s", q.Get("allowed_ips"))
	}
	if q.Get("reserved") != "0,0,0" {
		t.Errorf("Reserved mismatch: reserved=%s", q.Get("reserved"))
	}
	// Verify no redundant duplicate parameters
	for _, dup := range []string{"publicKey", "publickey", "privateKey", "privatekey", "ip", "psk", "presharedkey", "preSharedKey", "allowedips", "allowedIPs"} {
		if q.Has(dup) {
			t.Errorf("Expected no duplicate parameter %s in URL", dup)
		}
	}
}
