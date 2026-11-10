package job

import (
	"strconv"
	"sync"
	"time"

	"x-ui/internal/web/service"

	"github.com/shirou/gopsutil/v4/cpu"
)

var (
	lastCpuAlertTime time.Time
	cpuAlertLock     sync.Mutex
)

type CheckCpuJob struct {
	tgbotService   service.Tgbot
	settingService service.SettingService
}

func NewCheckCpuJob() *CheckCpuJob {
	return new(CheckCpuJob)
}

// Here run is a interface method of Job interface
func (j *CheckCpuJob) Run() {
	tgBotEnabled, err := j.settingService.GetTgbotEnabled()
	if err != nil || !tgBotEnabled {
		return
	}

	cpuNotify, err := j.settingService.GetTgBotCpuNotify()
	if err == nil && !cpuNotify {
		return
	}

	threshold, err := j.settingService.GetTgCpu()
	if err != nil || threshold <= 0 {
		return
	}

	// get latest status of server (sample over 1s)
	percent, err := cpu.Percent(time.Second, false)
	if err == nil && len(percent) > 0 && percent[0] > float64(threshold) {
		cpuAlertLock.Lock()
		if !lastCpuAlertTime.IsZero() && time.Since(lastCpuAlertTime) < 10*time.Minute {
			cpuAlertLock.Unlock()
			return
		}
		lastCpuAlertTime = time.Now()
		cpuAlertLock.Unlock()

		msg := j.tgbotService.I18nBot("tgbot.messages.cpuThreshold",
			"Percent=="+strconv.FormatFloat(percent[0], 'f', 2, 64),
			"Threshold=="+strconv.Itoa(threshold))

		j.tgbotService.SendMsgToTgbotAdmins(msg)
	}
}
