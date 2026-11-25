package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
	"github.com/CiBeDerORG/EasyNetTopo/internal/tasks"
)

func TestManualUpdateRetainsConfirmedRevision(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc, err := catalog.NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	source, err := db.CreateCatalogSource(ctx, "应用源", "https://github.com/example/catalog.git", "main")
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := tasks.New(ctx, store, svc, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, next := strings.Repeat("a", 40), strings.Repeat("b", 40)
	saveSnapshot := func(revision string, catalogOnly bool) {
		t.Helper()
		if err := db.SaveCatalogSnapshot(ctx, source.ID, revision, []storage.CatalogApp{{
			SourceID: source.ID, AppID: "demo", Revision: revision, CatalogOnly: catalogOnly,
		}}); err != nil {
			t.Fatal(err)
		}
	}
	request := func() int {
		body := fmt.Sprintf(`{"source_id":%q,"app_id":"demo","revision":%q}`, source.ID, first)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/hosts/host/apps/update", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		enqueueManualUpdate(w, req, "host", runner, svc)
		return w.Code
	}
	saveSnapshot(first, true)
	if code := request(); code != http.StatusUnprocessableEntity {
		t.Fatalf("只展示的条目不应接受更新请求：HTTP %d", code)
	}
	saveSnapshot(first, false)
	if code := request(); code != http.StatusAccepted {
		t.Fatalf("当前版本未入队：HTTP %d", code)
	}
	saveSnapshot(next, false)
	if code := request(); code != http.StatusConflict {
		t.Fatalf("刷新后仍然接受旧版本的新请求：HTTP %d", code)
	}
	task, ok, err := store.Next(ctx, "host")
	if err != nil || !ok || task.Revision != first {
		t.Fatalf("已入队的更新未保留确认版本：%+v，%v", task, err)
	}
}

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

func TestResolveHookTargetsRejectCatalogOnlyApp(t *testing.T) {
	hosts := []storage.Host{{ID: "host", Name: "Mac mini", AgentID: "paired"}}
	apps := []storage.CatalogApp{{AppID: "maestro", Name: "Maestro", CatalogOnly: true}}
	for _, service := range []string{"maestro", "Maestro"} {
		if _, err := resolveTargets([]hookTarget{{HostName: "Mac mini", ServiceName: service}}, hosts, apps); err == nil {
			t.Fatalf("只展示的目录条目不能通过 %q 创建更新任务", service)
		}
	}
}
