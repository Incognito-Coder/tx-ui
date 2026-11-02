package xray

import (
	"strings"
	"testing"

	"github.com/xtls/xray-core/common/serial"
	wireguardProxy "github.com/xtls/xray-core/proxy/wireguard"
	wgutil "x-ui/internal/util/wireguard"
)

func TestWireguardAddUserAccountBuilding(t *testing.T) {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("GenerateWireguardKeypair failed: %v", err)
	}

	user := map[string]interface{}{
		"email":        "client@wireguard.test",
		"publicKey":    pub,
		"privateKey":   priv,
		"allowedIPs":   []string{"10.0.0.5"},
		"preSharedKey": "",
		"keepAlive":    "25",
	}

	pubHex, err := wgutil.KeyToHex(safeGetString(user, "publicKey"))
	if err != nil || pubHex == "" {
		t.Fatalf("KeyToHex failed: %v", err)
	}

	allowedIPs := safeGetStringSlice(user, "allowedIPs")
	for i, aip := range allowedIPs {
		aip = strings.TrimSpace(aip)
		if aip != "" && !strings.Contains(aip, "/") {
			if strings.Contains(aip, ":") {
				allowedIPs[i] = aip + "/128"
			} else {
				allowedIPs[i] = aip + "/32"
			}
		}
	}

	if len(allowedIPs) != 1 || allowedIPs[0] != "10.0.0.5/32" {
		t.Errorf("expected allowedIPs to be ['10.0.0.5/32'], got %v", allowedIPs)
	}

	keepAlive := safeGetString(user, "keepAlive")
	account := serial.ToTypedMessage(&wireguardProxy.PeerConfig{
		PublicKey:  pubHex,
		AllowedIps: allowedIPs,
		KeepAlive:  keepAlive,
	})

	instance, err := account.GetInstance()
	if err != nil {
		t.Fatalf("account.GetInstance() failed: %v", err)
	}

	peerCfg, ok := instance.(*wireguardProxy.PeerConfig)
	if !ok {
		t.Fatalf("expected *wireguardProxy.PeerConfig, got %T", instance)
	}

	memAccount, err := peerCfg.AsAccount()
	if err != nil {
		t.Fatalf("peerCfg.AsAccount() failed: %v", err)
	}

	if memAccount == nil {
		t.Fatal("expected memAccount not to be nil")
	}
}
