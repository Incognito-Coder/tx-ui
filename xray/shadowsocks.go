package xray

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"

	"x-ui/internal/util/common"

	"github.com/xtls/xray-core/proxy/shadowsocks_2022"
)

// IsShadowsocks2022 returns true if method belongs to Shadowsocks 2022 standard.
func IsShadowsocks2022(method string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(method)), "2022")
}

// IsShadowsocks2022SingleUserOnly returns true for 2022-blake3-chacha20-poly1305,
// which in the Shadowsocks 2022 spec does not support multi-user header decryption.
func IsShadowsocks2022SingleUserOnly(method string) bool {
	return strings.EqualFold(strings.TrimSpace(method), "2022-blake3-chacha20-poly1305")
}

// Shadowsocks2022KeyLength is the key size a Shadowsocks 2022 method needs, or
// 0 for any other method, which takes a free-form password.
func Shadowsocks2022KeyLength(method string) int {
	cipher, err := shadowsocks_2022.GetCipherMethod(method)
	if err != nil {
		return 0
	}
	return cipher.KeySaltLength
}

// ValidShadowsocks2022Key reads a key the way the core does: base64, or else
// the raw string, and either way exactly length bytes.
func ValidShadowsocks2022Key(key string, length int) bool {
	_, err := shadowsocks_2022.ParseKey(key, length)
	return err == nil
}

func NewShadowsocks2022Key(length int) string {
	key := make([]byte, length)
	_, _ = rand.Read(key)
	return base64.StdEncoding.EncodeToString(key)
}

// CheckShadowsocks2022Keys rejects inbound settings holding a Shadowsocks 2022
// key of the wrong size or invalid multi-user configuration.
func CheckShadowsocks2022Keys(protocol string, settings string) error {
	if protocol != "shadowsocks" {
		return nil
	}
	var parsed struct {
		Method   string `json:"method"`
		Password string `json:"password"`
		Clients  []struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		} `json:"clients"`
	}
	if json.Unmarshal([]byte(settings), &parsed) != nil {
		return nil
	}
	if IsShadowsocks2022SingleUserOnly(parsed.Method) && len(parsed.Clients) > 0 {
		return common.NewErrorf("%s only supports single-user mode in Shadowsocks 2022. Please remove clients or use 2022-blake3-aes-128-gcm / 2022-blake3-aes-256-gcm for multi-user", parsed.Method)
	}
	length := Shadowsocks2022KeyLength(parsed.Method)
	if length == 0 {
		return nil
	}
	if !ValidShadowsocks2022Key(parsed.Password, length) {
		return common.NewErrorf("%s needs a base64 key of %d bytes, the inbound password is not one", parsed.Method, length)
	}
	for _, client := range parsed.Clients {
		if !ValidShadowsocks2022Key(client.Password, length) {
			return common.NewErrorf("%s needs a base64 key of %d bytes, the password of %s is not one", parsed.Method, length, client.Email)
		}
	}
	return nil
}
