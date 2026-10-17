package sub

import (
	"regexp"
	"strings"
	"time"

	"x-ui/internal/database"
	"x-ui/internal/database/model"

	"github.com/gin-gonic/gin"
)

// DeviceInfo represents device and client environment parsed from a subscription request.
type DeviceInfo struct {
	HWID      string
	Model     string
	OS        string
	ClientApp string
	UserAgent string
	IP        string
}

// Known Apple identifiers mapping to human-friendly product names.
var appleDeviceMap = map[string]string{
	"iPhone17,1": "iPhone 16 Pro",
	"iPhone17,2": "iPhone 16 Pro Max",
	"iPhone17,3": "iPhone 16",
	"iPhone17,4": "iPhone 16 Plus",
	"iPhone16,1": "iPhone 15 Pro",
	"iPhone16,2": "iPhone 15 Pro Max",
	"iPhone15,4": "iPhone 15",
	"iPhone15,5": "iPhone 15 Plus",
	"iPhone15,2": "iPhone 14 Pro",
	"iPhone15,3": "iPhone 14 Pro Max",
	"iPhone14,7": "iPhone 14",
	"iPhone14,8": "iPhone 14 Plus",
	"iPhone14,2": "iPhone 13 Pro",
	"iPhone14,3": "iPhone 13 Pro Max",
	"iPhone14,5": "iPhone 13",
	"iPhone14,4": "iPhone 13 mini",
	"iPhone14,6": "iPhone SE (3rd gen)",
	"iPhone13,1": "iPhone 12 mini",
	"iPhone13,2": "iPhone 12",
	"iPhone13,3": "iPhone 12 Pro",
	"iPhone13,4": "iPhone 12 Pro Max",
	"iPhone12,1": "iPhone 11",
	"iPhone12,3": "iPhone 11 Pro",
	"iPhone12,5": "iPhone 11 Pro Max",
	"iPhone12,8": "iPhone SE (2nd gen)",
	"iPhone11,2": "iPhone XS",
	"iPhone11,4": "iPhone XS Max",
	"iPhone11,6": "iPhone XS Max",
	"iPhone11,8": "iPhone XR",
	"iPhone10,3": "iPhone X",
	"iPhone10,6": "iPhone X",
	"iPhone10,1": "iPhone 8",
	"iPhone10,4": "iPhone 8",
	"iPhone10,2": "iPhone 8 Plus",
	"iPhone10,5": "iPhone 8 Plus",
	"iPad14,1":  "iPad mini (6th gen)",
	"iPad14,2":  "iPad mini (6th gen)",
	"iPad13,1":  "iPad Air (4th gen)",
	"iPad13,2":  "iPad Air (4th gen)",
	"iPad13,16": "iPad Air (5th gen)",
	"iPad13,17": "iPad Air (5th gen)",
	"iPad14,8":  "iPad Air 11-inch (M2)",
	"iPad14,9":  "iPad Air 11-inch (M2)",
	"iPad14,10": "iPad Air 13-inch (M2)",
	"iPad14,11": "iPad Air 13-inch (M2)",
	"iPad13,4":  "iPad Pro 11-inch (3rd gen)",
	"iPad13,8":  "iPad Pro 12.9-inch (5th gen)",
	"iPad14,3":  "iPad Pro 11-inch (4th gen)",
	"iPad14,5":  "iPad Pro 12.9-inch (6th gen)",
	"iPad16,3":  "iPad Pro 11-inch (M4)",
	"iPad16,5":  "iPad Pro 13-inch (M4)",
	"iPad11,6":  "iPad (8th gen)",
	"iPad12,1":  "iPad (9th gen)",
	"iPad13,18": "iPad (10th gen)",
}

