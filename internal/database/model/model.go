package model

import (
	"encoding/json"
	"fmt"

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
	}
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       string(i.Protocol),
		Settings:       json_util.RawMessage(settings),
		StreamSettings: json_util.RawMessage(i.StreamSettings),
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
	Flow       string         `json:"flow,omitempty"`     // Flow control (XTLS)
	Auth       string         `json:"auth,omitempty"`     // Auth password (Hysteria)
	PublicKey  string         `json:"publicKey,omitempty"`  // WireGuard Public Key
	PrivateKey string         `json:"privateKey,omitempty"` // WireGuard Private Key
	AllowedIPs []string       `json:"allowedIPs,omitempty"` // WireGuard Allowed IPs
	Psk        string         `json:"psk,omitempty"`        // WireGuard Preshared Key
	KeepAlive  int            `json:"keepAlive,omitempty"`  // WireGuard KeepAlive
	Reverse    *ClientReverse `json:"reverse" omitEmpty:"true"`
	Email      string         `json:"email"`
	LimitIP    int            `json:"limitIp"`
	TotalGB    int64          `json:"totalGB" form:"totalGB"`
	ExpiryTime int64          `json:"expiryTime" form:"expiryTime"`
	Enable     bool           `json:"enable" form:"enable"`
	TgID       int64          `json:"tgId" form:"tgId"`
	SubID      string         `json:"subId" form:"subId"`
	Comment    string         `json:"comment" form:"comment"`
	Reset      int            `json:"reset" form:"reset"`
}

type VLESSSettings struct {
	Clients    []Client `json:"clients"`
	Decryption string   `json:"decryption"`
	Encryption string   `json:"encryption"`
	Fallbacks  []any    `json:"fallbacks"`
}

// NodeClient is the shared, protocol-agnostic client identity.
type NodeClient struct {
	Id         int    `json:"id"         form:"id"         gorm:"primaryKey;autoIncrement"`
	Email      string `json:"email"      form:"email"      gorm:"uniqueIndex"`
	SubID      string `json:"subId"      form:"subId"      gorm:"uniqueIndex"`
	UUID       string `json:"uuid"       form:"uuid"       gorm:"column:uuid"` // for vmess/vless
	Password   string `json:"password"   form:"password"`                      // for trojan/shadowsocks
	Auth       string `json:"auth"       form:"auth"`                          // for hysteria
	Security   string `json:"security"   form:"security"`
	Flow       string `json:"flow"       form:"flow"` // global flow default; per-link override wins
	TotalGB    int64  `json:"totalGB"    form:"totalGB"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	LimitIP    int    `json:"limitIp"    form:"limitIp"`
	TgID       int64  `json:"tgId"       form:"tgId"`
	Enable     bool   `json:"enable"     form:"enable"     gorm:"default:true"`
	Reset      int    `json:"reset"      form:"reset"` // auto-reset interval in days (0 = off)
	Comment    string `json:"comment"    form:"comment"`
}

// NodeClientLink joins a NodeClient to an Inbound, with optional per-link overrides.
type NodeClientLink struct {
	Id           int    `json:"id"           gorm:"primaryKey;autoIncrement"`
	NodeClientId int    `json:"nodeClientId" gorm:"uniqueIndex:idx_nc_inbound"`
	InboundId    int    `json:"inboundId"    gorm:"uniqueIndex:idx_nc_inbound"`
	Flow         string `json:"flow"` // per-link flow override (empty = use NodeClient.Flow)
}
