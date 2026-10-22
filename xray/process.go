package xray

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"x-ui/config"
	"x-ui/internal/logger"
	"x-ui/internal/util/common"
)

func GetBinaryName() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("xray-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

func GetBinaryPath() string {
	basePath := config.GetBinFolderPath() + "/" + fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(basePath + ".exe"); err == nil {
			return basePath + ".exe"
		}
		if _, err := os.Stat(basePath); err == nil {
			return basePath
		}
		return basePath + ".exe"
	}
	return basePath
}

func GetConfigPath() string {
	return config.GetBinFolderPath() + "/config.json"
}

func GetLastWorkingConfigPath() string {
	return config.GetBinFolderPath() + "/config.json.last_working"
}

func GetLastFailedConfigPath() string {
	return config.GetBinFolderPath() + "/config.json.failed"
}

func HasLastWorkingConfig() bool {
	info, err := os.Stat(GetLastWorkingConfigPath())
	return err == nil && info.Size() > 0
}

func BackupLastWorkingConfig() error {
	src := GetConfigPath()
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("cannot backup empty config")
	}
	return os.WriteFile(GetLastWorkingConfigPath(), data, fs.ModePerm)
}

func SaveLastFailedConfig(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return os.WriteFile(GetLastFailedConfigPath(), data, fs.ModePerm)
}

func RollbackToLastWorkingConfig() error {
	src := GetLastWorkingConfigPath()
	data, err := os.ReadFile(src)
	if err != nil {
		return common.NewErrorf("cannot read last working config: %v", err)
	}
	if len(data) == 0 {
		return errors.New("last working config is empty")
	}
	dst := GetConfigPath()
	return os.WriteFile(dst, data, fs.ModePerm)
}

func TestConfigFile(configPath string) (bool, string, error) {
	cmd := exec.Command(GetBinaryPath(), "-test", "-c", configPath)
	out, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(out))
	if err != nil {
		return false, outputStr, err
	}
	return true, outputStr, nil
}

func GetGeositePath() string {
	return config.GetBinFolderPath() + "/geosite.dat"
}

func GetGeoipPath() string {
	return config.GetBinFolderPath() + "/geoip.dat"
}

func GetIPLimitLogPath() string {
	return config.GetLogFolder() + "/3xipl.log"
}

func GetIPLimitBannedLogPath() string {
	return config.GetLogFolder() + "/3xipl-banned.log"
}

func GetIPLimitBannedPrevLogPath() string {
	return config.GetLogFolder() + "/3xipl-banned.prev.log"
}

func GetAccessPersistentLogPath() string {
	return config.GetLogFolder() + "/3xipl-ap.log"
}

func GetAccessPersistentPrevLogPath() string {
	return config.GetLogFolder() + "/3xipl-ap.prev.log"
}

func GetAccessLogPath() (string, error) {
	config, err := os.ReadFile(GetConfigPath())
	if err != nil {
		logger.Warningf("Failed to read configuration file: %s", err)
		return "", err
	}

	jsonConfig := map[string]interface{}{}
	err = json.Unmarshal([]byte(config), &jsonConfig)
	if err != nil {
		logger.Warningf("Failed to parse JSON configuration: %s", err)
		return "", err
	}

	if jsonLog, ok := jsonConfig["log"].(map[string]interface{}); ok && jsonLog != nil {
		if accessLogPath, ok := jsonLog["access"].(string); ok {
			return accessLogPath, nil
		}
	}
	return "", err
}

func stopProcess(p *Process) {
	p.Stop()
}

type Process struct {
	*process
}