// Known Samsung prefixes mapping to marketing names.
var samsungModelMap = map[string]string{
	"SM-S928": "Galaxy S24 Ultra",
	"SM-S926": "Galaxy S24+",
	"SM-S921": "Galaxy S24",
	"SM-S918": "Galaxy S23 Ultra",
	"SM-S916": "Galaxy S23+",
	"SM-S911": "Galaxy S23",
	"SM-S908": "Galaxy S22 Ultra",
	"SM-S906": "Galaxy S22+",
	"SM-S901": "Galaxy S22",
	"SM-G998": "Galaxy S21 Ultra",
	"SM-G996": "Galaxy S21+",
	"SM-G991": "Galaxy S21",
	"SM-G988": "Galaxy S20 Ultra",
	"SM-A546": "Galaxy A54 5G",
	"SM-A556": "Galaxy A55 5G",
	"SM-A346": "Galaxy A34 5G",
	"SM-A356": "Galaxy A35 5G",
	"SM-A536": "Galaxy A53 5G",
	"SM-A528": "Galaxy A52s 5G",
	"SM-F946": "Galaxy Z Fold5",
	"SM-F731": "Galaxy Z Flip5",
	"SM-F956": "Galaxy Z Fold6",
	"SM-F741": "Galaxy Z Flip6",
}

