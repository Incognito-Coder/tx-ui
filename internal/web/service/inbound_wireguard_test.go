package service

import (
	"testing"
	"x-ui/internal/database/model"
	"x-ui/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}

	err = db.AutoMigrate(
		&model.Inbound{},
		&xray.ClientTraffic{},
		&model.InboundClientIps{},
		&model.NodeClient{},
		&model.NodeClientLink{},
	)
	if err != nil {
		t.Fatalf("failed to automigrate: %v", err)
	}
	return db
}

func TestAddClientStat_IdempotencyAndNoUniqueConstraintError(t *testing.T) {
	db := setupTestDB(t)
	svc := &InboundService{}

	inboundId := 1
	client := &model.Client{
		Email:      "test-wg-client@example.com",
		TotalGB:    100,
		ExpiryTime: 1700000000000,
		Enable:     true,
		Reset:      30,
	}

	// 1. Initial insert
	tx1 := db.Begin()
	err := svc.AddClientStat(tx1, inboundId, client)
	if err != nil {
		tx1.Rollback()
		t.Fatalf("AddClientStat failed on initial insert: %v", err)
	}
	tx1.Commit()

	// Simulate accumulated traffic
	db.Model(&xray.ClientTraffic{}).
		Where("inbound_id = ? AND email = ?", inboundId, client.Email).
		Updates(map[string]interface{}{
			"up":   1234567,
			"down": 7654321,
		})

	// 2. Second insert/update with changed configuration
	updatedClient := &model.Client{
		Email:      "test-wg-client@example.com",
		TotalGB:    200,
		ExpiryTime: 1800000000000,
		Enable:     false,
		Reset:      60,
	}

	tx2 := db.Begin()
	err = svc.AddClientStat(tx2, inboundId, updatedClient)
	if err != nil {
		tx2.Rollback()
		t.Fatalf("AddClientStat should not fail with unique constraint error on existing client: %v", err)
	}
	tx2.Commit()

	var record xray.ClientTraffic
	err = db.Where("inbound_id = ? AND email = ?", inboundId, client.Email).First(&record).Error
	if err != nil {
		t.Fatalf("failed to find client traffic record: %v", err)
	}

	// Traffic stats must be preserved!
	if record.Up != 1234567 || record.Down != 7654321 {
		t.Errorf("traffic stats were overwritten: up=%d, down=%d", record.Up, record.Down)
	}
	// Configuration must be updated
	if record.Total != 200 || record.ExpiryTime != 1800000000000 || record.Enable != false || record.Reset != 60 {
		t.Errorf("configuration was not updated properly: %+v", record)
	}

	// 3. Empty email should be a safe no-op
	tx3 := db.Begin()
	err = svc.AddClientStat(tx3, inboundId, &model.Client{Email: ""})
	if err != nil {
		tx3.Rollback()
		t.Fatalf("AddClientStat with empty email should be no-op: %v", err)
	}
	tx3.Commit()
}

func TestUpdateClientTraffics_WireguardExistingClients(t *testing.T) {
	db := setupTestDB(t)
	svc := &InboundService{}

	inboundId := 10

	// Pre-populate client traffic record for inboundId and user1
	db.Create(&xray.ClientTraffic{
		InboundId:  inboundId,
		Email:      "wg-peer1@example.com",
		Total:      50,
		ExpiryTime: 1700000000000,
		Enable:     true,
		Up:         1000,
		Down:       2000,
	})

	// oldInbound has settings with "peers"
	oldInbound := &model.Inbound{
		Id:       inboundId,
		Protocol: model.WireGuard,
		Settings: `{
			"secretKey": "aW52YWxpZHNlY3JldGtleWZvcmhlbGxvMTIzNDU2Nzg5MDE=",
			"address": ["10.0.0.1/24"],
			"peers": [
				{
					"email": "wg-peer1@example.com",
					"publicKey": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
					"allowedIPs": ["10.0.0.2/32"],
					"totalGB": 50,
					"enable": true
				}
			]
		}`,
	}

	// newInbound has settings with "clients" and updated totalGB
	newInbound := &model.Inbound{
		Id:       inboundId,
		Protocol: model.WireGuard,
		Settings: `{
			"secretKey": "aW52YWxpZHNlY3JldGtleWZvcmhlbGxvMTIzNDU2Nzg5MDE=",
			"address": ["10.0.0.1/24"],
			"peers": [
				{
					"email": "wg-peer1@example.com",
					"publicKey": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
					"allowedIPs": ["10.0.0.2/32"],
					"totalGB": 100,
					"enable": true
				}
			],
			"clients": [
				{
					"email": "wg-peer1@example.com",
					"publicKey": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
					"allowedIPs": ["10.0.0.2/32"],
					"totalGB": 100,
					"enable": true
				}
			]
		}`,
	}

	tx := db.Begin()
	err := svc.updateClientTraffics(tx, oldInbound, newInbound)
	if err != nil {
		tx.Rollback()
		t.Fatalf("updateClientTraffics failed: %v", err)
	}
	tx.Commit()

	var record xray.ClientTraffic
	err = db.Where("inbound_id = ? AND email = ?", inboundId, "wg-peer1@example.com").First(&record).Error
	if err != nil {
		t.Fatalf("record not found: %v", err)
	}
	if record.Total != 100 {
		t.Errorf("expected total 100, got %d", record.Total)
	}
	if record.Up != 1000 || record.Down != 2000 {
		t.Errorf("expected traffic to remain intact, got up=%d, down=%d", record.Up, record.Down)
	}
}
