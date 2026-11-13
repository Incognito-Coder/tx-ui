package model

import (
	"testing"
)

func TestHasPermission(t *testing.T) {
	// Staff with "inbounds:update" in permissions JSON
	u := &User{
		Role:        "staff",
		Permissions: `["inbounds:read","clients:read","inbounds:create","inbounds:update","inbounds:delete","clients:update"]`,
	}

	// Should match both edit and update
	if !u.HasPermission("inbounds:edit") {
		t.Errorf("Expected HasPermission('inbounds:edit') to be true when user has 'inbounds:update'")
	}
	if !u.HasPermission("inbounds:update") {
		t.Errorf("Expected HasPermission('inbounds:update') to be true when user has 'inbounds:update'")
	}
	if !u.HasPermission("inbounds:create") {
		t.Errorf("Expected HasPermission('inbounds:create') to be true")
	}
	if !u.HasPermission("inbounds:delete") {
		t.Errorf("Expected HasPermission('inbounds:delete') to be true")
	}
	if !u.HasPermission("clients:edit") {
		t.Errorf("Expected HasPermission('clients:edit') to be true when user has 'clients:update'")
	}
	if u.HasPermission("clients:create") {
		t.Errorf("Expected HasPermission('clients:create') to be false")
	}

	// Staff with "inbounds:edit" in permissions
	u2 := &User{
		Role:        "staff",
		Permissions: `["inbounds:edit","clients:edit"]`,
	}
	if !u2.HasPermission("inbounds:update") {
		t.Errorf("Expected HasPermission('inbounds:update') to be true when user has 'inbounds:edit'")
	}
	if !u2.HasPermission("clients:update") {
		t.Errorf("Expected HasPermission('clients:update') to be true when user has 'clients:edit'")
	}

	// Staff with wildcard "inbounds:*"
	u3 := &User{
		Role:        "staff",
		Permissions: `["inbounds:*"]`,
	}
	if !u3.HasPermission("inbounds:edit") {
		t.Errorf("Expected HasPermission('inbounds:edit') to be true with 'inbounds:*'")
	}
	if !u3.HasPermission("inbounds:create") {
		t.Errorf("Expected HasPermission('inbounds:create') to be true with 'inbounds:*'")
	}
	if u3.HasPermission("clients:edit") {
		t.Errorf("Expected HasPermission('clients:edit') to be false with 'inbounds:*'")
	}

	// Viewer role cannot edit
	uViewer := &User{
		Role:        "viewer",
		Permissions: "*",
	}
	if uViewer.HasPermission("inbounds:edit") {
		t.Errorf("Expected viewer HasPermission('inbounds:edit') to be false")
	}
	if !uViewer.HasPermission("inbounds:read") {
		t.Errorf("Expected viewer HasPermission('inbounds:read') to be true")
	}

	// Admin role has all permissions
	uAdmin := &User{
		Role: "admin",
	}
	if !uAdmin.HasPermission("inbounds:edit") {
		t.Errorf("Expected admin HasPermission('inbounds:edit') to be true")
	}
}

func TestCanAccessInbound(t *testing.T) {
	uAll := &User{
		Role:            "staff",
		AllowedInbounds: "*",
	}
	if !uAll.CanAccessInbound(1) || !uAll.CanAccessInbound(42) {
		t.Errorf("Expected CanAccessInbound to be true for all inbounds when AllowedInbounds is *")
	}

	uSpecific := &User{
		Role:            "staff",
		AllowedInbounds: "[1, 2, 5]",
	}
	if !uSpecific.CanAccessInbound(1) || !uSpecific.CanAccessInbound(2) || !uSpecific.CanAccessInbound(5) {
		t.Errorf("Expected CanAccessInbound to be true for allowed IDs")
	}
	if uSpecific.CanAccessInbound(3) {
		t.Errorf("Expected CanAccessInbound to be false for unallowed ID 3")
	}

	uComma := &User{
		Role:            "staff",
		AllowedInbounds: "1, 2, 5",
	}
	if !uComma.CanAccessInbound(2) || uComma.CanAccessInbound(99) {
		t.Errorf("Expected CanAccessInbound to parse comma-separated IDs properly")
	}
}
