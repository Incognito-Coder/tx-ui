package job

import (
	"x-ui/internal/logger"
	"x-ui/internal/web/service"
)

type CheckXrayRunningJob struct {
	xrayService service.XrayService

	checkTime int
}

func NewCheckXrayRunningJob() *CheckXrayRunningJob {
	return new(CheckXrayRunningJob)
}

func (j *CheckXrayRunningJob) Run() {
	if !j.xrayService.DidXrayCrash() {
		j.checkTime = 0
	} else {
		j.checkTime++
		// only restart if it's down 2 times in a row
		if j.checkTime > 1 {
			err := j.xrayService.RestartXray(false)
			j.checkTime = 0
			if err != nil {
				logger.Error("Restart xray failed:", err)
				if j.xrayService.HasLastWorkingConfig() && !j.xrayService.IsXrayRunning() {
					logger.Warning("Core crashed and restart failed. Rolling back to last working config...")
					if rbErr := j.xrayService.RollbackToLastWorkingConfig(); rbErr != nil {
						logger.Errorf("Automatic rollback to last working config failed: %v", rbErr)
					}
				}
			}
		}
	}
}
