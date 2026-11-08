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
