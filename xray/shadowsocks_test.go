package xray

import (
	"encoding/base64"
	"testing"
)

func TestShadowsocks2022KeyLength(t *testing.T) {
	if got := Shadowsocks2022KeyLength("2022-blake3-aes-128-gcm"); got != 16 {
		t.Fatalf("128-gcm length = %d, want 16", got)
	}
	if got := Shadowsocks2022KeyLength("2022-blake3-aes-256-gcm"); got != 32 {
		t.Fatalf("256-gcm length = %d, want 32", got)
	}
	if got := Shadowsocks2022KeyLength("2022-blake3-chacha20-poly1305"); got != 32 {
		t.Fatalf("chacha20 length = %d, want 32", got)
	}
	if got := Shadowsocks2022KeyLength("aes-128-gcm"); got != 0 {
		t.Fatalf("legacy aes-128-gcm length = %d, want 0", got)
	}
}

func TestCheckShadowsocks2022Keys(t *testing.T) {
	key16 := base64.StdEncoding.EncodeToString(make([]byte, 16))
	key32 := base64.StdEncoding.EncodeToString(make([]byte, 32))

	validSettings := `{"method":"2022-blake3-aes-128-gcm","password":"` + key16 + `","clients":[{"email":"a","password":"` + key16 + `"}]}`
	if err := CheckShadowsocks2022Keys("shadowsocks", validSettings); err != nil {
		t.Fatalf("unexpected error for valid keys: %v", err)
	}

	invalidSettings := `{"method":"2022-blake3-aes-128-gcm","password":"` + key32 + `"}`
	if err := CheckShadowsocks2022Keys("shadowsocks", invalidSettings); err == nil {
		t.Fatal("expected error for 32-byte key with 128-gcm")
	}
}
