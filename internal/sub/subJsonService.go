package sub

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"x-ui/internal/database/model"
	"x-ui/internal/logger"
	"x-ui/internal/util/json_util"
	"x-ui/internal/util/random"
	"x-ui/internal/web/service"
	"x-ui/xray"
)

//go:embed default.json
var defaultJson string

type SubJsonService struct {
	configJson       map[string]interface{}
	defaultOutbounds []json_util.RawMessage
	fragmentOrNoises bool
	mux              string

	inboundService service.InboundService
	SubService     *SubService
}

func NewSubJsonService(fragment string, noises string, mux string, rules string, subService *SubService) *SubJsonService {
	var configJson map[string]interface{}
	var defaultOutbounds []json_util.RawMessage
	json.Unmarshal([]byte(defaultJson), &configJson)
	if outboundSlices, ok := configJson["outbounds"].([]interface{}); ok {
		for _, defaultOutbound := range outboundSlices {
			jsonBytes, _ := json.Marshal(defaultOutbound)
			defaultOutbounds = append(defaultOutbounds, jsonBytes)
		}
	}

	fragmentOrNoises := false
	if fragment != "" || noises != "" {
		fragmentOrNoises = true
		defaultOutboundsSettings := map[string]interface{}{
			"domainStrategy": "UseIP",
			"redirect":       "",
		}

		if fragment != "" {
			defaultOutboundsSettings["fragment"] = json_util.RawMessage(fragment)
		}

		if noises != "" {
			defaultOutboundsSettings["noises"] = json_util.RawMessage(noises)
		}

		defaultDirectOutbound := map[string]interface{}{
			"protocol": "freedom",
			"settings": defaultOutboundsSettings,
			"tag":      "direct_out",
		}
		jsonBytes, _ := json.MarshalIndent(defaultDirectOutbound, "", "  ")
		defaultOutbounds = append(defaultOutbounds, jsonBytes)
	}

	if rules != "" {
		var newRules []interface{}
		routing, _ := configJson["routing"].(map[string]interface{})
		defaultRules, _ := routing["rules"].([]interface{})
		json.Unmarshal([]byte(rules), &newRules)
		defaultRules = append(newRules, defaultRules...)
		routing["rules"] = defaultRules
		configJson["routing"] = routing
	}

	return &SubJsonService{
		configJson:       configJson,
		defaultOutbounds: defaultOutbounds,
		fragmentOrNoises: fragmentOrNoises,
		mux:              mux,
		SubService:       subService,
	}
}

func (s *SubJsonService) GetJson(subId string, host string) (string, string, error) {
	inbounds, err := s.SubService.getInboundsBySubId(subId)
	if err != nil || len(inbounds) == 0 {
		return "", "", err
	}

	var header string
	var traffic xray.ClientTraffic
	var clientTraffics []xray.ClientTraffic
	var configArray []json_util.RawMessage

	// Prepare Inbounds
	for _, inbound := range inbounds {
		clients, err := s.SubService.getClients(inbound)
		if err != nil {
			logger.Error("SubJsonService - GetClients: Unable to get clients from inbound")
		}
		if len(clients) == 0 {
			continue
		}
		if len(inbound.Listen) > 0 && inbound.Listen[0] == '@' {
			listen, port, streamSettings, err := s.SubService.getFallbackMaster(inbound.Listen, inbound.StreamSettings)
			if err == nil {
				inbound.Listen = listen
				inbound.Port = port
				inbound.StreamSettings = streamSettings
			}
		}

		for _, client := range clients {
			if client.Enable && client.SubID == subId {
				clientTraffics = append(clientTraffics, s.SubService.getClientTraffics(inbound.ClientStats, client.Email))
				newConfigs := s.getConfig(inbound, client, host)
				configArray = append(configArray, newConfigs...)
			}
		}
	}

	if len(configArray) == 0 {
		return "", "", nil
	}

	// Prepare statistics
	traffic = s.SubService.prepareSubscriptionTraffic(subId, clientTraffics)

	// Combile outbounds
	var finalJson []byte
	if len(configArray) == 1 {
		finalJson, _ = json.MarshalIndent(configArray[0], "", "  ")
	} else {
		finalJson, _ = json.MarshalIndent(configArray, "", "  ")
	}

	header = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return string(finalJson), header, nil
}

