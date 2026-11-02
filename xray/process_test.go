package xray

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewProcessFromConfigFile_NilSafety(t *testing.T) {
	// Must not panic when created from non-existent or empty path
	p := NewProcessFromConfigFile("/non/existent/path/config.json")
	if p == nil {
		t.Fatal("expected non-nil Process")
	}
	if p.GetConfig() != nil {
		t.Fatal("expected nil config before Start()")
	}
	if p.GetAPIPort() != 0 {
		t.Fatalf("expected apiPort to be 0, got %d", p.GetAPIPort())
	}
	if p.IsRunning() {
		t.Fatal("expected process not to be running")
	}

	// Calling refreshAPIPort on nil config must be safe
	p.process.refreshAPIPort()
}

func TestNewProcessFromConfigFile_ExistingConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	validConfig := []byte(`{
		"inbounds": [
			{
				"tag": "api",
				"port": 62789,
				"protocol": "dokodemo-door"
			}
		]
	}`)
	if err := os.WriteFile(configPath, validConfig, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	p := NewProcessFromConfigFile(configPath)
	if p == nil {
		t.Fatal("expected non-nil Process")
	}
	// Before Start, config is nil and apiPort is 0
	if p.GetConfig() != nil {
		t.Fatal("expected nil config before Start()")
	}
	if p.GetAPIPort() != 0 {
		t.Fatalf("expected apiPort to be 0 before Start, got %d", p.GetAPIPort())
	}
}

func TestRollbackHelpers(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", tempDir)

	configPath := GetConfigPath()
	lastWorkingPath := GetLastWorkingConfigPath()
	failedPath := GetLastFailedConfigPath()

	if HasLastWorkingConfig() {
		t.Fatal("expected HasLastWorkingConfig() to be false initially")
	}

	// Create a dummy working config.json
	workingConfig := []byte(`{"log":{"loglevel":"warning"}}`)
	if err := os.WriteFile(configPath, workingConfig, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Backup
	if err := BackupLastWorkingConfig(); err != nil {
		t.Fatalf("BackupLastWorkingConfig failed: %v", err)
	}

	if !HasLastWorkingConfig() {
		t.Fatal("expected HasLastWorkingConfig() to be true after backup")
	}

	backedUp, err := os.ReadFile(lastWorkingPath)
	if err != nil || string(backedUp) != string(workingConfig) {
		t.Fatalf("backed up content mismatch: err=%v, content=%s", err, string(backedUp))
	}

	// Overwrite config with broken content
	brokenConfig := []byte(`{"invalid": true}`)
	if err := os.WriteFile(configPath, brokenConfig, 0o644); err != nil {
		t.Fatalf("failed to write broken config: %v", err)
	}

	// Save failed config
	if err := SaveLastFailedConfig(brokenConfig); err != nil {
		t.Fatalf("SaveLastFailedConfig failed: %v", err)
	}

	failedData, err := os.ReadFile(failedPath)
	if err != nil || string(failedData) != string(brokenConfig) {
		t.Fatalf("failed config content mismatch: err=%v, content=%s", err, string(failedData))
	}

	// Rollback
	if err := RollbackToLastWorkingConfig(); err != nil {
		t.Fatalf("RollbackToLastWorkingConfig failed: %v", err)
	}

	restored, err := os.ReadFile(configPath)
	if err != nil || string(restored) != string(workingConfig) {
		t.Fatalf("restored content mismatch: err=%v, content=%s", err, string(restored))
	}
}

