package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestTaskHierarchyAndPerHostScheduling(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "task-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	targets := []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "a1", Action: "update"},
		{HostID: "B", SourceID: "source", AppID: "b1", Action: "update"},
		{HostID: "A", SourceID: "source", AppID: "a2", Action: "update"},
		{HostID: "A", SourceID: "source", AppID: "a1", Action: "update"},
	}
	created, err := store.Enqueue(ctx, "github_actions", targets)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 5 {
		t.Fatalf("want two host tasks and three app tasks, got %d", len(created))
	}
	hosts, err := store.PendingHosts(ctx)
	if err != nil || len(hosts) != 2 {
		t.Fatalf("pending hosts = %v, %v", hosts, err)
	}
	a1, ok, err := store.Next(ctx, "A")
	if err != nil || !ok || a1.AppID != "a1" {
		t.Fatalf("first A task = %+v (%v, %v)", a1, ok, err)
	}
	b1, ok, err := store.Next(ctx, "B")
	if err != nil || !ok || b1.AppID != "b1" {
		t.Fatalf("independent B task = %+v (%v, %v)", b1, ok, err)
	}
	for _, state := range []string{"preparing", "running", "succeeded"} {
		if err := store.SetState(ctx, a1, state, ""); err != nil {
			t.Fatal(err)
		}
	}
	a2, ok, err := store.Next(ctx, "A")
	if err != nil || !ok || a2.AppID != "a2" || a2.ParentID != a1.ParentID {
		t.Fatalf("second A task = %+v (%v, %v)", a2, ok, err)
	}
	if err := store.SetState(ctx, a2, "failed", "test failure"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetState(ctx, b1, "succeeded", ""); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListLatest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parents := map[string]string{}
	for _, row := range rows {
		if row.Type == "host" {
			parents[row.HostID] = row.State
		}
	}
	if parents["A"] != "failed" || parents["B"] != "succeeded" {
		t.Fatalf("wrong parent aggregation: %v", parents)
	}
}

func TestTaskInterruptedBlocksHostAndPreparationRecovery(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "task-recover.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "one", Action: "update"},
		{HostID: "A", SourceID: "source", AppID: "two", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, ok, err := store.Next(ctx, "A")
	if err != nil || !ok {
		t.Fatalf("next: %v", err)
	}
	if err := store.SetState(ctx, first, "preparing", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverPreparation(ctx); err != nil {
		t.Fatal(err)
	}
	next, ok, err := store.Next(ctx, "A")
	if err != nil || !ok || next.ID != first.ID || next.State != "queued" {
		t.Fatalf("restart should retry only preparation: %+v (%v)", next, err)
	}
	if err := store.SetState(ctx, first, "interrupted", "uncertain"); err != nil {
		t.Fatal(err)
	}
	_, ok, err = store.Next(ctx, "A")
	if err != nil || ok {
		t.Fatal("interrupted task must block subsequent updates")
	}
}

func TestNativeAgentAndAppShareHostQueue(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "native-queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.Enqueue(ctx, "github_actions", []TaskTarget{
		{HostID: "host", SourceID: NativeAgentSourceID, AppID: NativeAgentAppID, Action: "update"},
		{HostID: "host", SourceID: "git-source", AppID: "other-app", Action: "update"},
	})
	if err != nil || len(rows) != 3 {
		t.Fatalf("native and application tasks must share one parent: %+v, %v", rows, err)
	}
	native, ok, err := store.Next(ctx, "host")
	if err != nil || !ok || native.SourceID != NativeAgentSourceID || native.AppID != NativeAgentAppID {
		t.Fatalf("native should run first: %+v, %v", native, err)
	}
	if err := store.SetState(ctx, native, "running", "agent-baseline:old"); err != nil {
		t.Fatal(err)
	}
	running, ok, err := store.Next(ctx, "host")
	if err != nil || !ok || running.ID != native.ID || running.Message != "agent-baseline:old" {
		t.Fatalf("restart must resume the same native task, not issue new work: %+v, %v", running, err)
	}
	if err := store.SetState(ctx, native, "succeeded", ""); err != nil {
		t.Fatal(err)
	}
	next, ok, err := store.Next(ctx, "host")
	if err != nil || !ok || next.AppID != "other-app" {
		t.Fatalf("application should follow native update: %+v, %v", next, err)
	}
}