func NewProcess(xrayConfig *Config) *Process {
	p := &Process{newProcess(xrayConfig)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

// NewTestProcess creates a new Xray process that uses a specific config file path.
// Used for test runs (e.g. outbound test) so the main config.json is not overwritten.
// The config file at configPath is removed when the process is stopped.
func NewTestProcess(xrayConfig *Config, configPath string) *Process {
	p := &Process{newTestProcess(xrayConfig, configPath)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

// NewProcessFromConfigFile creates an Xray process that starts directly from an existing config file on disk.
func NewProcessFromConfigFile(configPath string) *Process {
	p := &Process{newProcess(nil)}
	p.configPath = configPath
	runtime.SetFinalizer(p, stopProcess)
	return p
}

type process struct {
	cmd *exec.Cmd

	version string
	apiPort int

	onlineClients []string

	config     *Config
	configPath string
	logWriter  *LogWriter
	exitErr    error
	startTime  time.Time
}

func newProcess(config *Config) *process {
	return &process{
		version:   "Unknown",
		config:    config,
		logWriter: NewLogWriter(),
		startTime: time.Now(),
	}
}

// newTestProcess creates a process that writes and runs with a specific config path.
func newTestProcess(config *Config, configPath string) *process {
	p := newProcess(config)
	p.configPath = configPath
	return p
}

func (p *process) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	if p.cmd.ProcessState == nil {
		return true
	}
	return false
}

func (p *process) GetErr() error {
	return p.exitErr
}

func (p *process) GetResult() string {
	if len(p.logWriter.lastLine) == 0 && p.exitErr != nil {
		return p.exitErr.Error()
	}
	return p.logWriter.lastLine
}

func (p *process) GetVersion() string {
	return p.version
}

func (p *Process) GetAPIPort() int {
	return p.apiPort
}

func (p *Process) GetConfig() *Config {
	return p.config
}

func (p *Process) GetOnlineClients() []string {
	return p.onlineClients
}

func (p *Process) SetOnlineClients(users []string) {
	p.onlineClients = users
}

func (p *Process) GetUptime() uint64 {
	return uint64(time.Since(p.startTime).Seconds())
}

func (p *process) refreshAPIPort() {
	for _, inbound := range p.config.InboundConfigs {
		if inbound.Tag == "api" {
			p.apiPort = inbound.Port
			break
		}
	}
}

func (p *process) refreshVersion() {
	cmd := exec.Command(GetBinaryPath(), "-version")
	data, err := cmd.Output()
	if err != nil {
		p.version = "Unknown"
	} else {
		datas := bytes.Split(data, []byte(" "))
		if len(datas) <= 1 {
			p.version = "Unknown"
		} else {
			p.version = string(datas[1])
		}
	}
}

func (p *process) Start() (err error) {
	if p.IsRunning() {
		return errors.New("xray is already running")
	}

	defer func() {
		if err != nil {
			// On Windows, killing the process results in "exit status 1" which isn't an error for us
			if runtime.GOOS == "windows" {
				errStr := strings.ToLower(err.Error())
				if strings.Contains(errStr, "exit status 1") {
					// Suppress noisy log on graceful stop
					p.exitErr = err
					return
				}
			}
			logger.Error("Failure in running xray-core process: ", err)
			p.exitErr = err
		}
	}()

	err = os.MkdirAll(config.GetLogFolder(), 0o770)
	if err != nil {
		logger.Warningf("Failed to create log folder: %s", err)
	}

	configPath := GetConfigPath()
	if p.configPath != "" {
		configPath = p.configPath
	}

	isMainInstance := (p.configPath == "" || p.configPath == GetConfigPath())

	var data []byte
	if p.config != nil {
		data, err = json.MarshalIndent(p.config, "", "  ")
		if err != nil {
			return common.NewErrorf("Failed to generate XRAY configuration files: %v", err)
		}
	} else {
		data, err = os.ReadFile(configPath)
		if err != nil {
			return common.NewErrorf("Failed to read configuration file: %v", err)
		}
		cfg := &Config{}
		if uErr := json.Unmarshal(data, cfg); uErr == nil {
			p.config = cfg
		}
	}

	// For main Xray instance, pre-flight test new config before applying
	if isMainInstance && p.config != nil {
		testConfigPath := config.GetBinFolderPath() + "/config_test.json"
		if writeErr := os.WriteFile(testConfigPath, data, fs.ModePerm); writeErr == nil {
			defer os.Remove(testConfigPath)
			ok, testOut, testErr := TestConfigFile(testConfigPath)
			if !ok {
				_ = SaveLastFailedConfig(data)
				logger.Errorf("Xray config pre-test failed: %v\nOutput: %s", testErr, testOut)
				errMsg := testOut
				if errMsg == "" && testErr != nil {
					errMsg = testErr.Error()
				}
				return common.NewErrorf("Config test failed: %s", errMsg)
			}
		}
	}

	// If main instance and a valid working config already exists on disk, backup before overwriting
	if isMainInstance {
		if curData, readErr := os.ReadFile(configPath); readErr == nil && len(curData) > 0 {
			if !HasLastWorkingConfig() {
				_ = BackupLastWorkingConfig()
			}
		}
	}

	if p.config != nil {
		err = os.WriteFile(configPath, data, fs.ModePerm)
		if err != nil {
			return common.NewErrorf("Failed to write configuration file: %v", err)
		}
	}

	cmd := exec.Command(GetBinaryPath(), "-c", configPath)
	p.cmd = cmd

	cmd.Stdout = p.logWriter
	cmd.Stderr = p.logWriter

	err = cmd.Start()
	if err != nil {
		if isMainInstance {
			_ = SaveLastFailedConfig(data)
		}
		return common.NewErrorf("Failed to execute xray-core binary: %v", err)
	}

	exitChan := make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		if waitErr != nil {
			if !(runtime.GOOS == "windows" && strings.Contains(strings.ToLower(waitErr.Error()), "exit status 1")) {
				logger.Error("Failure in running xray-core:", waitErr)
				p.exitErr = waitErr
			}
		}
		exitChan <- waitErr
	}()

	// Brief health-check window (400ms) to detect immediate core exit on bad config
	select {
	case waitErr := <-exitChan:
		if waitErr != nil && !(runtime.GOOS == "windows" && strings.Contains(strings.ToLower(waitErr.Error()), "exit status 1")) {
			if isMainInstance {
				_ = SaveLastFailedConfig(data)
			}
			lastLog := p.GetResult()
			return common.NewErrorf("Xray core exited with error: %v (last log: %s)", waitErr, lastLog)
		}
	case <-time.After(400 * time.Millisecond):
		// Core is still running; backup this working state
		if isMainInstance {
			_ = BackupLastWorkingConfig()
		}
	}

	p.refreshVersion()
	p.refreshAPIPort()

	return nil
}

func (p *process) Stop() error {
	if !p.IsRunning() {
		return errors.New("xray is not running")
	}
	if p.configPath != "" {
		if p.configPath != GetConfigPath() {
			// Check if file exists before removing
			if _, err := os.Stat(p.configPath); err == nil {
				_ = os.Remove(p.configPath)
			}
		}
	}
	if runtime.GOOS == "windows" {
		return p.cmd.Process.Kill()
	} else {
		return p.cmd.Process.Signal(syscall.SIGTERM)
	}
}

func writeCrashReport(m []byte) error {
	crashReportPath := config.GetBinFolderPath() + "/core_crash_" + time.Now().Format("20060102_150405") + ".log"
	return os.WriteFile(crashReportPath, m, os.ModePerm)
}
