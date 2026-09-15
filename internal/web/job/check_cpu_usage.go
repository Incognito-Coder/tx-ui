package job

import (
	"strconv"
	"time"

	"x-ui/internal/web/service"

	"github.com/shirou/gopsutil/v4/cpu"
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
	threshold, _ := j.settingService.GetTgCpu()

	// get latest status of server (sample over 1s)
	percent, err := cpu.Percent(time.Second, false)
	if err == nil && len(percent) > 0 && percent[0] > float64(threshold) {
		msg := j.tgbotService.I18nBot("tgbot.messages.cpuThreshold",
			"Percent=="+strconv.FormatFloat(percent[0], 'f', 2, 64),
			"Threshold=="+strconv.Itoa(threshold))

		j.tgbotService.SendMsgToTgbotAdmins(msg)
	}
}
