package xray

import (
	"os"
	"testing"
)

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
