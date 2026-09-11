package global

import (
	"context"
	_ "unsafe"

	"github.com/robfig/cron/v3"
)

var (
	webServer         WebServer
	subServer         SubServer
	configLinkService ConfigLinkService
	RestartChan       = make(chan struct{}, 1)
)

func TriggerRestart() {
	select {
	case RestartChan <- struct{}{}:
	default:
	}
}

type WebServer interface {
	GetCron() *cron.Cron
	GetCtx() context.Context
}

type SubServer interface {
	GetCtx() context.Context
	GetConfigLinksByEmail(email string) (string, []string, error)
}

type ConfigLinkService interface {
	GetConfigLinksByEmail(email string) (string, []string, error)
}

func SetWebServer(s WebServer) {
	webServer = s
}

func GetWebServer() WebServer {
	return webServer
}

func SetSubServer(s SubServer) {
	subServer = s
}

func GetSubServer() SubServer {
	return subServer
}

func SetConfigLinkService(s ConfigLinkService) {
	configLinkService = s
}

func GetConfigLinkService() ConfigLinkService {
	return configLinkService
}