func (s *SubJsonService) getConfig(inbound *model.Inbound, client model.Client, host string) []json_util.RawMessage {
	var newJsonArray []json_util.RawMessage
	stream := s.streamData(inbound.StreamSettings)

	externalProxies, ok := stream["externalProxy"].([]interface{})
	if !ok || len(externalProxies) == 0 {
		externalProxies = []interface{}{
			map[string]interface{}{
				"forceTls": "same",
				"dest":     host,
				"port":     float64(inbound.Port),
				"remark":   "",
			},
		}
	}

	delete(stream, "externalProxy")

	for _, ep := range externalProxies {
		extPrxy, ok := ep.(map[string]interface{})
		if !ok {
			continue
		}
		if dest, ok := extPrxy["dest"].(string); ok {
			inbound.Listen = dest
		}
		if port, ok := extPrxy["port"].(float64); ok {
			inbound.Port = int(port)
		} else if portInt, ok := extPrxy["port"].(int); ok {
			inbound.Port = portInt
		}
		newStream := make(map[string]interface{}, len(stream))
		for k, v := range stream {
			newStream[k] = v
		}
		forceTls, _ := extPrxy["forceTls"].(string)
		switch forceTls {
		case "tls":
			if newStream["security"] != "tls" {
				newStream["security"] = "tls"
				newStream["tlsSettings"] = map[string]interface{}{}
			}
		case "none":
			if newStream["security"] != "none" {
				newStream["security"] = "none"
				delete(newStream, "tlsSettings")
			}
		}
		streamSettings, _ := json.MarshalIndent(newStream, "", "  ")

		var newOutbounds []json_util.RawMessage

		switch inbound.Protocol {
		case "vmess":
			newOutbounds = append(newOutbounds, s.genVnext(inbound, streamSettings, client, ""))
		case "vless":
			var vlessSettings model.VLESSSettings
			_ = json.Unmarshal([]byte(inbound.Settings), &vlessSettings)

			enc := vlessSettings.Encryption
			if (enc == "" || enc == "none") && vlessSettings.Decryption != "" && vlessSettings.Decryption != "none" {
				enc = vlessSettings.Decryption
			}

			newOutbounds = append(newOutbounds,
				s.genVnext(inbound, streamSettings, client, enc))
		case "trojan", "shadowsocks":
			newOutbounds = append(newOutbounds, s.genServer(inbound, streamSettings, client))
		case "hysteria":
			newOutbounds = append(newOutbounds, s.genHy(inbound, newStream, client))
		case "wireguard":
			newOutbounds = append(newOutbounds, s.genWireguard(inbound, client))
		}

		newOutbounds = append(newOutbounds, s.defaultOutbounds...)
		newConfigJson := make(map[string]interface{})
		for key, value := range s.configJson {
			newConfigJson[key] = value
		}
		newConfigJson["outbounds"] = newOutbounds
		remark, _ := extPrxy["remark"].(string)
		newConfigJson["remarks"] = s.SubService.genRemark(inbound, client.Email, remark)

		newConfig, _ := json.MarshalIndent(newConfigJson, "", "  ")
		newJsonArray = append(newJsonArray, newConfig)
	}

	return newJsonArray
}

func (s *SubJsonService) genHy(inbound *model.Inbound, newStream map[string]any, client model.Client) json_util.RawMessage {
	outbound := Outbound{}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"

	if s.mux != "" {
		outbound.Mux = json_util.RawMessage(s.mux)
	}

	var settings, stream map[string]any
	json.Unmarshal([]byte(inbound.Settings), &settings)
	var version float64
	if settings != nil {
		version, _ = settings["version"].(float64)
	}
	outbound.Settings = OutboundSettings{
		Version: int(version),
		Address: inbound.Listen,
		Port:    inbound.Port,
	}

	json.Unmarshal([]byte(inbound.StreamSettings), &stream)
	outHyStream := map[string]any{
		"version": int(version),
		"auth":    client.Auth,
	}
	if stream != nil {
		if hyStream, ok := stream["hysteriaSettings"].(map[string]any); ok {
			if udpIdleTimeout, ok := hyStream["udpIdleTimeout"].(float64); ok {
				outHyStream["udpIdleTimeout"] = int(udpIdleTimeout)
			}
			if finalmask, ok := hyStream["finalmask"].(map[string]any); ok {
				newStream["finalmask"] = finalmask
			}
		}
	}
	newStream["hysteriaSettings"] = outHyStream

	newStream["network"] = "hysteria"
	newStream["security"] = "tls"

	outbound.StreamSettings, _ = json.MarshalIndent(newStream, "", "  ")

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) streamData(stream string) map[string]interface{} {
	var streamSettings map[string]interface{}
	json.Unmarshal([]byte(stream), &streamSettings)
	security, _ := streamSettings["security"].(string)
	if security == "tls" {
		if tlsMap, ok := streamSettings["tlsSettings"].(map[string]interface{}); ok {
			streamSettings["tlsSettings"] = s.tlsData(tlsMap)
		}
	} else if security == "reality" {
		if realityMap, ok := streamSettings["realitySettings"].(map[string]interface{}); ok {
			streamSettings["realitySettings"] = s.realityData(realityMap)
		}
	}
	delete(streamSettings, "sockopt")

	if s.fragmentOrNoises {
		streamSettings["sockopt"] = json_util.RawMessage(`{"dialerProxy": "direct_out", "tcpKeepAliveIdle": 100}`)
	}

	// remove proxy protocol
	network, _ := streamSettings["network"].(string)
	switch network {
	case "tcp":
		streamSettings["tcpSettings"] = s.removeAcceptProxy(streamSettings["tcpSettings"])
	case "ws":
		streamSettings["wsSettings"] = s.removeAcceptProxy(streamSettings["wsSettings"])
	case "httpupgrade":
		streamSettings["httpupgradeSettings"] = s.removeAcceptProxy(streamSettings["httpupgradeSettings"])
	}
	return streamSettings
}

