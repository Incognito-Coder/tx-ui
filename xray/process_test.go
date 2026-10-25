package xray

import (
	"testing"
)

func TestProcess_IsRunningAndStopping(t *testing.T) {
	cfg := &Config{
		InboundConfigs: []InboundConfig{
			{
				Tag:  "api",
				Port: 62789,
			},
		},
	}

	p := newProcess(cfg)

	// API port must be refreshed immediately upon process creation
	if p.apiPort != 62789 {
		t.Fatalf("expected apiPort to be 62789, got %d", p.apiPort)
	}

	// Not started yet
	if p.IsRunning() {
		t.Errorf("expected IsRunning to be false initially")
	}

	// Mark as stopping
	p.isStopping.Store(true)
	if p.IsRunning() {
		t.Errorf("expected IsRunning to be false when isStopping is true")
	}
}
