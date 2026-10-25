package service

import (
	"testing"
)

func TestXrayService_RestartState(t *testing.T) {
	s := &XrayService{}

	// Initial state when p is nil and not restarting
	isRestarting.Store(false)
	isManuallyStopped.Store(false)

	if s.IsXrayRunning() {
		t.Errorf("expected IsXrayRunning to be false when p is nil")
	}
	if s.IsXrayRestarting() {
		t.Errorf("expected IsXrayRestarting to be false initially")
	}
	if !s.DidXrayCrash() {
		t.Errorf("expected DidXrayCrash to be true when not running and not manually stopped")
	}

	// While restarting
	isRestarting.Store(true)

	if s.IsXrayRunning() {
		t.Errorf("expected IsXrayRunning to be false while restarting")
	}
	if !s.IsXrayRestarting() {
		t.Errorf("expected IsXrayRestarting to be true while restarting")
	}
	// DidXrayCrash must NOT report crash during an active restart
	if s.DidXrayCrash() {
		t.Errorf("expected DidXrayCrash to be false while restarting")
	}

	// GetXrayTraffic must return error immediately without dialing when restarting or not running
	traffics, clientTraffics, err := s.GetXrayTraffic()
	if err == nil {
		t.Errorf("expected error from GetXrayTraffic when restarting, got nil")
	}
	if traffics != nil || clientTraffics != nil {
		t.Errorf("expected nil traffics and clientTraffics, got %v, %v", traffics, clientTraffics)
	}

	// Reset state
	isRestarting.Store(false)
}
