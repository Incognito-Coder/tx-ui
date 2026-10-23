package model

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"x-ui/internal/util/json_util"
	"x-ui/xray"
)

type Protocol string

const (
	VMESS       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Tunnel      Protocol = "tunnel"
	HTTP        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
	Mixed       Protocol = "mixed"
	WireGuard   Protocol = "wireguard"
	Hysteria    Protocol = "hysteria"
	Masque      Protocol = "masque"
	MTProto     Protocol = "mtproto"
	AmneziaWG   Protocol = "amneziawg"
)

type User struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	LoginSecret string `json:"loginSecret"`
}

type Inbound struct {
	Id          int                  `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	UserId      int                  `json:"-"`
	Up          int64                `json:"up" form:"up"`
	Down        int64                `json:"down" form:"down"`
	Total       int64                `json:"total" form:"total"`
	Remark      string               `json:"remark" form:"remark"`
	Enable      bool                 `json:"enable" form:"enable"`
	ExpiryTime  int64                `json:"expiryTime" form:"expiryTime"`
	ClientStats []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats" form:"clientStats"`

	// config part
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`

	// Sort order for UI listing. Lower numbers appear first.
	Sort int `json:"sort" form:"sort" gorm:"column:sort;default:0"`
}

type OutboundTraffics struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Tag   string `json:"tag" form:"tag" gorm:"unique"`
	Up    int64  `json:"up" form:"up" gorm:"default:0"`
	Down  int64  `json:"down" form:"down" gorm:"default:0"`
	Total int64  `json:"total" form:"total" gorm:"default:0"`
}

type InboundClientIps struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientEmail string `json:"clientEmail" form:"clientEmail" gorm:"unique"`
	Ips         string `json:"ips" form:"ips"`
}

type HistoryOfSeeders struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SeederName string `json:"seederName"`
}

func (i *Inbound) GenXrayInboundConfig() *xray.InboundConfig {
	listen := i.Listen
	if listen == "" {
		listen = "0.0.0.0"
	}
	listen = fmt.Sprintf("\"%v\"", listen)
	settings := i.Settings
	if i.Protocol == WireGuard {
		settings = wireguardClientsAsPeers(settings)
	} else if i.Protocol == Masque {
		settings = masqueSettingsNormalize(settings)
	}
	streamSettings := i.StreamSettings
	if streamSettings != "" {
		var stream map[string]any
		if err := json.Unmarshal([]byte(streamSettings), &stream); err == nil && stream != nil {
			if xray.TakeUDPHop(stream) != nil {
				if b, err := json.Marshal(stream); err == nil {
					streamSettings = string(b)
				}
			}
		}
	}
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       string(i.Protocol),
		Settings:       json_util.RawMessage(settings),
		StreamSettings: json_util.RawMessage(streamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

func wireguardClientsAsPeers(settings string) string {
	var raw map[string]any
	if err := json.Unmarshal([]byte(settings), &raw); err != nil {
		return settings
	}
	clients, ok := raw["clients"]
	if !ok {
		return settings
	}
	if clientList, isList := clients.([]any); isList {
		var peers []any
		for _, clientItem := range clientList {
			c, isMap := clientItem.(map[string]any)
			if !isMap {
				continue
			}
			peer := map[string]any{}
			for k, v := range c {
				peer[k] = v
			}
			if psk, ok := c["psk"].(string); ok && psk != "" {
				peer["preSharedKey"] = psk
			}
			if pubKey, ok := c["publicKey"].(string); ok && pubKey != "" {
				peer["publicKey"] = pubKey
			} else if id, ok := c["id"].(string); ok && id != "" {
				peer["publicKey"] = id
			}
			if v, ok := c["allowedIPs"]; ok && v != nil {
				switch a := v.(type) {
				case []string:
					peer["allowedIPs"] = a
				case []any:
					arr := make([]string, 0, len(a))
					for _, item := range a {
						if s, ok := item.(string); ok && s != "" {
							arr = append(arr, s)
						}
					}
					if len(arr) > 0 {
						peer["allowedIPs"] = arr
					} else {
						peer["allowedIPs"] = []string{"10.0.0.2/32"}
					}
				case string:
					if a != "" {
						peer["allowedIPs"] = []string{a}
					} else {
						peer["allowedIPs"] = []string{"10.0.0.2/32"}
					}
				default:
					peer["allowedIPs"] = []string{"10.0.0.2/32"}
				}
			} else {
				peer["allowedIPs"] = []string{"10.0.0.2/32"}
			}
			peers = append(peers, peer)
		}
		raw["peers"] = peers
	} else {
		raw["peers"] = clients
	}
	delete(raw, "clients")
	converted, err := json.Marshal(raw)
	if err != nil {
		return settings
	}
	return string(converted)
}

func masqueSettingsNormalize(settings string) string {
	var raw map[string]any
	if err := json.Unmarshal([]byte(settings), &raw); err != nil {
		return settings
	}
	if addr, ok := raw["address"]; ok && addr != nil {
		switch a := addr.(type) {
		case []string:
			raw["address"] = a
		case []any:
			arr := make([]string, 0, len(a))
			for _, item := range a {
				if s, ok := item.(string); ok && s != "" {
					arr = append(arr, s)
				}
			}
			if len(arr) > 0 {
				raw["address"] = arr
			} else {
				raw["address"] = []string{"10.13.0.1/24"}
			}
		case string:
			if a != "" {
				raw["address"] = []string{a}
			} else {
				raw["address"] = []string{"10.13.0.1/24"}
			}
		default:
			raw["address"] = []string{"10.13.0.1/24"}
		}
	} else {
		raw["address"] = []string{"10.13.0.1/24"}
	}

	clients, ok := raw["clients"]
	if !ok {
		clients = raw["users"]
	}
	if clientList, isList := clients.([]any); isList {
		var finalUsers []any
		for _, clientItem := range clientList {
			c, isMap := clientItem.(map[string]any)
			if !isMap {
				continue
			}
			if enable, ok := c["enable"].(bool); ok && !enable {
				continue
			}
			user := map[string]any{}
			if email, ok := c["email"].(string); ok && email != "" {
				user["email"] = email
			}
			pass := ""
			if p, ok := c["pass"].(string); ok && p != "" {
				pass = p
			} else if p, ok := c["password"].(string); ok && p != "" {
				pass = p
			} else if p, ok := c["id"].(string); ok && p != "" {
				pass = p
			}
			user["pass"] = pass
			if level, ok := c["level"].(float64); ok {
				user["level"] = uint32(level)
			}
			finalUsers = append(finalUsers, user)
		}
		raw["clients"] = finalUsers
		delete(raw, "users")
	}

	if mtu, ok := raw["mtu"].(float64); !ok || mtu == 0 {
		raw["mtu"] = 1500
	}

	result, err := json.Marshal(raw)
	if err != nil {
		return settings
	}
	return string(result)
}

type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

type ClientReverse struct {
	Tag      string `json:"tag"`
	Sniffing string `json:"sniffing" omitEmpty:"true"`
}

type Client struct {
	ID         string         `json:"id,omitempty"`
	Security   string         `json:"security"`
	Password   string         `json:"password,omitempty"` // Client password
	SSPassword string         `json:"ssPassword,omitempty"` // Shadowsocks password / key
	MasquePass string         `json:"masquePass,omitempty"` // MASQUE password
	Pass       string         `json:"pass,omitempty"`     // MASQUE password
	Flow       string         `json:"flow,omitempty"`     // Flow control (XTLS)
	Auth       string         `json:"auth,omitempty"`     // Auth password (Hysteria)
	PublicKey  string         `json:"publicKey,omitempty"`  // WireGuard Public Key
	PrivateKey string         `json:"privateKey,omitempty"` // WireGuard Private Key
	AllowedIPs []string       `json:"allowedIPs,omitempty"` // WireGuard Allowed IPs
	Psk            string         `json:"psk,omitempty"`        // WireGuard Preshared Key
	PreSharedKey   string         `json:"preSharedKey,omitempty"` // WireGuard / AmneziaWG Preshared Key
	ForwardedPorts string         `json:"forwardedPorts,omitempty"` // AmneziaWG forwarded ports
	Secret         string         `json:"secret,omitempty"`     // MTProto FakeTLS secret
	AdTag          string         `json:"adTag,omitempty"`      // MTProto AdTag
	KeepAlive      int            `json:"keepAlive,omitempty"`  // WireGuard KeepAlive
	Reverse        *ClientReverse `json:"reverse" omitEmpty:"true"`
	Email          string         `json:"email"`
	LimitIP        int            `json:"limitIp"`
	TotalGB        int64          `json:"totalGB" form:"totalGB"`
	ExpiryTime     int64          `json:"expiryTime" form:"expiryTime"`
	Enable         bool           `json:"enable" form:"enable"`
	TgID           int64          `json:"tgId" form:"tgId"`
	SubID          string         `json:"subId" form:"subId"`
	Comment        string         `json:"comment" form:"comment"`
	Reset          int            `json:"reset" form:"reset"`
}

func (c *Client) KeepAliveSeconds() int {
	if c == nil {
		return 0
	}
	return c.KeepAlive
}

type VLESSSettings struct {
	Clients    []Client `json:"clients"`
	Decryption string   `json:"decryption"`
	Encryption string   `json:"encryption"`
	Fallbacks  []any    `json:"fallbacks"`
}

// NodeClient is the shared, protocol-agnostic client identity.
type NodeClient struct {
	Id             int    `json:"id"             form:"id"             gorm:"primaryKey;autoIncrement"`
	Email          string `json:"email"          form:"email"          gorm:"uniqueIndex"`
	SubID          string `json:"subId"          form:"subId"          gorm:"uniqueIndex"`
	UUID           string `json:"uuid"           form:"uuid"           gorm:"column:uuid"` // for vmess/vless
	Password       string `json:"password"       form:"password"`                          // for trojan
	SSPassword     string `json:"ssPassword"     form:"ssPassword"`                        // for shadowsocks
	MasquePass     string `json:"masquePass"     form:"masquePass"`                        // for masque
	Auth           string `json:"auth"           form:"auth"`                              // for hysteria
	Secret         string `json:"secret"         form:"secret"`                            // for mtproto
	AdTag          string `json:"adTag"          form:"adTag"`                             // for mtproto
	PreSharedKey   string `json:"preSharedKey"   form:"preSharedKey"`                      // for wireguard/amneziawg
	PublicKey      string `json:"publicKey"      form:"publicKey"`                         // for wireguard/amneziawg
	PrivateKey     string `json:"privateKey"     form:"privateKey"`                        // for wireguard/amneziawg
	ForwardedPorts string `json:"forwardedPorts" form:"forwardedPorts"`                   // for amneziawg
	Security       string `json:"security"       form:"security"`
	Flow       string `json:"flow"       form:"flow"` // global flow default; per-link override wins
	TotalGB    int64  `json:"totalGB"    form:"totalGB"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	LimitIP    int    `json:"limitIp"    form:"limitIp"`
	TgID       int64  `json:"tgId"       form:"tgId"`
	Enable     bool   `json:"enable"     form:"enable"     gorm:"default:true"`
	Reset      int    `json:"reset"      form:"reset"` // auto-reset interval in days (0 = off)
	Comment    string `json:"comment"    form:"comment"`

	// Device & Subscription metadata captured from subscription requests
	DeviceModel   string `json:"deviceModel"   form:"deviceModel"`
	DeviceHWID    string `json:"deviceHwid"    form:"deviceHwid"`
	LastSubTime   int64  `json:"lastSubTime"   form:"lastSubTime"`
	LastSubIP     string `json:"lastSubIP"     form:"lastSubIP"`
	LastUserAgent string `json:"lastUserAgent" form:"lastUserAgent"`
	LastClientApp string `json:"lastClientApp" form:"lastClientApp"`
	LastOS        string `json:"lastOS"        form:"lastOS"`
}

