package job

import (
	"x-ui/internal/logger"
	"x-ui/internal/mtproto"
	"x-ui/internal/web/service"
	"x-ui/xray"
)

// MtprotoJob reconciles the running mtg sidecar processes against the enabled
// mtproto inbounds in the database, restarts any that crashed, and folds the
// per-client traffic scraped from each mtg /stats endpoint into the usual client
// and inbound traffic accounting.
type MtprotoJob struct {
	inboundService service.InboundService
}

// NewMtprotoJob creates a new mtproto reconcile/traffic job instance.
func NewMtprotoJob() *MtprotoJob {
	return new(MtprotoJob)
}

// Run reconciles desired mtproto inbounds with running mtg processes and records
// per-client traffic deltas and online status.
func (j *MtprotoJob) Run() {
	desired, err := j.inboundService.DesiredMtprotoInstances()
	if err != nil {
		logger.Warning("mtproto job: get desired instances failed:", err)
		return
	}

	routedTags := make(map[string]bool)
	for _, inst := range desired {
		if inst.RouteThroughXray {
			routedTags[inst.Tag] = true
		}
	}

	mgr := mtproto.GetManager()
	mgr.Reconcile(desired)

	deltas, _ := mgr.CollectTraffic()

	// A routed inbound's total is already metered through the Xray bridge by
	// xray_traffic_job, so only non-routed inbounds are rolled up here; per-client
	// deltas are always kept, since the bridge cannot tell mtproto users apart.
	clientTraffics := make([]*xray.ClientTraffic, 0, len(deltas))
	inboundUp := make(map[string]int64)
	inboundDown := make(map[string]int64)
	for _, d := range deltas {
		clientTraffics = append(clientTraffics, &xray.ClientTraffic{
			Email: d.Email,
			Up:    d.Up,
			Down:  d.Down,
		})
		if !routedTags[d.Tag] {
			inboundUp[d.Tag] += d.Up
			inboundDown[d.Tag] += d.Down
		}
	}

	traffics := make([]*xray.Traffic, 0, len(inboundUp))
	for tag, up := range inboundUp {
		traffics = append(traffics, &xray.Traffic{
			IsInbound: true,
			Tag:       tag,
			Up:        up,
			Down:      inboundDown[tag],
		})
	}

	if len(traffics) > 0 || len(clientTraffics) > 0 {
		if err, _ := j.inboundService.AddTraffic(traffics, clientTraffics); err != nil {
			logger.Warning("mtproto job: add traffic failed:", err)
		}
	}
}