// Compiled regexes for User-Agent parsing.
var (
	reAndroid = regexp.MustCompile(`(?i)(?:Linux; )?(?:U; )?Android\s+([\d\.]+)(?:;\s*([a-z]{2}-[a-z]{2}))?;\s*([^;\)]+?)(?:\s+Build/[^\)]+)?\)`)
	reAppleID = regexp.MustCompile(`(iPhone\d+,\d+|iPad\d+,\d+|iPod\d+,\d+)`)
	reIOSVer  = regexp.MustCompile(`(?i)(?:iPhone OS|iOS|iPadOS)[\s_/]+([\d_\.]+)`)

	// Common VPN client regexes.
	reClients = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(v2rayNG)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(v2rayN)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(V2Box)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Streisand)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Throne)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(HappProxy|Happ)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Exclave)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(HiddifyNext|Hiddify)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Shadowrocket)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Sing-box|sing-box|SFA|SFM|SFW)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(ClashMetaForAndroid)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(ClashMeta)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Clash\.Meta)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(ClashForAndroid)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Clash-verge)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Clash\.Verge)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Clash)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(NekoBoxForAndroid)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(NekoBox)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(NekoRay)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Quantumult%20X|Quantumult)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Surge)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Loon)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Stash)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Flclash)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Karing)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(FoXray)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(Matsuri)/([\w\.\-]+)`),
		regexp.MustCompile(`(?i)(PassWall)`),
		regexp.MustCompile(`(?i)(OpenClash)`),
		regexp.MustCompile(`(?i)(curl)/([\w\.\-]+)`),
	}
)

// ParseDeviceInfo extracts HWID, device model, OS, client app, IP, and UA from a request.
func ParseDeviceInfo(c *gin.Context) *DeviceInfo {
	info := &DeviceInfo{
		IP:        c.ClientIP(),
		UserAgent: strings.TrimSpace(c.GetHeader("User-Agent")),
	}

	// 1. Extract HWID from query parameters first (e.g. ?hwid=... or ?device_id=...)
	for _, param := range []string{"hwid", "device_id", "deviceId", "client_id", "clientId", "uid"} {
		if val := strings.TrimSpace(c.Query(param)); val != "" {
			info.HWID = cleanString(val, 128)
			break
		}
	}
	// Fallback to headers for HWID
	if info.HWID == "" {
		for _, hdr := range []string{"X-HWID", "X-Device-Id", "X-Device-ID", "X-Client-HWID", "HWID", "Device-Id"} {
			if val := strings.TrimSpace(c.GetHeader(hdr)); val != "" {
				info.HWID = cleanString(val, 128)
				break
			}
		}
	}

	// 2. Extract Device Model from query parameters first (e.g. ?model=... or ?device=...)
	for _, param := range []string{"model", "device", "device_model", "deviceModel", "phone", "deviceName"} {
		if val := strings.TrimSpace(c.Query(param)); val != "" {
			info.Model = cleanString(val, 64)
			break
		}
	}
	// Fallback to headers for Model
	if info.Model == "" {
		for _, hdr := range []string{"X-Device-Model", "X-Model", "X-Device-Name", "Device-Model"} {
			if val := strings.TrimSpace(c.GetHeader(hdr)); val != "" {
				info.Model = cleanString(val, 64)
				break
			}
		}
	}

	// 3. Extract OS & App from query or headers if explicitly provided
	if val := strings.TrimSpace(c.Query("os")); val != "" {
		info.OS = cleanString(val, 32)
	} else if val := strings.TrimSpace(c.GetHeader("X-OS")); val != "" {
		info.OS = cleanString(val, 32)
	} else if val := strings.TrimSpace(c.GetHeader("X-Client-OS")); val != "" {
		info.OS = cleanString(val, 32)
	}

	if val := strings.TrimSpace(c.Query("app")); val != "" {
		info.ClientApp = cleanString(val, 64)
	} else if val := strings.TrimSpace(c.GetHeader("X-Client")); val != "" {
		info.ClientApp = cleanString(val, 64)
	} else if val := strings.TrimSpace(c.GetHeader("X-Client-App")); val != "" {
		info.ClientApp = cleanString(val, 64)
	}

	// 4. Parse from User-Agent if model, OS, or client app are not yet determined
	if info.UserAgent != "" {
		parseUserAgent(info)
	}

	return info
}

// parseUserAgent parses client app, operating system, and hardware model from User-Agent.
func parseUserAgent(info *DeviceInfo) {
	ua := info.UserAgent

	// Extract Client App
	if info.ClientApp == "" {
		for _, re := range reClients {
			m := re.FindStringSubmatch(ua)
			if len(m) >= 3 {
				appClean := strings.ReplaceAll(m[1], "%20", " ")
				info.ClientApp = appClean + " " + m[2]
				break
			} else if len(m) == 2 {
				info.ClientApp = m[1]
				break
			}
		}
	}

	// Generic app fallback if not in known list (e.g. CustomApp/1.0.0)
	if info.ClientApp == "" {
		reGeneric := regexp.MustCompile(`^([a-zA-Z0-9_\-\.]+)/([0-9\.\-]+)`)
		if m := reGeneric.FindStringSubmatch(ua); len(m) >= 3 {
			lower := strings.ToLower(m[1])
			if lower != "mozilla" && lower != "cfnetwork" && lower != "darwin" && lower != "applewebkit" && lower != "gecko" && lower != "chrome" && lower != "safari" {
				info.ClientApp = m[1] + " " + m[2]
			}
		}
	}

	// Check Apple Device Identifiers (e.g. iPhone15,2, iPad13,8)
	if appleMatches := reAppleID.FindStringSubmatch(ua); len(appleMatches) > 1 {
		rawAppleID := appleMatches[1]
		if friendlyName, ok := appleDeviceMap[rawAppleID]; ok {
			if info.Model == "" {
				info.Model = friendlyName
			}
		} else if info.Model == "" {
			info.Model = rawAppleID
		}
		if info.OS == "" {
			if iosMatch := reIOSVer.FindStringSubmatch(ua); len(iosMatch) > 1 {
				info.OS = "iOS " + strings.ReplaceAll(iosMatch[1], "_", ".")
			} else if strings.Contains(ua, "iPad") {
				info.OS = "iPadOS"
			} else {
				info.OS = "iOS"
			}
		}
		return
	}

	// Generic iPhone/iPad/iPod in UA
	if strings.Contains(ua, "iPhone") {
		if info.Model == "" {
			info.Model = "iPhone"
		}
		if info.OS == "" {
			if iosMatch := reIOSVer.FindStringSubmatch(ua); len(iosMatch) > 1 {
				info.OS = "iOS " + strings.ReplaceAll(iosMatch[1], "_", ".")
			} else {
				info.OS = "iOS"
			}
		}
		return
	} else if strings.Contains(ua, "iPad") {
		if info.Model == "" {
			info.Model = "iPad"
		}
		if info.OS == "" {
			info.OS = "iPadOS"
		}
		return
	}

	// Check Android
	if andMatch := reAndroid.FindStringSubmatch(ua); len(andMatch) > 3 {
		androidVer := andMatch[1]
		deviceRaw := strings.TrimSpace(andMatch[3])

		if info.OS == "" {
			info.OS = "Android " + androidVer
		}
		if info.Model == "" && deviceRaw != "" {
			info.Model = formatAndroidModel(deviceRaw)
		}
		return
	}

	// Desktop / PC Platforms
	if strings.Contains(ua, "Windows NT 10.0") {
		if info.OS == "" {
			info.OS = "Windows 10/11"
		}
		if info.Model == "" {
			info.Model = "Windows PC"
		}
	} else if strings.Contains(ua, "Windows NT 6.3") {
		if info.OS == "" {
			info.OS = "Windows 8.1"
		}
		if info.Model == "" {
			info.Model = "Windows PC"
		}
	} else if strings.Contains(ua, "Windows NT 6.1") {
		if info.OS == "" {
			info.OS = "Windows 7"
		}
		if info.Model == "" {
			info.Model = "Windows PC"
		}
	} else if strings.Contains(ua, "Macintosh") {
		if info.OS == "" {
			info.OS = "macOS"
		}
		if info.Model == "" {
			if strings.Contains(ua, "arm64") {
				info.Model = "Mac (Apple Silicon)"
			} else {
				info.Model = "Mac"
			}
		}
	} else if strings.Contains(ua, "Darwin") || strings.Contains(ua, "CFNetwork") {
		// iOS or Apple Darwin client
		reDarwin := regexp.MustCompile(`Darwin/(\d+)\.`)
		darwinMajor := ""
		if dm := reDarwin.FindStringSubmatch(ua); len(dm) > 1 {
			darwinMajor = dm[1]
		}
		if info.OS == "" {
			switch darwinMajor {
			case "24":
				info.OS = "iOS 18"
			case "23":
				info.OS = "iOS 17"
			case "22":
				info.OS = "iOS 16"
			case "21":
				info.OS = "iOS 15"
			case "20":
				info.OS = "iOS 14"
			default:
				info.OS = "iOS"
			}
		}
		if info.Model == "" {
			if strings.Contains(ua, "iPad") {
				info.Model = "iPad"
			} else {
				info.Model = "iPhone"
			}
		}
	} else if strings.Contains(ua, "Linux") && !strings.Contains(ua, "Android") {
		if info.OS == "" {
			info.OS = "Linux"
		}
		if info.Model == "" {
			info.Model = "Linux PC"
		}
	}
}

// formatAndroidModel formats raw Android model strings into friendly names.
func formatAndroidModel(raw string) string {
	raw = strings.TrimSpace(raw)
	// Check Samsung model prefixes (e.g. SM-S918B -> SM-S918)
	if strings.HasPrefix(strings.ToUpper(raw), "SM-") {
		upper := strings.ToUpper(raw)
		for prefix, friendly := range samsungModelMap {
			if strings.HasPrefix(upper, prefix) {
				return "Samsung " + friendly + " (" + raw + ")"
			}
		}
		return "Samsung " + raw
	}
	// Google Pixel
	if strings.HasPrefix(strings.ToLower(raw), "pixel") {
		return "Google " + raw
	}
	return raw
}

// cleanString trims and limits string length to prevent oversized inputs.
func cleanString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}

// GetBaseApp extracts the base application name without version (e.g., "v2rayNG 1.8.19" -> "v2rayNG").
func GetBaseApp(clientApp string) string {
	clientApp = strings.TrimSpace(clientApp)
	if idx := strings.Index(clientApp, " "); idx != -1 {
		return clientApp[:idx]
	}
	return clientApp
}

// IsGenericModel returns true if the model name is generic or a platform placeholder.
func IsGenericModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return m == "" || m == "device" || m == "iphone" || m == "ipad" || m == "windows pc" || m == "mac" || m == "linux pc"
}

// MatchesDeviceInfo checks whether a recorded ClientDevice corresponds to incoming DeviceInfo.
func MatchesDeviceInfo(dev model.ClientDevice, info *DeviceInfo) bool {
	if info == nil {
		return false
	}

	// 1. If HWID is present on both, HWID is authoritative
	if info.HWID != "" && dev.HWID != "" {
		return strings.EqualFold(dev.HWID, info.HWID)
	}

	// 2. Specific hardware model matching (e.g. "Samsung Galaxy S24 Ultra (SM-S928B)", "iPhone 14 Pro", "Pixel 8 Pro")
	if info.Model != "" && dev.Model != "" && !IsGenericModel(info.Model) && !IsGenericModel(dev.Model) {
		if strings.EqualFold(dev.Model, info.Model) {
			return true
		}
	}

	// 3. Client application match (e.g. v2rayNG, Streisand, Shadowrocket, V2Box, etc.)
	baseAppInfo := strings.ToLower(GetBaseApp(info.ClientApp))
	baseAppDev := strings.ToLower(GetBaseApp(dev.ClientApp))
	if baseAppInfo != "" && baseAppDev != "" && baseAppInfo == baseAppDev {
		if info.Model == "" || dev.Model == "" || IsGenericModel(info.Model) || IsGenericModel(dev.Model) || strings.EqualFold(dev.Model, info.Model) {
			return true
		}
	}

	// 4. Exact Model + OS match
	if info.Model != "" && dev.Model != "" && strings.EqualFold(dev.Model, info.Model) {
		if info.OS != "" && dev.OS != "" && strings.EqualFold(dev.OS, info.OS) {
			return true
		}
	}

	// 5. User-Agent exact match
	if info.UserAgent != "" && dev.UserAgent != "" && info.UserAgent == dev.UserAgent {
		return true
	}

	// 6. IP match as last fallback when no app/model info is available
	if (info.Model == "" || IsGenericModel(info.Model)) && info.ClientApp == "" && dev.ClientApp == "" {
		if info.IP != "" && dev.IP != "" && info.IP == dev.IP {
			return true
		}
	}

	return false
}

// IsSameDevice checks whether two ClientDevice records represent the same device.
func IsSameDevice(a, b model.ClientDevice) bool {
	if a.Id > 0 && b.Id > 0 && a.Id == b.Id {
		return true
	}
	if a.HWID != "" && b.HWID != "" {
		return strings.EqualFold(a.HWID, b.HWID)
	}

	if a.Model != "" && b.Model != "" && !IsGenericModel(a.Model) && !IsGenericModel(b.Model) {
		if strings.EqualFold(a.Model, b.Model) {
			return true
		}
	}

	baseAppA := strings.ToLower(GetBaseApp(a.ClientApp))
	baseAppB := strings.ToLower(GetBaseApp(b.ClientApp))
	if baseAppA != "" && baseAppB != "" && baseAppA == baseAppB {
		if a.Model == "" || b.Model == "" || IsGenericModel(a.Model) || IsGenericModel(b.Model) || strings.EqualFold(a.Model, b.Model) {
			return true
		}
	}

	if a.Model != "" && b.Model != "" && strings.EqualFold(a.Model, b.Model) {
		if a.OS != "" && b.OS != "" && strings.EqualFold(a.OS, b.OS) {
			return true
		}
	}

	if a.UserAgent != "" && b.UserAgent != "" && a.UserAgent == b.UserAgent {
		return true
	}

	if (a.Model == "" || IsGenericModel(a.Model)) && (b.Model == "" || IsGenericModel(b.Model)) && a.ClientApp == "" && b.ClientApp == "" {
		if a.IP != "" && b.IP != "" && a.IP == b.IP {
			return true
		}
	}

	return false
}

// DeduplicateDevices removes duplicate device records, retaining the most recently seen entry.
func DeduplicateDevices(devices []model.ClientDevice) []model.ClientDevice {
	if len(devices) <= 1 {
		if devices == nil {
			return []model.ClientDevice{}
		}
		return devices
	}

	result := make([]model.ClientDevice, 0, len(devices))
	for _, d := range devices {
		duplicate := false
		for _, existing := range result {
			if IsSameDevice(d, existing) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, d)
		}
	}
	return result
}

// RecordClientDevice parses device info from the subscription request and stores it.
func RecordClientDevice(c *gin.Context, subId string) {
	if subId == "" {
		return
	}

	info := ParseDeviceInfo(c)
	if info == nil {
		return
	}

	db := database.GetDB()
	if db == nil {
		return
	}

	now := time.Now().UnixMilli()

	// 1. Find NodeClient by SubID
	var nc model.NodeClient
	err := db.Where("sub_id = ?", subId).First(&nc).Error
	nodeClientId := 0
	clientEmail := ""
	if err == nil && nc.Id > 0 {
		nodeClientId = nc.Id
		clientEmail = nc.Email
		updates := map[string]any{
			"last_sub_time": now,
			"last_sub_ip":   info.IP,
		}
		if info.Model != "" {
			updates["device_model"] = info.Model
		}
		if info.HWID != "" {
			updates["device_hwid"] = info.HWID
		}
		if info.OS != "" {
			updates["last_os"] = info.OS
		}
		if info.ClientApp != "" {
			updates["last_client_app"] = info.ClientApp
		}
		if info.UserAgent != "" {
			updates["last_user_agent"] = info.UserAgent
		}
		_ = db.Model(&nc).Updates(updates).Error
	}

	// 2. Fetch all existing devices associated with this client / subId
	var existingDevices []model.ClientDevice
	if nodeClientId > 0 {
		_ = db.Where("node_client_id = ? OR (sub_id != '' AND sub_id = ?)", nodeClientId, subId).
			Order("last_seen desc").
			Find(&existingDevices).Error
	} else {
		_ = db.Where("sub_id = ?", subId).
			Order("last_seen desc").
			Find(&existingDevices).Error
	}

	// 3. Find matching device and any duplicate records to prune
	var matchedDev *model.ClientDevice
	var duplicateIds []int

	for i := range existingDevices {
		dev := &existingDevices[i]
		if MatchesDeviceInfo(*dev, info) {
			if matchedDev == nil {
				matchedDev = dev
			} else {
				// Old duplicate in DB, mark for deletion
				duplicateIds = append(duplicateIds, dev.Id)
			}
		}
	}

	// Clean up duplicate rows from DB if any exist
	if len(duplicateIds) > 0 {
		_ = db.Where("id IN ?", duplicateIds).Delete(&model.ClientDevice{}).Error
	}

	if matchedDev != nil {
		// Update existing device record
		devUpdates := map[string]any{
			"last_seen": now,
			"ip":        info.IP,
		}
		if nodeClientId > 0 && matchedDev.NodeClientId == 0 {
			devUpdates["node_client_id"] = nodeClientId
		}
		if clientEmail != "" && matchedDev.ClientEmail == "" {
			devUpdates["client_email"] = clientEmail
		}
		if subId != "" && matchedDev.SubID == "" {
			devUpdates["sub_id"] = subId
		}
		if info.HWID != "" {
			devUpdates["hwid"] = info.HWID
		}
		if info.Model != "" {
			devUpdates["model"] = info.Model
		}
		if info.OS != "" {
			devUpdates["os"] = info.OS
		}
		if info.ClientApp != "" {
			devUpdates["client_app"] = info.ClientApp
		}
		if info.UserAgent != "" {
			devUpdates["user_agent"] = info.UserAgent
		}
		_ = db.Model(matchedDev).Updates(devUpdates).Error
	} else {
		// Insert brand new device record
		newDev := model.ClientDevice{
			NodeClientId: nodeClientId,
			SubID:        subId,
			ClientEmail:  clientEmail,
			HWID:         info.HWID,
			Model:        info.Model,
			OS:           info.OS,
			ClientApp:    info.ClientApp,
			UserAgent:    info.UserAgent,
			IP:           info.IP,
			LastSeen:     now,
			CreatedAt:    now,
		}
		_ = db.Create(&newDev).Error
	}
}