func (s *SubJsonService) removeAcceptProxy(setting interface{}) map[string]interface{} {
	netSettings, ok := setting.(map[string]interface{})
	if ok {
		delete(netSettings, "acceptProxyProtocol")
	}
	return netSettings
}

func (s *SubJsonService) tlsData(tData map[string]interface{}) map[string]interface{} {
	tlsData := make(map[string]interface{}, 1)
	if tData == nil {
		return tlsData
	}
	tlsClientSettings, _ := tData["settings"].(map[string]interface{})

	tlsData["serverName"] = tData["serverName"]
	tlsData["alpn"] = tData["alpn"]
	if fingerprint, ok := tlsClientSettings["fingerprint"].(string); ok {
		tlsData["fingerprint"] = fingerprint
	}
	if echConfigList, ok := tlsClientSettings["echConfigList"].(string); ok && strings.TrimSpace(echConfigList) != "" {
		tlsData["echConfigList"] = strings.TrimSpace(echConfigList)
	} else if echConfigList, ok := tData["echConfigList"].(string); ok && strings.TrimSpace(echConfigList) != "" {
		tlsData["echConfigList"] = strings.TrimSpace(echConfigList)
	}
	if echForceQuery, ok := tlsClientSettings["echForceQuery"].(string); ok && strings.TrimSpace(echForceQuery) != "" && echForceQuery != "none" {
		tlsData["echForceQuery"] = strings.TrimSpace(echForceQuery)
	} else if echForceQuery, ok := tData["echForceQuery"].(string); ok && strings.TrimSpace(echForceQuery) != "" && echForceQuery != "none" {
		tlsData["echForceQuery"] = strings.TrimSpace(echForceQuery)
	}
	return tlsData
}

func (s *SubJsonService) realityData(rData map[string]interface{}) map[string]interface{} {
	rltyData := make(map[string]interface{}, 1)
	if rData == nil {
		return rltyData
	}
	rltyClientSettings, _ := rData["settings"].(map[string]interface{})

	rltyData["show"] = false
	if rltyClientSettings != nil {
		rltyData["publicKey"] = rltyClientSettings["publicKey"]
		rltyData["fingerprint"] = rltyClientSettings["fingerprint"]
		rltyData["mldsa65Verify"] = rltyClientSettings["mldsa65Verify"]
	}

	// Set random data
	rltyData["spiderX"] = "/" + random.Seq(15)
	if shortIds, ok := rData["shortIds"].([]interface{}); ok && len(shortIds) > 0 {
		idx := random.Num(len(shortIds))
		if str, ok := shortIds[idx].(string); ok {
			rltyData["shortId"] = str
		} else {
			rltyData["shortId"] = fmt.Sprint(shortIds[idx])
		}
	} else {
		rltyData["shortId"] = ""
	}
	if serverNames, ok := rData["serverNames"].([]interface{}); ok && len(serverNames) > 0 {
		idx := random.Num(len(serverNames))
		if str, ok := serverNames[idx].(string); ok {
			rltyData["serverName"] = str
		} else {
			rltyData["serverName"] = fmt.Sprint(serverNames[idx])
		}
	} else {
		rltyData["serverName"] = ""
	}

	return rltyData
}

