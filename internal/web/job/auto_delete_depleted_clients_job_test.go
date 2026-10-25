package job

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
	"x-ui/internal/web/service"
	"x-ui/xray"
)

func TestAutoDeleteDepletedClientsJob_DisabledWhenZero(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tx-ui-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatal(err)
	}

	settingService := &service.SettingService{}
	inboundService := &service.InboundService{}

	// Set autoDeleteDay = "0" (disabled)
	db := database.GetDB()
	db.Save(&model.Setting{Key: "autoDeleteDay", Value: "0"})

	// Create a depleted client traffic record that is expired 30 days ago
	traffic := &xray.ClientTraffic{
		InboundId:  1,
		Email:      "depleted@test.com",
		Enable:     false,
		Reset:      0,
		ExpiryTime: time.Now().Add(-30 * 24 * time.Hour).UnixMilli(),
	}
	db.Create(traffic)

	job := &AutoDeleteDepletedClientsJob{
		settingService: settingService,
		inboundService: inboundService,
	}

	// Run the job with autoDeleteDay == 0
	job.Run()

	// Verify the traffic record was NOT deleted
	var count int64
	db.Model(&xray.ClientTraffic{}).Where("email = ?", "depleted@test.com").Count(&count)
	if count != 1 {
		t.Fatalf("expected depleted client to remain untouched when autoDeleteDay is 0, but count is %d", count)
	}

	// Now set autoDeleteDay = "-1" (negative)
	db.Save(&model.Setting{Key: "autoDeleteDay", Value: "-1"})
	job.Run()

	db.Model(&xray.ClientTraffic{}).Where("email = ?", "depleted@test.com").Count(&count)
	if count != 1 {
		t.Fatalf("expected depleted client to remain untouched when autoDeleteDay is -1, but count is %d", count)
	}
}
