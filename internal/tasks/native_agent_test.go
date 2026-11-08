package tasks

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestNativeAgentRestartedSameCommit(t *testing.T) {
	before := nativeAgentBaseline{StartedAt: "2026-11-08T14:55:00Z", Commit: "same-commit"}
	cases := []struct {
		name  string
		after nativeAgentBaseline
		want  bool
	}{
		{"same commit after restart", nativeAgentBaseline{StartedAt: "2026-11-08T14:56:00Z", Commit: "same-commit"}, true},
		{"new commit after restart", nativeAgentBaseline{StartedAt: "2026-11-08T14:56:00Z", Commit: "new-commit"}, true},
		{"commit unavailable after restart", nativeAgentBaseline{StartedAt: "2026-11-08T14:56:00Z", Commit: "unknown"}, true},
		{"network restored without restart", nativeAgentBaseline{StartedAt: before.StartedAt, Commit: before.Commit}, false},
		{"missing restart marker", nativeAgentBaseline{}, false},
		{"invalid restart marker", nativeAgentBaseline{StartedAt: "not-a-timestamp"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeAgentRestarted(before, tc.after); got != tc.want {
				t.Fatalf("nativeAgentRestarted() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveLegacyInterruptedNativeAgentAfterRecovery(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "interrupted-native.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := storage.NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Enqueue(ctx, "github_actions", []storage.TaskTarget{
		{HostID: "mac", SourceID: storage.NativeAgentSourceID, AppID: storage.NativeAgentAppID, Action: "update"},
		{HostID: "mac", SourceID: "git", AppID: "following-app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	native, ok, err := store.Next(ctx, "mac")
	if err != nil || !ok {
		t.Fatalf("native task: %+v, %v", native, err)
	}
	for _, state := range []string{"preparing", "running", "interrupted"} {
		if err := store.SetState(ctx, native, state, "Agent 已重启，但 Commit 未变化"); err != nil {
			t.Fatal(err)
		}
	}
	interrupted, err := store.Get(ctx, native.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 老任务的基线消息已经被中断错误覆盖；任务开始时间仍可用于
	// 和已配对 Agent 实时返回的进程启动时间比较。
	if interrupted.StartedAt == "" {
		t.Fatal("task start time must survive interruption")
	}
	svc := &Service{store: store}
	future := time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano)
	restarted, err := svc.ResolveNativeInterrupted(ctx, native.ID, future)
	if err != nil || !restarted {
		t.Fatalf("recovered native update: restarted=%v err=%v", restarted, err)
	}
	current, err := store.Get(ctx, native.ID)
	if err != nil || current.State != "succeeded" {
		t.Fatalf("recovered state: %+v, %v", current, err)
	}
	next, ok, err := store.Next(ctx, "mac")
	if err != nil || !ok || next.AppID != "following-app" {
		t.Fatalf("host queue not released: %+v, %v", next, err)
	}
}

func TestResolveNativeInterruptedWithoutRestartNeedsManualConfirmation(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "manual-native.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := storage.NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Enqueue(ctx, "manual", []storage.TaskTarget{
		{HostID: "mac", SourceID: storage.NativeAgentSourceID, AppID: storage.NativeAgentAppID, Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	native, ok, err := store.Next(ctx, "mac")
	if err != nil || !ok {
		t.Fatalf("native task: %+v, %v", native, err)
	}
	if err := store.SetState(ctx, native, "running", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetState(ctx, native, "interrupted", "未确认重启"); err != nil {
		t.Fatal(err)
	}
	svc := &Service{store: store}
	if _, err := svc.ResolveNativeInterrupted(ctx, native.ID, "malformed"); err == nil {
		t.Fatal("malformed authenticated start marker must be rejected")
	}
	earlier := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	restarted, err := svc.ResolveNativeInterrupted(ctx, native.ID, earlier)
	if err != nil || restarted {
		t.Fatalf("no restart expected: restarted=%v err=%v", restarted, err)
	}
	updated, err := store.Get(ctx, native.ID)
	if err != nil || updated.State != "failed" {
		t.Fatalf("manual release should close unconfirmed operation as failed: %+v, %v", updated, err)
	}
}
