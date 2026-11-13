package sub

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"x-ui/internal/util/common"
	"x-ui/internal/web/service"
)

type SubTemplateContentResult struct {
	Content  string `json:"content"`
	IsCustom bool   `json:"isCustom"`
}

func GetEmbeddedSubTemplate() (string, error) {
	bytes, err := htmlFS.ReadFile("html/sub.html")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func GetActiveSubTemplate() (string, bool, error) {
	customFolders := []string{"/etc/x-ui/html", "sub/html"}
	for _, folder := range customFolders {
		filePath := filepath.Join(folder, "sub.html")
		if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
			return string(data), true, nil
		}
	}
	content, err := GetEmbeddedSubTemplate()
	return content, false, err
}

func GetSubTemplateContent() (*SubTemplateContentResult, error) {
	content, isCustom, err := GetActiveSubTemplate()
	if err != nil {
		return nil, err
	}
	return &SubTemplateContentResult{
		Content:  content,
		IsCustom: isCustom,
	}, nil
}

func SaveSubTemplateContent(content string) error {
	content = strings.TrimSpace(content)
	if len(content) < 20 {
		return common.NewError("Template content is too short or empty")
	}

	// Validate template syntax
	tmpl, err := template.New("sub.html").Parse(content)
	if err != nil {
		return common.NewErrorf("Invalid HTML template syntax: %v", err)
	}

	var buf bytes.Buffer
	mockData := map[string]interface{}{
		"result":       []string{"vless://demo@127.0.0.1:443#Demo"},
		"total":        "100.00 GB",
		"expire":       "2026-12-31 23:59:59",
		"upload":       "10.00 GB",
		"download":     "20.00 GB",
		"totalByte":    int64(100 * 1024 * 1024 * 1024),
		"uploadByte":   int64(10 * 1024 * 1024 * 1024),
		"downloadByte": int64(20 * 1024 * 1024 * 1024),
		"sId":          "demo",
		"emails":       []string{"demo@example.com"},
		"subUrl":       "https://example.com/sub/demo",
		"jsonUrl":      "https://example.com/json/demo",
	}
	if err := tmpl.Execute(&buf, mockData); err != nil {
		return common.NewErrorf("Template execution test failed: %v", err)
	}

	targetDirs := []string{
		"sub/html",
		"/etc/x-ui/html",
	}

	written := false
	bodyBytes := []byte(content)
	for _, dir := range targetDirs {
		if err := os.MkdirAll(dir, 0755); err == nil {
			subPath := filepath.Join(dir, "sub.html")
			indexPath := filepath.Join(dir, "index.html")
			_ = os.WriteFile(subPath, bodyBytes, 0644)
			_ = os.WriteFile(indexPath, bodyBytes, 0644)
			written = true
		}
	}

	if !written {
		return common.NewError("Failed to write sub template to filesystem")
	}

	settingService := service.SettingService{}
	_ = settingService.SetSubCustomUI(true)
	panelService := service.PanelService{}
	_ = panelService.RestartPanel(time.Second * 1)
	return nil
}

func PreviewSubTemplate(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		c, _, err := GetActiveSubTemplate()
		if err != nil {
			return "", err
		}
		content = c
	}

	tmpl, err := template.New("sub.html").Parse(content)
	if err != nil {
		return "", common.NewErrorf("Template syntax error: %v", err)
	}

	mockData := map[string]interface{}{
		"result": []string{
			"vless://11111111-2222-3333-4444-555555555555@example.com:443?type=tcp&security=reality&pbk=mockPubKey123&fp=chrome&sni=example.com&sid=mockSid#VLESS-Reality",
			"vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsImFpZCI6IjAiLCJob3N0IjoiZXhhbXBsZS5jb20iLCJpZCI6IjExMTExMTExLTIyMjItMzMzMy00NDQ0LTU1NTU1NTU1NTU1NSIsIm5ldCI6IndzIiwicGF0aCI6Ii93cyIsInBvcnQiOiI0NDMiLCJwcyI6IlZNZXNzLVdTIiwic2N5IjoiYXV0byIsInNuaSI6ImV4YW1wbGUuY29tIiwidGxzIjoidGxzIiwidHlwZSI6Im5vbmUiLCJ2IjoiMiJ9",
			"trojan://mockPassword123@example.com:443?security=tls&sni=example.com#Trojan-TLS",
			"wireguard://mockPrivKey123=@example.com:51820?publickey=mockServerPub123=&address=10.0.0.2/32&mtu=1420&reserved=0,0,0#WireGuard",
			"hysteria2://mockPassword123@example.com:443?insecure=0&sni=example.com#Hysteria2",
		},
		"total":        "150.00 GB",
		"expire":       "2026-12-31 23:59:59",
		"upload":       "24.50 GB",
		"download":     "62.80 GB",
		"totalByte":    int64(150 * 1024 * 1024 * 1024),
		"uploadByte":   int64(24500000000),
		"downloadByte": int64(62800000000),
		"sId":          "demo-live-preview",
		"emails":       []string{"demo-user@example.com"},
		"subUrl":       "https://example.com/sub/demo-live-preview",
		"jsonUrl":      "https://example.com/json/demo-live-preview",
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, mockData); err != nil {
		return "", common.NewErrorf("Template rendering error: %v", err)
	}

	return buf.String(), nil
}