// NodeClientLink joins a NodeClient to an Inbound, with optional per-link overrides.
type NodeClientLink struct {
	Id           int    `json:"id"           gorm:"primaryKey;autoIncrement"`
	NodeClientId int    `json:"nodeClientId" gorm:"uniqueIndex:idx_nc_inbound"`
	InboundId    int    `json:"inboundId"    gorm:"uniqueIndex:idx_nc_inbound"`
	Flow         string `json:"flow"` // per-link flow override (empty = use NodeClient.Flow)
}

// ClientDevice tracks devices that have accessed a client's subscription.
type ClientDevice struct {
	Id           int    `json:"id"           gorm:"primaryKey;autoIncrement"`
	NodeClientId int    `json:"nodeClientId" gorm:"index"`
	SubID        string `json:"subId"        gorm:"index"`
	ClientEmail  string `json:"clientEmail"  gorm:"index"`
	HWID         string `json:"hwid"         gorm:"index"`
	Model        string `json:"model"`
	OS           string `json:"os"`
	ClientApp    string `json:"clientApp"`
	UserAgent    string `json:"userAgent"`
	IP           string `json:"ip"`
	LastSeen     int64  `json:"lastSeen"`
	CreatedAt    int64  `json:"createdAt"`
}

// GenerateFakeTLSSecret builds an MTProto FakeTLS secret for the given domain:
// the "ee" FakeTLS marker, 16 random bytes, then the domain encoded as hex.
func GenerateFakeTLSSecret(domain string) string {
	if domain == "" {
		domain = "google.com"
	}
	return "ee" + mtprotoRandomMiddle() + hex.EncodeToString([]byte(domain))
}

