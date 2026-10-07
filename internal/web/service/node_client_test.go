package service

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"x-ui/internal/database"
	"x-ui/internal/database/model"
)

func setupTestDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := database.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() {
		if db := database.GetDB(); db != nil {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
	})
}

func TestBulkSetLinks(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	ncService := &NodeClientService{}

	// Create test inbounds
	ib1 := &model.Inbound{Id: 101, UserId: 1, Tag: "inbound-101", Port: 1010, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}
	ib2 := &model.Inbound{Id: 102, UserId: 1, Tag: "inbound-102", Port: 1020, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}
	ib3 := &model.Inbound{Id: 103, UserId: 1, Tag: "inbound-103", Port: 1030, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[]}`}
	if err := db.Create(ib1).Error; err != nil {
		t.Fatalf("create ib1 failed: %v", err)
	}
	if err := db.Create(ib2).Error; err != nil {
		t.Fatalf("create ib2 failed: %v", err)
	}
	if err := db.Create(ib3).Error; err != nil {
		t.Fatalf("create ib3 failed: %v", err)
	}

	// Create test clients with unique SubIDs
	c1 := &model.NodeClient{Id: 1, Email: "client1@example.com", SubID: "sub-1", UUID: "uuid-1", Enable: true}
	c2 := &model.NodeClient{Id: 2, Email: "client2@example.com", SubID: "sub-2", UUID: "uuid-2", Enable: true}
	c3 := &model.NodeClient{Id: 3, Email: "client3@example.com", SubID: "sub-3", UUID: "uuid-3", Enable: true}
	if err := db.Create(c1).Error; err != nil {
		t.Fatalf("create c1 failed: %v", err)
	}
	if err := db.Create(c2).Error; err != nil {
		t.Fatalf("create c2 failed: %v", err)
	}
	if err := db.Create(c3).Error; err != nil {
		t.Fatalf("create c3 failed: %v", err)
	}

	// 1. Bulk Set (replace) links for c1 and c2 with [101, 102]
	_, err := ncService.BulkSetLinks([]int{1, 2}, []int{101, 102}, "set", "xtls-rprx-vision")
	if err != nil {
		t.Fatalf("BulkSetLinks 'set' failed: %v", err)
	}

	links1, err := ncService.GetLinks(1)
	if err != nil || len(links1) != 2 {
		t.Fatalf("expected 2 links for client 1, got %d, err: %v", len(links1), err)
	}
	links2, err := ncService.GetLinks(2)
	if err != nil || len(links2) != 2 {
		t.Fatalf("expected 2 links for client 2, got %d, err: %v", len(links2), err)
	}

	for _, l := range links1 {
		if l.Flow != "xtls-rprx-vision" {
			t.Errorf("expected flow xtls-rprx-vision, got %s", l.Flow)
		}
	}

	// 2. Bulk Add inbound 103 to clients 1, 2, and 3
	_, err = ncService.BulkSetLinks([]int{1, 2, 3}, []int{103}, "add", "")
	if err != nil {
		t.Fatalf("BulkSetLinks 'add' failed: %v", err)
	}

	links1AfterAdd, _ := ncService.GetLinks(1)
	if len(links1AfterAdd) != 3 {
		t.Errorf("expected 3 links for client 1 after add, got %d", len(links1AfterAdd))
	}
	links3AfterAdd, _ := ncService.GetLinks(3)
	if len(links3AfterAdd) != 1 || links3AfterAdd[0].InboundId != 103 {
		t.Errorf("expected 1 link (103) for client 3 after add, got %d", len(links3AfterAdd))
	}

	// 3. Bulk Remove inbound 101 from clients 1 and 2
	_, err = ncService.BulkSetLinks([]int{1, 2}, []int{101}, "remove", "")
	if err != nil {
		t.Fatalf("BulkSetLinks 'remove' failed: %v", err)
	}

	links1AfterRemove, _ := ncService.GetLinks(1)
	if len(links1AfterRemove) != 2 {
		t.Errorf("expected 2 links for client 1 after remove (102, 103), got %d", len(links1AfterRemove))
	}
	for _, l := range links1AfterRemove {
		if l.InboundId == 101 {
			t.Errorf("inbound 101 should have been removed from client 1")
		}
	}

	// 4. Bulk Replace with empty list (unlinks all inbounds)
	_, err = ncService.BulkSetLinks([]int{1}, []int{}, "set", "")
	if err != nil {
		t.Fatalf("BulkSetLinks empty 'set' failed: %v", err)
	}
	links1Empty, _ := ncService.GetLinks(1)
	if len(links1Empty) != 0 {
		t.Errorf("expected 0 links for client 1 after empty set, got %d", len(links1Empty))
	}
}

func TestBulkSetLinks_ConcurrencyAndNoLock(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	ncService := &NodeClientService{}

	// Create 5 inbounds
	inboundIds := []int{201, 202, 203, 204, 205}
	for _, id := range inboundIds {
		ib := &model.Inbound{
			Id:       id,
			UserId:   1,
			Tag:      fmt.Sprintf("inbound-%d", id),
			Port:     id * 10,
			Protocol: model.VLESS,
			Enable:   true,
			Settings: `{"clients":[]}`,
		}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create ib %d failed: %v", id, err)
		}
	}

	// Create 20 clients
	var clientIds []int
	for i := 10; i < 30; i++ {
		c := &model.NodeClient{
			Id:     i,
			Email:  "stress" + string(rune('a'+i-10)) + "@test.com",
			SubID:  "sub-stress-" + string(rune('a'+i-10)),
			UUID:   "uuid-stress-" + string(rune('a'+i-10)),
			Enable: true,
		}
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("create client %d failed: %v", i, err)
		}
		clientIds = append(clientIds, i)
	}

	// Run concurrent writers and readers
	var wg sync.WaitGroup
	errCh := make(chan error, 50)

	// Writer 1: BulkSetLinks on all clients
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 5; j++ {
			if _, err := ncService.BulkSetLinks(clientIds, inboundIds, "set", "xtls-rprx-vision"); err != nil {
				errCh <- err
			}
		}
	}()

	// Writer 2: BulkSetLinks add & remove
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 5; j++ {
			if _, err := ncService.BulkSetLinks(clientIds[:5], []int{201, 202}, "add", ""); err != nil {
				errCh <- err
			}
			if _, err := ncService.BulkSetLinks(clientIds[:5], []int{201}, "remove", ""); err != nil {
				errCh <- err
			}
		}
	}()

	// Writer 3: Single SetLinks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 5; j++ {
			links := []NodeClientLinkInput{
				{InboundId: 203, Flow: "xtls-rprx-vision"},
				{InboundId: 204, Flow: ""},
			}
			if _, err := ncService.SetLinks(10, links); err != nil {
				errCh <- err
			}
		}
	}()

	// Readers: Concurrent reads simulating frontend background polling
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				var inbounds []model.Inbound
				if err := db.Find(&inbounds).Error; err != nil {
					errCh <- err
				}
				if _, err := ncService.GetAllWithDetails(); err != nil {
					errCh <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("encountered error during concurrent stress test: %v", err)
	}
}
