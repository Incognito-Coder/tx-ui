package job

import (
	"strings"
	"time"

	"x-ui/internal/database"
	"x-ui/internal/logger"
	"x-ui/internal/web/service"
	"x-ui/xray"
)

var autoDeleteDepletedClientsJob *AutoDeleteDepletedClientsJob

type AutoDeleteDepletedClientsJob struct {
	settingService *service.SettingService
	inboundService *service.InboundService
}

func NewAutoDeleteDepletedClientsJob() *AutoDeleteDepletedClientsJob {
	return &AutoDeleteDepletedClientsJob{
		settingService: &service.SettingService{},
		inboundService: &service.InboundService{},
	}
}

func (j *AutoDeleteDepletedClientsJob) Run() {
	logger.Debug("AutoDeleteDepletedClientsJob started")
	defer logger.Debug("AutoDeleteDepletedClientsJob finished")

	autoDeleteDay, err := j.settingService.GetAutoDeleteDay()
	if err != nil {
		logger.Warning("failed to get auto delete day setting:", err)
		return
	}
	if autoDeleteDay == 0 {
		return
	}

	db := database.GetDB()
	traffics := make([]*xray.ClientTraffic, 0)

	// Get all depleted clients
	err = db.Model(&xray.ClientTraffic{}).
		Where("enable = ? AND reset = ?", false, 0).
		Find(&traffics).Error
	if err != nil {
		logger.Warning("failed to get depleted clients:", err)
		return
	}

	if len(traffics) > 0 {
		emails := make([]string, len(traffics))
		for i, t := range traffics {
			emails[i] = t.Email
		}
		logger.Info("found depleted clients to check for deletion:", strings.Join(emails, ", "))
	}

	expiryLimit := time.Now().Unix()*1000 - int64(autoDeleteDay)*24*60*60*1000

	for _, traffic := range traffics {
		if traffic.ExpiryTime > 0 && traffic.ExpiryTime < expiryLimit {
			inbound, err := j.inboundService.GetInbound(traffic.InboundId)
			if err != nil {
				logger.Warningf("failed to get inbound %v: %v", traffic.InboundId, err)
				continue
			}
			_, client, err := j.inboundService.GetClientByEmail(traffic.Email)
			if err != nil {
				logger.Warningf("failed to get client %v: %v", traffic.Email, err)
				continue
			}

			clientId := client.ID
			if inbound.Protocol == "trojan" {
				clientId = client.Password
			} else if inbound.Protocol == "shadowsocks" {
				clientId = client.Email
			}

			_, err = j.inboundService.DelInboundClient(traffic.InboundId, clientId)
			if err != nil {
				logger.Warningf("failed to delete client %v from inbound %v: %v", traffic.Email, traffic.InboundId, err)
			} else {
				logger.Infof("client %v from inbound %v deleted successfully", traffic.Email, traffic.InboundId)
			}
		}
	}
}
