package httpapi

import (
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestResolveHookTargets(t *testing.T) {
	hosts := []storage.Host{
		{ID: "a-id", Name: "Machine A", AgentID: "agent-a"},
		{ID: "b-id", Name: "Machine B", AgentID: "agent-b"},
	}
	apps := []storage.CatalogApp{
		{SourceID: "one", AppID: "app-a", Name: "Service A"},
		{SourceID: "one", AppID: "app-b", Name: "Service B"},
	}
	targets, err := resolveTargets([]hookTarget{
		{HostName: "Machine A", ServiceName: "Service A"},
		{HostName: "Machine B", ServiceName: "app-b"},
		{HostName: "Machine A", ServiceName: "app-b"},
	}, hosts, apps)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 || targets[0].HostID != "a-id" || targets[0].AppID != "app-a" ||
		targets[1].HostID != "b-id" || targets[2].HostID != "a-id" {
		t.Fatalf("unexpected targets: %+v", targets)
	}
}

func TestResolveHookTargetsRejectAmbiguous(t *testing.T) {
	hosts := []storage.Host{{Name: "duplicate", AgentID: "agent1"}, {Name: "duplicate", AgentID: "agent2"}}
	apps := []storage.CatalogApp{{AppID: "app", Name: "App", SourceID: "source"}}
	if _, err := resolveTargets([]hookTarget{{HostName: "duplicate", ServiceName: "App"}}, hosts, apps); err == nil {
		t.Fatal("must reject duplicate host names")
	}
	hosts = []storage.Host{{ID: "valid", Name: "machine", AgentID: "agent"}}
	apps = append(apps, storage.CatalogApp{AppID: "different", Name: "App", SourceID: "source2"})
	if _, err := resolveTargets([]hookTarget{{HostName: "machine", ServiceName: "App"}}, hosts, apps); err == nil {
		t.Fatal("must reject ambiguous app display names")
	}
	if _, err := resolveTargets([]hookTarget{{HostName: "missing", ServiceName: "App"}}, hosts, apps); err == nil {
		t.Fatal("must reject unknown host")
	}
}


func TestResolveHookTargetsAcceptsNativeAgentWithoutCatalogEntry(t *testing.T) {
	hosts := []storage.Host{{ID: "host", Name: "Mac mini", AgentID: "paired"}}
	targets, err := resolveTargets([]hookTarget{{HostName: "Mac mini", ServiceName: "easyhostagent"}}, hosts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].SourceID != storage.NativeAgentSourceID ||
		targets[0].AppID != storage.NativeAgentAppID || targets[0].Action != "update" {
		t.Fatalf("native Agent target: %+v", targets)
	}
}
