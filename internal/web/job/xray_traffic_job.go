package job

import (
	"x-ui/internal/logger"
	"x-ui/internal/web/service"
)

type XrayTrafficJob struct {
	xrayService     service.XrayService
	inboundService  service.InboundService
	outboundService service.OutboundService
}

func NewXrayTrafficJob() *XrayTrafficJob {
	return new(XrayTrafficJob)
}

func (j *XrayTrafficJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}
	traffics, clientTraffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		return
	}
	err, needRestart0 := j.inboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add inbound traffic failed:", err)
	}
	err, needRestart1 := j.outboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add outbound traffic failed:", err)
	}
	if needRestart0 || needRestart1 {
		// Expired/exhausted clients are removed from Xray in-memory via the
		// AlterInbound RemoveUser API (zero-downtime). A full core restart is
		// only scheduled here as a fallback for protocols that do not support
		// hot user removal (e.g. WireGuard) or when the API call failed.
		logger.Info("Traffic check detected client change; scheduling graceful restart if needed")
		j.xrayService.SetToNeedRestart()
	}
}
