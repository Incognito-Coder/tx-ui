package service

import (
	_ "embed"
	"encoding/json"
	"strings"

	"x-ui/internal/logger"
	"x-ui/internal/util/common"
	"x-ui/internal/util/json_util"
	"x-ui/xray"
)

type XraySettingService struct {
	SettingService
}

func (s *XraySettingService) SaveXraySetting(newXraySettings string) error {
	xrayConfig := &xray.Config{}
	err := json.Unmarshal([]byte(newXraySettings), xrayConfig)
	if err != nil {
		return common.NewError("xray template config invalid:", err)
	}
	return s.ensureLocalLogFile(xrayConfig, true)
}

func (s *XraySettingService) ensureLocalLogFile(xrayConfig *xray.Config, alwaysSave bool) error {
	if len(xrayConfig.LogConfig) == 0 {
		return nil
	}
	var logConfig map[string]interface{}
	if err := json.Unmarshal(xrayConfig.LogConfig, &logConfig); err != nil {
		return err
	}

	changed := false
	for _, key := range []string{"access", "error"} {
		val, ok := logConfig[key].(string)
		if !ok || val == "" {
			continue
		}
		if sanitized, wasChanged := sanitizeLogPath(val); wasChanged {
			logConfig[key] = sanitized
			changed = true
		}
	}

	if !changed && !alwaysSave {
		return nil
	}

	if changed {
		newLogConfig, err := json.Marshal(logConfig)
		if err != nil {
			return err
		}
		xrayConfig.LogConfig = json_util.RawMessage(newLogConfig)
	}

	updatedTemplate, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		logger.Warning("ensureLocalLogFile: failed to marshal template:", err)
		return err
	}
	return s.SettingService.saveSetting("xrayTemplateConfig", string(updatedTemplate))
}

func sanitizeLogPath(p string) (string, bool) {
	check := strings.TrimPrefix(p, "./")
	if !strings.Contains(check, "/") {
		return p, false
	}
	idx := strings.LastIndex(p, "/")
	return "./" + p[idx+1:], true
}
