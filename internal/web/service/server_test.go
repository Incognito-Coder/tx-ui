package service

import (
	"encoding/json"
	"testing"
)

func TestExtractMetricsSummary_JSON(t *testing.T) {
	jsonBody := []byte(`{
		"cmdline": ["/usr/local/x-ui/bin/xray-linux-amd64"],
		"goroutines": 42,
		"memstats": {
			"Alloc": 1234567,
			"Sys": 9876543,
			"HeapAlloc": 1000000,
			"HeapSys": 8000000,
			"HeapObjects": 5000,
			"NumGC": 10
		},
		"stats.inbound.api.uplink": 1024
	}`)

	var parsed map[string]interface{}
	err := json.Unmarshal(jsonBody, &parsed)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	summary := extractMetricsSummary(jsonBody, parsed)
	if summary.Goroutines != 42 {
		t.Errorf("Expected Goroutines=42, got %d", summary.Goroutines)
	}
	if summary.AllocBytes != 1234567 {
		t.Errorf("Expected AllocBytes=1234567, got %d", summary.AllocBytes)
	}
	if summary.HeapObjects != 5000 {
		t.Errorf("Expected HeapObjects=5000, got %d", summary.HeapObjects)
	}
	if summary.StatsTraffic["stats.inbound.api.uplink"] != 1024 {
		t.Errorf("Expected traffic stat 1024, got %d", summary.StatsTraffic["stats.inbound.api.uplink"])
	}
}

func TestExtractMetricsSummary_Prometheus(t *testing.T) {
	promBody := []byte(`# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 128
# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.
go_memstats_alloc_bytes 5242880
go_memstats_sys_bytes 20971520
go_memstats_heap_alloc_bytes 4194304
go_memstats_heap_sys_bytes 16777216
go_memstats_heap_objects 12000
go_memstats_gc_sys_bytes 50000
xray_traffic_inbound_uplink{tag="proxy"} 4096
`)

	summary := extractMetricsSummary(promBody, nil)
	if summary.Goroutines != 128 {
		t.Errorf("Expected Goroutines=128, got %d", summary.Goroutines)
	}
	if summary.AllocBytes != 5242880 {
		t.Errorf("Expected AllocBytes=5242880, got %d", summary.AllocBytes)
	}
	if summary.HeapObjects != 12000 {
		t.Errorf("Expected HeapObjects=12000, got %d", summary.HeapObjects)
	}
	if summary.StatsTraffic["xray_traffic_inbound_uplink"] != 4096 {
		t.Errorf("Expected traffic stat 4096, got %d", summary.StatsTraffic["xray_traffic_inbound_uplink"])
	}
}
