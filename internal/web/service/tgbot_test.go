package service

import (
	"bytes"
	"testing"

	"x-ui/internal/web/global"

	"github.com/skip2/go-qrcode"
)

type mockLinkService struct {
	expectedEmail string
	links         []string
	err           error
}

func (m *mockLinkService) GetConfigLinksByEmail(email string) (string, []string, error) {
	if m.err != nil {
		return email, nil, m.err
	}
	return m.expectedEmail, m.links, nil
}

func TestTgbot_GetClientConfigLinksByEmail_WithService(t *testing.T) {
	mock := &mockLinkService{
		expectedEmail: "test@example.com",
		links: []string{
			"vless://uuid@domain:443?type=tcp#test",
		},
	}
	global.SetConfigLinkService(mock)

	tg := &Tgbot{}
	email, links, err := tg.getClientConfigLinksByEmail("test@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", email)
	}
	if len(links) != 1 || links[0] != "vless://uuid@domain:443?type=tcp#test" {
		t.Errorf("unexpected links: %v", links)
	}
}

func TestTgbot_QRCodeGeneration(t *testing.T) {
	sampleLink := "vless://b831381d-6324-4d53-ad4f-8cda48b30811@example.com:443?security=reality&encryption=none&pbk=xyz&headerType=none&type=tcp&flow=xtls-rprx-vision#example"

	pngBytes, err := qrcode.Encode(sampleLink, qrcode.Medium, 512)
	if err != nil {
		t.Fatalf("qrcode.Encode failed: %v", err)
	}
	if len(pngBytes) == 0 {
		t.Fatal("expected non-empty png bytes")
	}

	// Verify PNG header: 89 50 4E 47 0D 0A 1A 0A
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(pngBytes, pngHeader) {
		t.Fatalf("generated data does not have valid PNG header")
	}
}
