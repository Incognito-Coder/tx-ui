package sub

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func createTestContext(targetURL string, headers map[string]string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", targetURL, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c
}

func TestParseDeviceInfo_QueryParams(t *testing.T) {
	c := createTestContext("/sub/mytestsub?hwid=HWID-9876-XYZ&model=SM-S928B", nil)
	info := ParseDeviceInfo(c)

	if info.HWID != "HWID-9876-XYZ" {
		t.Fatalf("expected HWID-9876-XYZ, got %s", info.HWID)
	}
	if info.Model != "SM-S928B" {
		t.Fatalf("expected SM-S928B, got %s", info.Model)
	}

	// Test device_id and device aliases
	c2 := createTestContext("/sub/mytestsub?device_id=MY-DEVICE-ID&device=Pixel-8-Pro", nil)
	info2 := ParseDeviceInfo(c2)

	if info2.HWID != "MY-DEVICE-ID" {
		t.Fatalf("expected MY-DEVICE-ID, got %s", info2.HWID)
	}
	if info2.Model != "Pixel-8-Pro" {
		t.Fatalf("expected Pixel-8-Pro, got %s", info2.Model)
	}
}

func TestParseDeviceInfo_Headers(t *testing.T) {
	headers := map[string]string{
		"X-HWID":         "HEADER-HWID-555",
		"X-Device-Model": "Sony Xperia 1 V",
		"X-OS":           "Android 14",
		"X-Client-App":   "CustomClient 2.0",
	}
	c := createTestContext("/sub/mytestsub", headers)
	info := ParseDeviceInfo(c)

	if info.HWID != "HEADER-HWID-555" {
		t.Fatalf("expected HEADER-HWID-555, got %s", info.HWID)
	}
	if info.Model != "Sony Xperia 1 V" {
		t.Fatalf("expected Sony Xperia 1 V, got %s", info.Model)
	}
	if info.OS != "Android 14" {
		t.Fatalf("expected Android 14, got %s", info.OS)
	}
	if info.ClientApp != "CustomClient 2.0" {
		t.Fatalf("expected CustomClient 2.0, got %s", info.ClientApp)
	}
}

func TestParseDeviceInfo_UserAgents(t *testing.T) {
	tests := []struct {
		name          string
		ua            string
		expectedModel string
		expectedOS    string
		expectedApp   string
	}{
		{
			name:          "Shadowrocket iPhone",
			ua:            "Shadowrocket/2.2.33 (iPhone15,2; iOS 17.4.1)",
			expectedModel: "iPhone 14 Pro",
			expectedOS:    "iOS 17.4.1",
			expectedApp:   "Shadowrocket 2.2.33",
		},
		{
			name:          "Sing-box iPhone 13",
			ua:            "Sing-box/1.9.0 (iPhone14,5; iOS 16.6)",
			expectedModel: "iPhone 13",
			expectedOS:    "iOS 16.6",
			expectedApp:   "Sing-box 1.9.0",
		},
		{
			name:          "v2rayNG Samsung S24 Ultra",
			ua:            "v2rayNG/1.8.19 (Linux; Android 14; SM-S928B Build/UP1A.231005.007)",
			expectedModel: "Samsung Galaxy S24 Ultra (SM-S928B)",
			expectedOS:    "Android 14",
			expectedApp:   "v2rayNG 1.8.19",
		},
		{
			name:          "Pixel 7 Pro",
			ua:            "ClashMetaForAndroid/2.10.1 (Android 13; Pixel 7 Pro)",
			expectedModel: "Google Pixel 7 Pro",
			expectedOS:    "Android 13",
			expectedApp:   "ClashMetaForAndroid 2.10.1",
		},
		{
			name:          "Windows PC v2rayN",
			ua:            "v2rayN/6.39 (Windows NT 10.0; Win64; x64)",
			expectedModel: "Windows PC",
			expectedOS:    "Windows 10/11",
			expectedApp:   "v2rayN 6.39",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := createTestContext("/sub/mytestsub", map[string]string{
				"User-Agent": tt.ua,
			})
			info := ParseDeviceInfo(c)

			if !strings.Contains(info.Model, tt.expectedModel) {
				t.Errorf("[%s] expected model %s, got %s", tt.name, tt.expectedModel, info.Model)
			}
			if info.OS != tt.expectedOS {
				t.Errorf("[%s] expected OS %s, got %s", tt.name, tt.expectedOS, info.OS)
			}
			if info.ClientApp != tt.expectedApp {
				t.Errorf("[%s] expected App %s, got %s", tt.name, tt.expectedApp, info.ClientApp)
			}
		})
	}
}
