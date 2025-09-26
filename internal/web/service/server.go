package service

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	stdnet "net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tcnksm/go-latest"
	"golang.org/x/crypto/ssh"

	"x-ui/config"
	"x-ui/internal/database"
	"x-ui/internal/logger"
	"x-ui/internal/util/common"
	"x-ui/internal/util/sys"
	"x-ui/xray"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

var (
	isOutdated    bool
	latestVersion string
	checkErr      error
)

type ProcessState string

const (
	Running ProcessState = "running"
	Stop    ProcessState = "stop"
	Error   ProcessState = "error"
)

type Status struct {
	T           time.Time `json:"-"`
	Cpu         float64   `json:"cpu"`
	CpuCores    int       `json:"cpuCores"`
	LogicalPro  int       `json:"logicalPro"`
	CpuSpeedMhz float64   `json:"cpuSpeedMhz"`
	Mem         struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"mem"`
	Swap struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"swap"`
	Disk struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"disk"`
	Xray struct {
		State    ProcessState `json:"state"`
		ErrorMsg string       `json:"errorMsg"`
		Version  string       `json:"version"`
	} `json:"xray"`
	Uptime   uint64    `json:"uptime"`
	HostName string    `json:"hostname"`
	Loads    []float64 `json:"loads"`
	TcpCount int       `json:"tcpCount"`
	UdpCount int       `json:"udpCount"`
	NetIO    struct {
		Up   uint64 `json:"up"`
		Down uint64 `json:"down"`
	} `json:"netIO"`
	NetTraffic struct {
		Sent uint64 `json:"sent"`
		Recv uint64 `json:"recv"`
	} `json:"netTraffic"`
	PublicIP struct {
		IPv4 string `json:"ipv4"`
		IPv6 string `json:"ipv6"`
	} `json:"publicIP"`
	AppStats struct {
		Threads      uint32 `json:"threads"`
		Mem          uint64 `json:"mem"`
		Uptime       uint64 `json:"uptime"`
		TXUpdate     string `json:"tx_update"`
		PanelVersion string `json:"panel_version"`
	} `json:"appStats"`
}

type Release struct {
	TagName string `json:"tag_name"`
}

type ServerService struct {
	xrayService    XrayService
	inboundService InboundService
}

type LogEntry struct {
	DateTime    time.Time
	FromAddress string
	ToAddress   string
	Inbound     string
	Outbound    string
	Email       string
	Event       int
}

func getPublicIP(url string) string {
	conn, err := stdnet.Dial("udp", url)
	if err != nil {
		return "N/A"
	}
	defer conn.Close()
	return conn.LocalAddr().(*stdnet.UDPAddr).IP.String()
}

func (s *ServerService) GetStatus(lastStatus *Status) *Status {
	now := time.Now()
	status := &Status{
		T: now,
	}

	percents, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warning("get cpu percent failed:", err)
	} else {
		status.Cpu = percents[0]
	}

	status.CpuCores, err = cpu.Counts(false)
	if err != nil {
		logger.Warning("get cpu cores count failed:", err)
	}

	status.LogicalPro = runtime.NumCPU()
	if p != nil && p.IsRunning() {
		status.AppStats.Uptime = p.GetUptime()
	} else {
		status.AppStats.Uptime = 0
	}

	cpuInfos, err := cpu.Info()
	if err != nil {
		logger.Warning("get cpu info failed:", err)
	} else if len(cpuInfos) > 0 {
		cpuInfo := cpuInfos[0]
		status.CpuSpeedMhz = cpuInfo.Mhz // setting CPU speed in MHz
	} else {
		logger.Warning("could not find cpu info")
	}

	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("get uptime failed:", err)
	} else {
		status.Uptime = upTime
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Warning("get virtual memory failed:", err)
	} else {
		status.Mem.Current = memInfo.Used
		status.Mem.Total = memInfo.Total
	}

	swapInfo, err := mem.SwapMemory()
	if err != nil {
		logger.Warning("get swap memory failed:", err)
	} else {
		status.Swap.Current = swapInfo.Used
		status.Swap.Total = swapInfo.Total
	}

	distInfo, err := disk.Usage("/")
	if err != nil {
		logger.Warning("get dist usage failed:", err)
	} else {
		status.Disk.Current = distInfo.Used
		status.Disk.Total = distInfo.Total
	}

	avgState, err := load.Avg()
	if err != nil {
		logger.Warning("get load avg failed:", err)
	} else {
		status.Loads = []float64{avgState.Load1, avgState.Load5, avgState.Load15}
	}

	ioStats, err := net.IOCounters(false)
	if err != nil {
		logger.Warning("get io counters failed:", err)
	} else if len(ioStats) > 0 {
		ioStat := ioStats[0]
		status.NetTraffic.Sent = ioStat.BytesSent
		status.NetTraffic.Recv = ioStat.BytesRecv

		if lastStatus != nil {
			duration := now.Sub(lastStatus.T)
			seconds := float64(duration) / float64(time.Second)
			up := uint64(float64(status.NetTraffic.Sent-lastStatus.NetTraffic.Sent) / seconds)
			down := uint64(float64(status.NetTraffic.Recv-lastStatus.NetTraffic.Recv) / seconds)
			status.NetIO.Up = up
			status.NetIO.Down = down
		}
	} else {
		logger.Warning("can not find io counters")
	}

	status.TcpCount, err = sys.GetTCPCount()
	if err != nil {
		logger.Warning("get tcp connections failed:", err)
	}

	status.UdpCount, err = sys.GetUDPCount()
	if err != nil {
		logger.Warning("get udp connections failed:", err)
	}

	status.PublicIP.IPv4 = getPublicIP("8.8.8.8:80")
	status.PublicIP.IPv6 = getPublicIP("[2001:4860:4860::8888]:80")

	status.HostName, _ = os.Hostname()

	if s.xrayService.IsXrayRunning() {
		status.Xray.State = Running
		status.Xray.ErrorMsg = ""
	} else {
		err := s.xrayService.GetXrayErr()
		if err != nil {
			status.Xray.State = Error
		} else {
			status.Xray.State = Stop
		}
		status.Xray.ErrorMsg = s.xrayService.GetXrayResult()
	}
	status.Xray.Version = s.xrayService.GetXrayVersion()
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)

	status.AppStats.Mem = rtm.Sys
	status.AppStats.Threads = uint32(runtime.NumGoroutine())
	if p != nil && p.IsRunning() {
		status.AppStats.Uptime = p.GetUptime()
	} else {
		status.AppStats.Uptime = 0
	}

	if isOutdated {
		status.AppStats.TXUpdate = latestVersion
		status.AppStats.PanelVersion = config.GetVersion()
	} else {
		status.AppStats.PanelVersion = config.GetVersion()
		status.AppStats.TXUpdate = latestVersion
	}
	return status
}

func (s *ServerService) CheckForUpdate(owner, repo, currentVersion string) (bool, string, error) {
	g := &latest.GithubTag{
		Owner:      owner,
		Repository: repo,
	}

	res, err := latest.Check(g, strings.TrimPrefix(currentVersion, "v"))
	if err != nil {
		checkErr = err
	}

	isOutdated = res.Outdated
	latestVersion = strings.TrimPrefix(res.Current, "v")
	if isOutdated {
		logger.Info("A new version is available: ", latestVersion)
	} else {
		logger.Info("You are using the latest version.")
	}

	return isOutdated, latestVersion, checkErr
}

func (s *ServerService) GetXrayVersions() ([]string, error) {
	const (
		XrayURL    = "https://api.github.com/repos/XTLS/Xray-core/releases"
		bufferSize = 8192
	)

	resp, err := http.Get(XrayURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buffer := bytes.NewBuffer(make([]byte, bufferSize))
	buffer.Reset()
	if _, err := buffer.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	var releases []Release
	if err := json.Unmarshal(buffer.Bytes(), &releases); err != nil {
		return nil, err
	}

	var versions []string
	for _, release := range releases {
		tagVersion := strings.TrimPrefix(release.TagName, "v")
		tagParts := strings.Split(tagVersion, ".")
		if len(tagParts) != 3 {
			continue
		}

		major, err1 := strconv.Atoi(tagParts[0])
		minor, err2 := strconv.Atoi(tagParts[1])
		patch, err3 := strconv.Atoi(tagParts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}

		if major > 25 || (major == 25 && minor > 9) || (major == 25 && minor == 9 && patch >= 11) {
			versions = append(versions, release.TagName)
		}
	}
	return versions, nil
}

func (s *ServerService) StopXrayService() (string error) {
	err := s.xrayService.StopXray()
	if err != nil {
		logger.Error("stop xray failed:", err)
		return err
	}

	return nil
}

func (s *ServerService) RestartXrayService() (string error) {
	defer func() {
		err := s.xrayService.RestartXray(true)
		if err != nil {
			logger.Error("start xray failed:", err)
		}
	}()

	return nil
}

func (s *ServerService) downloadXRay(version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		osName = "macos"
	}

	switch arch {
	case "amd64":
		arch = "64"
	case "arm64":
		arch = "arm64-v8a"
	case "armv7":
		arch = "arm32-v7a"
	case "armv6":
		arch = "arm32-v6"
	case "armv5":
		arch = "arm32-v5"
	case "386":
		arch = "32"
	case "s390x":
		arch = "s390x"
	}

	fileName := fmt.Sprintf("Xray-%s-%s.zip", osName, arch)
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", version, fileName)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	os.Remove(fileName)
	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func (s *ServerService) UpdateXray(version string) error {
	if err := s.StopXrayService(); err != nil {
		logger.Warning("failed to stop xray before update:", err)
	}

	zipFileName, err := s.downloadXRay(version)
	if err != nil {
		return err
	}
	defer os.Remove(zipFileName)

	zipFile, err := os.Open(zipFileName)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	stat, err := zipFile.Stat()
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(zipFile, stat.Size())
	if err != nil {
		return err
	}

	copyZipFile := func(zipName string, fileName string) error {
		zipFile, err := reader.Open(zipName)
		if err != nil {
			return err
		}
		defer zipFile.Close()
		os.MkdirAll(filepath.Dir(fileName), 0755)
		os.Remove(fileName)
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, fs.ModePerm)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, zipFile)
		return err
	}

	if runtime.GOOS == "windows" {
		targetBinary := filepath.Join("bin", "xray-windows-amd64.exe")
		err = copyZipFile("xray.exe", targetBinary)
	} else {
		err = copyZipFile("xray", xray.GetBinaryPath())
	}
	if err != nil {
		return err
	}
	if err := s.xrayService.RestartXray(true); err != nil {
		logger.Error("start xray failed:", err)
		return err
	}

	return nil
}

func (s *ServerService) UpdatePanel(version string) {
	fmt.Println("Starting x-ui installation...")

	// Detect system architecture
	archMap := map[string]string{
		"x86_64": "amd64", "x64": "amd64", "amd64": "amd64",
		"i386": "386", "i686": "386", "x86": "386",
		"armv8": "arm64", "arm64": "arm64", "aarch64": "arm64",
		"armv7": "armv7", "arm": "armv7",
		"armv6": "armv6", "armv5": "armv5", "s390x": "s390x",
	}

	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		fmt.Println("Error detecting architecture:", err)
		return
	}
	arch := strings.TrimSpace(string(out))
	if val, exists := archMap[arch]; exists {
		arch = val
	} else {
		fmt.Println("Unsupported CPU architecture!")
		return
	}

	// Get latest version if not provided
	if version == "" {
		logger.Info("Fetching latest version...")
		out, err := exec.Command("curl", "-Ls", "https://api.github.com/repos/AghayeCoder/tx-ui/releases/latest").Output()
		if err != nil {
			logger.Error("Failed to fetch latest version:", err)
			return
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "\"tag_name\":") {
				version = strings.Split(line, ":")[1]
				version = strings.Trim(version, " \"\t,")
				break
			}
		}
	}

	// Download x-ui
	url := fmt.Sprintf("https://github.com/AghayeCoder/tx-ui/releases/download/%s/x-ui-linux-%s.tar.gz", version, arch)
	filePath := fmt.Sprintf("/usr/local/x-ui-linux-%s.tar.gz", arch)
	logger.Info("Downloading:", url)
	err = exec.Command("wget", "-N", "--no-check-certificate", "-O", filePath, url).Run()
	if err != nil {
		logger.Error("Download failed:", err)
		return
	}

	os.RemoveAll("/usr/local/x-ui/")

	// Extract, configure, and install
	logger.Debug("Extracting...")
	exec.Command("tar", "-zxvf", filePath, "-C", "/usr/local/").Run()
	os.Remove(filePath)
	exec.Command("chmod", "+x", "/usr/local/x-ui/x-ui").Run()
	xrayLinux := fmt.Sprintf("/usr/local/x-ui/bin/xray-linux-%s", arch)
	exec.Command("chmod", "+x", "/usr/local/x-ui/x-ui", xrayLinux).Run()
	exec.Command("cp", "-f", "/usr/local/x-ui/x-ui.service", "/etc/systemd/system/").Run()
	exec.Command("wget", "--no-check-certificate", "-O", "/usr/bin/x-ui", "https://raw.githubusercontent.com/AghayeCoder/tx-ui/main/x-ui.sh").Run()
	exec.Command("chmod", "+x", "/usr/bin/x-ui").Run()
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "x-ui").Run()
	exec.Command("systemctl", "start", "x-ui").Run()
	exec.Command("x-ui", "restart").Run()
	logger.Infof("x-ui %s installation finished and is now running!", version)
}

func (s *ServerService) ApplyTunnel(ip string, port string, username string, password string) {
	exec.Command("wget", "--no-check-certificate", "https://raw.githubusercontent.com/AghayeCoder/6to4/master/sender.py").Run()
	exec.Command("python3", "sender.py", ip).Output()
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", ip+":"+port, config)
	if err != nil {
		logger.Errorf("Failed to dial: %s", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		logger.Errorf("Failed to create session: %s", err)
	}
	defer session.Close()
	session.CombinedOutput("wget https://raw.githubusercontent.com/AghayeCoder/6to4/master/receiver.py && python3 receiver.py " + getPublicIP("8.8.8.8:80"))
}

func (s *ServerService) GetLogs(count string, level string, syslog string) []string {
	c, _ := strconv.Atoi(count)
	var lines []string

	if syslog == "true" {
		cmdArgs := []string{"journalctl", "-u", "x-ui", "--no-pager", "-n", count, "-p", level}
		// Run the command
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			return []string{"Failed to run journalctl command!"}
		}
		lines = strings.Split(out.String(), "\n")
	} else {
		lines = logger.GetLogs(c, level)
	}

	return lines
}

func (s *ServerService) GetXrayLogs(
	count string,
	filter string,
	showDirect string,
	showBlocked string,
	showProxy string,
	freedoms []string,
	blackholes []string) []LogEntry {

	const (
		Direct = iota
		Blocked
		Proxied
	)

	countInt, _ := strconv.Atoi(count)
	var entries []LogEntry

	pathToAccessLog, err := xray.GetAccessLogPath()
	if err != nil {
		return nil
	}

	file, err := os.Open(pathToAccessLog)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.Contains(line, "api -> api") {
			continue
		}
		if filter != "" && !strings.Contains(line, filter) {
			//applying filter if it's not empty
			continue
		}

		var entry LogEntry
		parts := strings.Fields(line)

		for i, part := range parts {

			if i == 0 {
				dateTime, err := time.Parse("2006/01/02 15:04:05.999999", parts[0]+" "+parts[1])
				if err != nil {
					continue
				}
				entry.DateTime = dateTime
			}

			if part == "from" {
				entry.FromAddress = parts[i+1]
			} else if part == "accepted" {
				entry.ToAddress = parts[i+1]
			} else if strings.HasPrefix(part, "[") {
				entry.Inbound = part[1:]
			} else if strings.HasSuffix(part, "]") {
				entry.Outbound = part[:len(part)-1]
			} else if part == "email:" {
				entry.Email = parts[i+1]
			}
		}

		if logEntryContains(line, freedoms) {
			if showDirect == "false" {
				continue
			}
			entry.Event = Direct
		} else if logEntryContains(line, blackholes) {
			if showBlocked == "false" {
				continue
			}
			entry.Event = Blocked
		} else {
			if showProxy == "false" {
				continue
			}
			entry.Event = Proxied
		}

		entries = append(entries, entry)
	}

	if len(entries) > countInt {
		entries = entries[len(entries)-countInt:]
	}

	return entries
}

func logEntryContains(line string, suffixes []string) bool {
	for _, sfx := range suffixes {
		if strings.Contains(line, sfx+"]") {
			return true
		}
	}
	return false
}

func (s *ServerService) GetConfigJson() (interface{}, error) {
	config, err := s.xrayService.GetXrayConfig()
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}

	var jsonData interface{}
	err = json.Unmarshal(contents, &jsonData)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}

func (s *ServerService) GetDb() ([]byte, error) {
	// Update by manually trigger a checkpoint operation
	err := database.Checkpoint()
	if err != nil {
		return nil, err
	}
	// Open the file for reading
	file, err := os.Open(config.GetDBPath())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file contents
	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return fileContents, nil
}

func (s *ServerService) ImportDB(file multipart.File) error {
	// Check if the file is a SQLite database
	isValidDb, err := database.IsSQLiteDB(file)
	if err != nil {
		return common.NewErrorf("Error checking db file format: %v", err)
	}
	if !isValidDb {
		return common.NewError("Invalid db file format")
	}

	// Reset the file reader to the beginning
	_, err = file.Seek(0, 0)
	if err != nil {
		return common.NewErrorf("Error resetting file reader: %v", err)
	}

	// Save the file as temporary file
	tempPath := fmt.Sprintf("%s.temp", config.GetDBPath())
	// Remove the existing fallback file (if any) before creating one
	_, err = os.Stat(tempPath)
	if err == nil {
		errRemove := os.Remove(tempPath)
		if errRemove != nil {
			return common.NewErrorf("Error removing existing temporary db file: %v", errRemove)
		}
	}
	// Create the temporary file
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return common.NewErrorf("Error creating temporary db file: %v", err)
	}
	defer tempFile.Close()

	// Remove temp file before returning
	defer os.Remove(tempPath)

	// Save uploaded file to temporary file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		return common.NewErrorf("Error saving db: %v", err)
	}

	// Check if we can init db or not
	err = database.InitDB(tempPath)
	if err != nil {
		return common.NewErrorf("Error checking db: %v", err)
	}

	// Stop Xray
	s.StopXrayService()

	// Backup the current database for fallback
	fallbackPath := fmt.Sprintf("%s.backup", config.GetDBPath())
	// Remove the existing fallback file (if any)
	_, err = os.Stat(fallbackPath)
	if err == nil {
		errRemove := os.Remove(fallbackPath)
		if errRemove != nil {
			return common.NewErrorf("Error removing existing fallback db file: %v", errRemove)
		}
	}
	// Move the current database to the fallback location
	err = os.Rename(config.GetDBPath(), fallbackPath)
	if err != nil {
		return common.NewErrorf("Error backing up temporary db file: %v", err)
	}

	// Remove the temporary file before returning
	defer os.Remove(fallbackPath)

	// Move temp to DB path
	err = os.Rename(tempPath, config.GetDBPath())
	if err != nil {
		errRename := os.Rename(fallbackPath, config.GetDBPath())
		if errRename != nil {
			return common.NewErrorf("Error moving db file and restoring fallback: %v", errRename)
		}
		return common.NewErrorf("Error moving db file: %v", err)
	}

	// Migrate DB
	err = database.InitDB(config.GetDBPath())
	if err != nil {
		errRename := os.Rename(fallbackPath, config.GetDBPath())
		if errRename != nil {
			return common.NewErrorf("Error migrating db and restoring fallback: %v", errRename)
		}
		return common.NewErrorf("Error migrating db: %v", err)
	}
	s.inboundService.MigrateDB()

	// Start Xray
	err = s.RestartXrayService()
	if err != nil {
		return common.NewErrorf("Imported DB but Failed to start Xray: %v", err)
	}

	return nil
}

func (s *ServerService) GetNewX25519Cert() (interface{}, error) {
	// Run the command
	cmd := exec.Command(xray.GetBinaryPath(), "x25519")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	privateKeyLine := strings.Split(lines[0], ":")
	publicKeyLine := strings.Split(lines[1], ":")

	privateKey := strings.TrimSpace(privateKeyLine[1])
	publicKey := strings.TrimSpace(publicKeyLine[1])

	keyPair := map[string]interface{}{
		"privateKey": privateKey,
		"publicKey":  publicKey,
	}

	return keyPair, nil
}

func (s *ServerService) GetNewmldsa65() (any, error) {
	cmd := exec.Command(xray.GetBinaryPath(), "mldsa65")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	SeedLine := strings.Split(lines[0], ":")
	VerifyLine := strings.Split(lines[1], ":")

	seed := strings.TrimSpace(SeedLine[1])
	verify := strings.TrimSpace(VerifyLine[1])

	keyPair := map[string]any{
		"seed":   seed,
		"verify": verify,
	}

	return keyPair, nil
}
func (s *ServerService) GetNewEchCert(sni string) (interface{}, error) {
	// Run the command
	cmd := exec.Command(xray.GetBinaryPath(), "tls", "ech", "--serverName", sni)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")
	if len(lines) < 4 {
		return nil, common.NewError("invalid ech cert")
	}

	configList := lines[1]
	serverKeys := lines[3]

	return map[string]interface{}{
		"echServerKeys": serverKeys,
		"echConfigList": configList,
	}, nil
}

func (s *ServerService) GetNewVlessEnc() (any, error) {
	cmd := exec.Command(xray.GetBinaryPath(), "vlessenc")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	var auths []map[string]string
	var current map[string]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Authentication:") {
			if current != nil {
				auths = append(auths, current)
			}
			current = map[string]string{
				"label": strings.TrimSpace(strings.TrimPrefix(line, "Authentication:")),
			}
		} else if strings.HasPrefix(line, `"decryption"`) || strings.HasPrefix(line, `"encryption"`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && current != nil {
				key := strings.Trim(parts[0], `" `)
				val := strings.Trim(parts[1], `" `)
				current[key] = val
			}
		}
	}

	if current != nil {
		auths = append(auths, current)
	}

	return map[string]any{
		"auths": auths,
	}, nil
}

func (s *ServerService) GetNewUUID() (map[string]string, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	return map[string]string{
		"uuid": newUUID.String(),
	}, nil
}

func (s *ServerService) GetNewmlkem768() (any, error) {
	// Run the command
	cmd := exec.Command(xray.GetBinaryPath(), "mlkem768")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	SeedLine := strings.Split(lines[0], ":")
	ClientLine := strings.Split(lines[1], ":")

	seed := strings.TrimSpace(SeedLine[1])
	client := strings.TrimSpace(ClientLine[1])

	keyPair := map[string]any{
		"seed":   seed,
		"client": client,
	}

	return keyPair, nil
}
