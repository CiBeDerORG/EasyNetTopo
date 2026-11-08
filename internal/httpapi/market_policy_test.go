package httpapi

import "testing"

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