// IsValidMtprotoSecret reports whether secret is a well-formed FakeTLS MTProto secret.
// FakeTLS secrets begin with "ee" (or "dd") followed by at least 16 hex bytes and domain hex,
// or represent a base64-encoded secret of sufficient length. Trojan/alphanumeric passwords return false.
func IsValidMtprotoSecret(secret string) bool {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return false
	}
	if strings.HasPrefix(secret, "ee") || strings.HasPrefix(secret, "dd") {
		if len(secret) < 34 {
			return false
		}
		_, err := hex.DecodeString(secret)
		return err == nil
	}
	if len(secret) >= 22 {
		if b, err := base64.StdEncoding.DecodeString(secret); err == nil && len(b) >= 17 {
			return true
		}
		if b, err := base64.RawStdEncoding.DecodeString(secret); err == nil && len(b) >= 17 {
			return true
		}
	}
	return false
}

// EnsureMtprotoSecret guarantees a valid FakeTLS secret is returned. If the provided
// secret is not a valid FakeTLS secret (such as a Trojan password or arbitrary text),
// a fresh valid FakeTLS secret is generated.
func EnsureMtprotoSecret(secret string, domain string) string {
	if IsValidMtprotoSecret(secret) {
		return strings.TrimSpace(secret)
	}
	return GenerateFakeTLSSecret(domain)
}