func (s *SubJsonService) genVnext(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client, encryption string) json_util.RawMessage {
	outbound := Outbound{}
	usersData := make([]UserVnext, 1)

	usersData[0].ID = client.ID
	usersData[0].Level = 8
	if inbound.Protocol == model.VMESS {
		usersData[0].Security = client.Security
	}
	if inbound.Protocol == model.VLESS {
		usersData[0].Flow = client.Flow
		usersData[0].Encryption = encryption
	}

	vnextData := make([]VnextSetting, 1)
	vnextData[0] = VnextSetting{
		Address: inbound.Listen,
		Port:    inbound.Port,
		Users:   usersData,
	}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"
	if s.mux != "" {
		outbound.Mux = json_util.RawMessage(s.mux)
	}
	outbound.StreamSettings = streamSettings
	outbound.Settings = OutboundSettings{
		Vnext: vnextData,
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

func (s *SubJsonService) genServer(inbound *model.Inbound, streamSettings json_util.RawMessage, client model.Client) json_util.RawMessage {
	outbound := Outbound{}

	serverData := make([]ServerSetting, 1)
	serverData[0] = ServerSetting{
		Address:  inbound.Listen,
		Port:     inbound.Port,
		Level:    8,
		Password: client.Password,
	}

	if inbound.Protocol == model.Shadowsocks {
		var inboundSettings map[string]interface{}
		json.Unmarshal([]byte(inbound.Settings), &inboundSettings)
		method, _ := inboundSettings["method"].(string)
		serverData[0].Method = method

		// server password in multi-user 2022 protocols
		if strings.HasPrefix(method, "2022") {
			if serverPassword, ok := inboundSettings["password"].(string); ok {
				serverData[0].Password = fmt.Sprintf("%s:%s", serverPassword, client.Password)
			}
		}
	}

	outbound.Protocol = string(inbound.Protocol)
	outbound.Tag = "proxy"
	if s.mux != "" {
		outbound.Mux = json_util.RawMessage(s.mux)
	}
	outbound.StreamSettings = streamSettings
	outbound.Settings = OutboundSettings{
		Servers: serverData,
	}

	result, _ := json.MarshalIndent(outbound, "", "  ")
	return result
}

type Outbound struct {
	Protocol       string                 `json:"protocol"`
	Tag            string                 `json:"tag"`
	StreamSettings json_util.RawMessage   `json:"streamSettings"`
	Mux            json_util.RawMessage   `json:"mux,omitempty"`
	ProxySettings  map[string]interface{} `json:"proxySettings,omitempty"`
	Settings       OutboundSettings       `json:"settings,omitempty"`
}

type OutboundSettings struct {
	Vnext   []VnextSetting  `json:"vnext,omitempty"`
	Servers []ServerSetting `json:"servers,omitempty"`
	Version int             `json:"version,omitempty"`
	Address string          `json:"address,omitempty"`
	Port    int             `json:"port,omitempty"`
}

type VnextSetting struct {
	Address string      `json:"address"`
	Port    int         `json:"port"`
	Users   []UserVnext `json:"users"`
}

type UserVnext struct {
	Encryption string `json:"encryption,omitempty"`
	Flow       string `json:"flow,omitempty"`
	ID         string `json:"id"`
	Security   string `json:"security,omitempty"`
	Level      int    `json:"level"`
}

type ServerSetting struct {
	Password string `json:"password"`
	Level    int    `json:"level"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Flow     string `json:"flow,omitempty"`
	Method   string `json:"method,omitempty"`
}

func (s *SubJsonService) genWireguard(inbound *model.Inbound, client model.Client) json_util.RawMessage {
	var settings map[string]interface{}
	json.Unmarshal([]byte(inbound.Settings), &settings)

	serverPubKey, _ := settings["pubKey"].(string)
	if serverPubKey == "" {
		if secKey, ok := settings["secretKey"].(string); ok && secKey != "" {
			serverPubKey = getWireguardPubKey(secKey)
		}
	}
	privKey := client.PrivateKey
	if privKey == "" {
		privKey = client.Password
	}
	allowedIps := []string{"10.0.0.2/32"}
	if len(client.AllowedIPs) > 0 {
		allowedIps = client.AllowedIPs
	}

	address := s.SubService.address
	if inbound.Listen != "" && inbound.Listen != "0.0.0.0" {
		address = inbound.Listen
	}

	peer := map[string]interface{}{
		"publicKey": serverPubKey,
		"endpoint":  fmt.Sprintf("%s:%d", address, inbound.Port),
	}
	if client.Psk != "" {
		peer["preSharedKey"] = client.Psk
	}
	if client.KeepAlive > 0 {
		peer["keepAlive"] = client.KeepAlive
	}
	if reserved, ok := settings["reserved"].([]interface{}); ok && len(reserved) > 0 {
		peer["reserved"] = reserved
	}

	wgSettings := map[string]interface{}{
		"secretKey": privKey,
		"address":   allowedIps,
		"peers":     []interface{}{peer},
	}
	if mtu, ok := settings["mtu"].(float64); ok && mtu > 0 {
		wgSettings["mtu"] = int(mtu)
	}

	outbound := map[string]interface{}{
		"protocol": "wireguard",
		"tag":      "proxy",
		"settings": wgSettings,
	}

	res, _ := json.Marshal(outbound)
	return json_util.RawMessage(res)
}
