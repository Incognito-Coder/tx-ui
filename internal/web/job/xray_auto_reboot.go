package job

import (
	"x-ui/internal/logger"
	"x-ui/internal/web/service"
)

type XrayRebootJob struct {
	xrayService service.XrayService
}

func NewXrayRebootJob() *XrayRebootJob {
	return new(XrayRebootJob)
}

func (j *XrayRebootJob) Run() {
	err := j.xrayService.RestartXray(true)
	if err != nil {
		logger.Error("Restart xray failed:", err)
	} else {
		logger.Info("Restart xray success")
	}
}