func TestEnqueueLatestWinsOnlyForSameHostAndApp(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "latest-wins.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	old1Rows, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "same-app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	old1 := old1Rows[1]

	old2Rows, err := store.Enqueue(ctx, "github_actions", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "same-app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	old2 := old2Rows[1]

	otherHostRows, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "B", SourceID: "source", AppID: "same-app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherHost := otherHostRows[1]

	otherAppRows, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "other-app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherApp := otherAppRows[1]

	otherSourceRows, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "other-source", AppID: "same-app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherSource := otherSourceRows[1]

	latestRows, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "same-app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	latest := latestRows[1]

	for _, id := range []string{old1.ID, old2.ID} {
		got, err := store.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != "cancelled" || got.Message != "已被更新任务替代" {
			t.Fatalf("old task %s should be cancelled, got %+v", id, got)
		}
		if err := store.SetState(ctx, got, "succeeded", "late worker result"); err != ErrTaskSuperseded {
			t.Fatalf("late worker must not overwrite cancelled task, got %v", err)
		}
		parent, err := store.Get(ctx, got.ParentID)
		if err != nil || parent.State != "cancelled" {
			t.Fatalf("old parent should be cancelled: %+v, %v", parent, err)
		}
	}

	for _, task := range []Task{otherHost, otherApp, otherSource, latest} {
		got, err := store.Get(ctx, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != "queued" {
			t.Fatalf("unrelated task %s was affected: %+v", task.ID, got)
		}
	}

	nextA, ok, err := store.Next(ctx, "A")
	if err != nil || !ok || nextA.ID != otherApp.ID {
		t.Fatalf("same-host other app must remain queued: %+v, %v, %v", nextA, ok, err)
	}
	nextB, ok, err := store.Next(ctx, "B")
	if err != nil || !ok || nextB.ID != otherHost.ID {
		t.Fatalf("same app on another host must remain queued: %+v, %v, %v", nextB, ok, err)
	}
}

func TestEnqueueCancelsRunningTaskAndRejectsLateCompletion(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "running-latest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	oldRows, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	old := oldRows[1]
	if err := store.SetState(ctx, old, "preparing", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetState(ctx, old, "running", ""); err != nil {
		t.Fatal(err)
	}
	newRows, err := store.Enqueue(ctx, "github_actions", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	newTask := newRows[1]
	current, err := store.Get(ctx, old.ID)
	if err != nil || current.State != "cancelled" {
		t.Fatalf("running task should be superseded: %+v, %v", current, err)
	}
	if err := store.SetState(ctx, old, "succeeded", "late completion"); err != ErrTaskSuperseded {
		t.Fatalf("late completion must be rejected, got %v", err)
	}
	next, ok, err := store.Next(ctx, "A")
	if err != nil || !ok || next.ID != newTask.ID {
		t.Fatalf("newest task should be next: %+v, %v, %v", next, ok, err)
	}
}

func TestInterruptedTaskIsNotAutomaticallySuperseded(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "interrupted-latest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	oldRows, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "app", Action: "update"},
	})
	if err != nil {
		t.Fatal(err)
	}
	old := oldRows[1]
	if err := store.SetState(ctx, old, "running", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetState(ctx, old, "interrupted", "远端结果未知"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enqueue(ctx, "manual", []TaskTarget{
		{HostID: "A", SourceID: "source", AppID: "app", Action: "update"},
	}); err != nil {
		t.Fatal(err)
	}
	current, err := store.Get(ctx, old.ID)
	if err != nil || current.State != "interrupted" {
		t.Fatalf("uncertain remote operation must remain interrupted: %+v, %v", current, err)
	}
	if _, ok, err := store.Next(ctx, "A"); err != nil || ok {
		t.Fatalf("interrupted task must continue blocking host queue, ok=%v err=%v", ok, err)
	}
}

func TestLegacyTaskStateSchemaMigratesToCancelled(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "legacy-task-schema.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.db.ExecContext(ctx, `CREATE TABLE execution_tasks (
		id TEXT PRIMARY KEY,
		parent_id TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL CHECK(type IN ('host','application')),
		host_id TEXT NOT NULL,
		source_id TEXT NOT NULL DEFAULT '',
		app_id TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL,
		trigger_source TEXT NOT NULL,
		state TEXT NOT NULL CHECK(state IN ('queued','preparing','running','succeeded','failed','interrupted')),
		message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		started_at TEXT NOT NULL DEFAULT '',
		finished_at TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.db.ExecContext(ctx, `INSERT INTO execution_tasks
		(id,parent_id,type,host_id,source_id,app_id,action,trigger_source,state,message,created_at,started_at,finished_at)
		VALUES('legacy-task','legacy-parent','application','A','source','app','update','manual','queued','','2026-11-08T00:00:00Z','','')`)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	task, err := store.Get(ctx, "legacy-task")
	if err != nil || task.State != "queued" {
		t.Fatalf("legacy task should survive schema migration: %+v, %v", task, err)
	}
	if err := store.SetState(ctx, task, "cancelled", "migration test"); err != nil {
		t.Fatal(err)
	}
	task, err = store.Get(ctx, "legacy-task")
	if err != nil || task.State != "cancelled" {
		t.Fatalf("migrated schema must accept cancelled state: %+v, %v", task, err)
	}
}
