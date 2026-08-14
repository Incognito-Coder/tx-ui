package xray

import (
	"encoding/json"
	"testing"

	"x-ui/internal/util/json_util"

	"github.com/xtls/xray-core/infra/conf"
)

func TestConfigPreservesEnvironment(t *testing.T) {
	const input = `{"env":{"XRAY_LOCATION_ASSET":"/var/lib/xray"},"inbounds":[]}`

	var config Config
	if err := json.Unmarshal([]byte(input), &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	output, err := json.Marshal(&config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	env, ok := decoded["env"].(map[string]any)
	if !ok || env["XRAY_LOCATION_ASSET"] != "/var/lib/xray" {
		t.Fatalf("environment was not preserved: %s", output)
	}
}

func TestConfigEqualsIncludesRestartSensitiveSections(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "env", mutate: func(c *Config) { c.Env = json_util.RawMessage(`{"A":"B"}`) }},
		{name: "observatory", mutate: func(c *Config) { c.Observatory = json_util.RawMessage(`{"subjectSelector":["proxy"]}`) }},
		{name: "burst observatory", mutate: func(c *Config) { c.BurstObservatory = json_util.RawMessage(`{"subjectSelector":["proxy"]}`) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := &Config{}
			changed := &Config{}
			tt.mutate(changed)
			if base.Equals(changed) {
				t.Fatal("configs with different restart-sensitive sections compared equal")
			}
		})
	}
}

func TestConfigEqualsHandlesNil(t *testing.T) {
	var nilConfig *Config
	if !nilConfig.Equals(nil) {
		t.Fatal("two nil configs should compare equal")
	}
	if nilConfig.Equals(&Config{}) {
		t.Fatal("nil and non-nil configs should not compare equal")
	}
}

func TestXray26728XMCStreamConfigBuilds(t *testing.T) {
	const input = `{
		"network":"tcp",
		"security":"none",
		"finalmask":{"tcp":[{"type":"xmc","settings":{
			"hostname":"mc.example.com",
			"password":"shared-password",
			"profiles":[{
				"username":"TestPlayer",
				"uuid":"00112233-4455-6677-8899-aabbccddeeff",
				"texturesValue":"textures-value",
				"texturesSignature":"textures-signature"
			}]
		}}]}
	}`

	var stream conf.StreamConfig
	if err := json.Unmarshal([]byte(input), &stream); err != nil {
		t.Fatalf("unmarshal XMC stream config: %v", err)
	}
	built, err := stream.Build()
	if err != nil {
		t.Fatalf("build XMC stream config: %v", err)
	}
	if len(built.Tcpmasks) != 1 {
		t.Fatalf("expected one TCP mask, got %d", len(built.Tcpmasks))
	}
}
