package httpapi

import "testing"

func TestConditionalCatalogConfigFieldInputPolicy(t *testing.T) {
	fields := map[string]map[string]any{
		"ROLE": {
			"key": "ROLE", "type": "string", "default": "cloud", "required": true,
			"options": []any{
				map[string]any{"label": "Cloud", "value": "cloud"},
				map[string]any{"label": "Intranet", "value": "intranet"},
			},
		},
		"CLOUD_URL": {
			"key": "CLOUD_URL", "type": "string", "required": true,
			"visible_when": map[string]any{"field": "ROLE", "equals": "cloud"},
		},
		"INTRANET_URL": {
			"key": "INTRANET_URL", "type": "string", "required": true,
			"visible_when": map[string]any{"field": "ROLE", "equals": "intranet"},
		},
		"INTERNAL": {"key": "INTERNAL", "custom_input": false},
	}
	cloudConfig := map[string]string{"ROLE": "cloud", "CLOUD_URL": "https://example.test"}
	if !catalogConfigFieldInputAllowed("CLOUD_URL", fields["CLOUD_URL"], fields, cloudConfig) {
		t.Fatal("cloud role should accept its visible field")
	}
	if catalogConfigFieldInputAllowed("INTRANET_URL", fields["INTRANET_URL"], fields, cloudConfig) {
		t.Fatal("cloud role must reject the hidden intranet field")
	}
	if catalogConfigFieldInputAllowed("INTERNAL", fields["INTERNAL"], fields, cloudConfig) {
		t.Fatal("custom_input=false field must remain protected")
	}
	if missing := catalogRequiredConfigFieldMissing(fields, map[string]string{"ROLE": "cloud"}); missing != "CLOUD_URL" {
		t.Fatalf("cloud role must require its visible URL, got %q", missing)
	}
	if missing := catalogRequiredConfigFieldMissing(fields, cloudConfig); missing != "" {
		t.Fatalf("complete cloud configuration should pass, got %q", missing)
	}

	intranetConfig := map[string]string{"ROLE": "intranet", "INTRANET_URL": "udp://example.test"}
	if !catalogConfigFieldInputAllowed("INTRANET_URL", fields["INTRANET_URL"], fields, intranetConfig) {
		t.Fatal("intranet role should accept its visible field")
	}
	if catalogConfigFieldInputAllowed("CLOUD_URL", fields["CLOUD_URL"], fields, intranetConfig) {
		t.Fatal("intranet role must reject the hidden cloud field")
	}
	if missing := catalogRequiredConfigFieldMissing(fields, map[string]string{"ROLE": "intranet"}); missing != "INTRANET_URL" {
		t.Fatalf("intranet role must require its visible URL, got %q", missing)
	}
	if missing := catalogRequiredConfigFieldMissing(fields, intranetConfig); missing != "" {
		t.Fatalf("complete intranet configuration should pass, got %q", missing)
	}

	defaultConfig := map[string]string{"CLOUD_URL": "https://example.test"}
	if !catalogConfigFieldInputAllowed("CLOUD_URL", fields["CLOUD_URL"], fields, defaultConfig) {
		t.Fatal("a missing selector should resolve through its declared default")
	}
	invalidOption := map[string]string{"ROLE": "unknown"}
	if catalogConfigFieldInputAllowed("ROLE", fields["ROLE"], fields, invalidOption) {
		t.Fatal("selector values outside options must be rejected")
	}
}

func TestEasyNetTopoMarketUpdateOnly(t *testing.T) {
	for _, action := range []string{"install", "uninstall", "force_update"} {
		if marketActionAllowed("easynettopo", action) {
			t.Errorf("EasyNetTopo must reject %s", action)
		}
	}
	for _, action := range []string{"update", "adopt", "status"} {
		if !marketActionAllowed("easynettopo", action) {
			t.Errorf("EasyNetTopo must allow %s (adopt supports automatic legacy linking)", action)
		}
	}
	if !marketActionAllowed("portainer", "install") || !marketActionAllowed("portainer", "uninstall") {
		t.Fatal("Other applications must retain their existing action policy")
	}
}
