package service

import (
	"os"
	"runtime"
	"syscall"
	"time"

	"x-ui/internal/logger"
	"x-ui/internal/web/global"
)

type PanelService struct{}

func (s *PanelService) RestartPanel(delay time.Duration) error {
	go func() {
		time.Sleep(delay)
		if runtime.GOOS == "windows" {
			global.TriggerRestart()
		} else {
			p, err := os.FindProcess(syscall.Getpid())
			if err != nil {
				global.TriggerRestart()
				return
			}
			err = p.Signal(syscall.SIGHUP)
			if err != nil {
				logger.Debug("failed to send SIGHUP signal, triggering internal restart:", err)
				global.TriggerRestart()
			}
		}
	}()
	return nil
}
