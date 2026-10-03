package service

import (
	"encoding/json"
	"errors"
	"runtime"
	"sync"

	"x-ui/internal/logger"
	"x-ui/internal/util/json_util"
	"x-ui/xray"

	"go.uber.org/atomic"
)

var (
	p                 *xray.Process
	lock              sync.Mutex
	isNeedXrayRestart atomic.Bool
	isManuallyStopped atomic.Bool
	result            string
)

type XrayService struct {
	inboundService     InboundService
	settingService     SettingService
	xraySettingService XraySettingService
	xrayAPI            xray.XrayAPI
}

func (s *XrayService) IsXrayRunning() bool {
	return p != nil && p.IsRunning()
}

func (s *XrayService) GetXrayErr() error {
	if p == nil {
		return nil
	}
	err := p.GetErr()
	if err == nil {
		return nil
	}
	if runtime.GOOS == "windows" && err.Error() == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return nil
	}

	return err
}

func (s *XrayService) GetXrayResult() string {
	if result != "" {
		return result
	}
	if s.IsXrayRunning() {
		return ""
	}
	if p == nil {
		return ""
	}
	result = p.GetResult()
	if runtime.GOOS == "windows" && result == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return ""
	}
	return result
}

func (s *XrayService) GetXrayVersion() string {
	if p == nil {
		return "Unknown"
	}
	return p.GetVersion()
}

func RemoveIndex(s []interface{}, index int) []interface{} {
	return append(s[:index], s[index+1:]...)
}

func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	err = json.Unmarshal([]byte(templateConfig), xrayConfig)
	if err != nil {
		return nil, err
	}
	xrayConfig.OutboundConfigs, err = normalizeLegacyOutbounds(xrayConfig.OutboundConfigs)
	if err != nil {
		return nil, err
	}

	err = s.xraySettingService.ensureLocalLogFile(xrayConfig, false)
	if err != nil {
		return nil, err
	}

	s.inboundService.AddTraffic(nil, nil)

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		inboundConfig, err := s.inboundService.BuildInboundConfig(inbound)
		if err != nil {
			logger.Warningf("GetXrayConfig: BuildInboundConfig failed for inbound %d: %v", inbound.Id, err)
			continue
		}
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

func normalizeLegacyVMessUser(user map[string]interface{}) {
	security, ok := user["security"].(string)
	if !ok {
		return
	}
	switch security {
	case "none", "zero", "plain", "":
		user["security"] = "auto"
	}
}

func normalizeLegacyOutbounds(raw json_util.RawMessage) (json_util.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}

	var outbounds []map[string]interface{}
	if err := json.Unmarshal(raw, &outbounds); err != nil {
		return nil, err
	}
	changed := false
	for _, outbound := range outbounds {
		protocol, _ := outbound["protocol"].(string)
		if protocol == "vmess" {
			settings, _ := outbound["settings"].(map[string]interface{})
			vnext, _ := settings["vnext"].([]interface{})
			for _, destinationValue := range vnext {
				destination, _ := destinationValue.(map[string]interface{})
				users, _ := destination["users"].([]interface{})
				for _, userValue := range users {
					user, _ := userValue.(map[string]interface{})
					before, _ := user["security"].(string)
					normalizeLegacyVMessUser(user)
					after, _ := user["security"].(string)
					changed = changed || before != after
				}
			}
		} else if protocol == "wireguard" {
			settings, ok := outbound["settings"].(map[string]interface{})
			if ok {
				if _, hasDS := settings["domainStrategy"]; hasDS {
					delete(settings, "domainStrategy")
					changed = true
				}
			}
		}
	}
	if !changed {
		return raw, nil
	}

	result, err := json.Marshal(outbounds)
	if err != nil {
		return nil, err
	}
	return json_util.RawMessage(result), nil
}

func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	if !s.IsXrayRunning() {
		err := errors.New("xray is not running")
		logger.Debug("Attempted to fetch Xray traffic, but Xray is not running:", err)
		return nil, nil, err
	}
	apiPort := p.GetAPIPort()
	s.xrayAPI.Init(apiPort)
	defer s.xrayAPI.Close()

	traffic, clientTraffic, err := s.xrayAPI.GetTraffic(true)
	if err != nil {
		logger.Debug("Failed to fetch Xray traffic:", err)
		return nil, nil, err
	}
	return traffic, clientTraffic, nil
}

func (s *XrayService) RestartXray(isForce bool) error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("restart xray, force:", isForce)
	isManuallyStopped.Store(false)

	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	if s.IsXrayRunning() {
		if !isForce && p.GetConfig().Equals(xrayConfig) && !isNeedXrayRestart.Load() {
			logger.Debug("It does not need to restart Xray")
			return nil
		}
		p.Stop()
	}

	p = xray.NewProcess(xrayConfig)
	result = ""
	err = p.Start()
	if err != nil {
		return err
	}
	return nil
}

func (s *XrayService) StopXray() error {
	lock.Lock()
	defer lock.Unlock()
	isManuallyStopped.Store(true)
	logger.Debug("Attempting to stop Xray...")
	if s.IsXrayRunning() {
		return p.Stop()
	}
	return errors.New("xray is not running")
}

func (s *XrayService) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CompareAndSwap(true, false)
}

func (s *XrayService) DidXrayCrash() bool {
	return !s.IsXrayRunning() && !isManuallyStopped.Load()
}