func mtprotoRandomMiddle() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("mtproto: crypto/rand read failed: %w", err))
	}
	return hex.EncodeToString(buf)
}

func mtprotoSecretMiddle(secret string) string {
	s := secret
	if strings.HasPrefix(s, "ee") || strings.HasPrefix(s, "dd") {
		s = s[2:]
	}
	if len(s) >= 32 {
		mid := s[:32]
		if _, err := hex.DecodeString(mid); err == nil {
			return mid
		}
	}
	return mtprotoRandomMiddle()
}

// ValidMtprotoAdTag reports whether a Telegram advertising tag from
// @MTProxybot is well-formed: exactly 16 bytes as 32 hex characters.
func ValidMtprotoAdTag(tag string) bool {
	if len(tag) != 32 {
		return false
	}
	_, err := hex.DecodeString(tag)
	return err == nil
}

// StripMtprotoInboundSecret removes the vestigial inbound-level `secret` from an
// mtproto inbound's settings JSON.
func StripMtprotoInboundSecret(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	if _, ok := parsed["secret"]; !ok {
		return settings, false
	}
	delete(parsed, "secret")
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}

// StripMtprotoInboundAdTag drops the dead inbound-level `adTag`.
func StripMtprotoInboundAdTag(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	if _, ok := parsed["adTag"]; !ok {
		return settings, false
	}
	delete(parsed, "adTag")
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}

func mtprotoSecretDomain(secret string) string {
	s := secret
	if strings.HasPrefix(s, "ee") || strings.HasPrefix(s, "dd") {
		s = s[2:]
	}
	if len(s) <= 32 {
		return ""
	}
	decoded, err := hex.DecodeString(s[32:])
	if err != nil || len(decoded) == 0 {
		return ""
	}
	return string(decoded)
}

// HealMtprotoClientSecrets normalises every client's FakeTLS secret in an
// mtproto inbound's settings JSON.
func HealMtprotoClientSecrets(settings string) (string, bool) {
	if settings == "" {
		return settings, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		return settings, false
	}
	clients, ok := parsed["clients"].([]any)
	if !ok || len(clients) == 0 {
		return settings, false
	}
	defaultDomain, _ := parsed["fakeTlsDomain"].(string)
	defaultDomain = strings.TrimSpace(defaultDomain)
	changed := false
	for _, raw := range clients {
		client, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		secret, _ := client["secret"].(string)
		domain := mtprotoSecretDomain(secret)
		if domain == "" {
			domain = defaultDomain
		}
		if domain == "" {
			continue
		}
		expected := "ee" + mtprotoSecretMiddle(secret) + hex.EncodeToString([]byte(domain))
		if secret != expected {
			client["secret"] = expected
			changed = true
		}
	}
	if !changed {
		return settings, false
	}
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return settings, false
	}
	return string(out), true
}

