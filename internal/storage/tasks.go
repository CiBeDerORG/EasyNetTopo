package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Task 是 EasyNetTopo 的调度记录，不是远程 Agent 自己的任务 ID。
type Task struct {
	ID            string `json:"id"`
	ParentID      string `json:"parent_id,omitempty"`
	Type          string `json:"type"`
	HostID        string `json:"host_id"`
	SourceID      string `json:"source_id,omitempty"`
	AppID         string `json:"app_id,omitempty"`
	Action        string `json:"action"`
	TriggerSource string `json:"trigger_source"`
	State         string `json:"state"`
	Message       string `json:"message,omitempty"`
	CreatedAt     string `json:"created_at"`
	StartedAt     string `json:"started_at,omitempty"`
	FinishedAt    string `json:"finished_at,omitempty"`
}

type TaskTarget struct {
	HostID   string
	SourceID string
	AppID    string
	Action   string
}

type TaskStore struct{ db *sql.DB }

func NewTaskStore(ctx context.Context, store *Store) (*TaskStore, error) {
	for _, query := range []string{
		`CREATE TABLE IF NOT EXISTS execution_tasks (
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
		)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_tasks_queue ON execution_tasks(host_id,type,state,created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_tasks_parent ON execution_tasks(parent_id)`,
	} {
		if _, err := store.db.ExecContext(ctx, query); err != nil {
			return nil, fmt.Errorf("初始化任务表: %w", err)
		}
	}
	return &TaskStore{db: store.db}, nil
}

// Enqueue 原子地保存主机父任务及其全部应用子任务。
// 不联系远端主机，即使主机暂时离线也能立即返回。
func (s *TaskStore) Enqueue(ctx context.Context, trigger string, targets []TaskTarget) ([]Task, error) {
	if trigger == "" || len(targets) == 0 || len(targets) > 64 {
		return nil, errors.New("任务来源或目标数量无效")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	parents := make(map[string]string)
	result := make([]Task, 0, len(targets)*2)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	insert := func(t Task) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO execution_tasks
			(id,parent_id,type,host_id,source_id,app_id,action,trigger_source,state,message,created_at,started_at,finished_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			t.ID, t.ParentID, t.Type, t.HostID, t.SourceID, t.AppID, t.Action,
			t.TriggerSource, t.State, t.Message, t.CreatedAt, t.StartedAt, t.FinishedAt)
		return err
	}
	seen := make(map[string]bool)
	for _, target := range targets {
		if target.HostID == "" || target.SourceID == "" || target.AppID == "" || target.Action != "update" {
			return nil, errors.New("不支持的任务目标")
		}
		key := target.HostID + "|" + target.SourceID + "|" + target.AppID
		if seen[key] {
			continue
		}
		seen[key] = true
		parentID, ok := parents[target.HostID]
		if !ok {
			id, e := uuid()
			if e != nil {
				return nil, e
			}
			parentID = "HT-" + id
			parents[target.HostID] = parentID
			parent := Task{ID: parentID, Type: "host", HostID: target.HostID, Action: "update",
				TriggerSource: trigger, State: "queued", CreatedAt: stamp}
			if e := insert(parent); e != nil {
				return nil, e
			}
			result = append(result, parent)
		}
		id, e := uuid()
		if e != nil {
			return nil, e
		}
		task := Task{ID: "AT-" + id, ParentID: parentID, Type: "application", HostID: target.HostID,
			SourceID: target.SourceID, AppID: target.AppID, Action: target.Action,
			TriggerSource: trigger, State: "queued", CreatedAt: stamp}
		if e := insert(task); e != nil {
			return nil, e
		}
		result = append(result, task)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

const taskColumns = "id,parent_id,type,host_id,source_id,app_id,action,trigger_source,state,message,created_at,started_at,finished_at"

func scanTask(row interface{ Scan(...any) error }) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.ParentID, &t.Type, &t.HostID, &t.SourceID, &t.AppID,
		&t.Action, &t.TriggerSource, &t.State, &t.Message, &t.CreatedAt, &t.StartedAt, &t.FinishedAt)
	return t, err
}

// ListLatest 包括最近完成记录与全部待执行任务，供 UI 查找当前任务与最近执行时间。
func (s *TaskStore) ListLatest(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+taskColumns+" FROM execution_tasks ORDER BY rowid DESC LIMIT 2000")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (s *TaskStore) PendingHosts(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT host_id FROM execution_tasks
		WHERE type='application' AND state IN ('queued','preparing','running')
		ORDER BY host_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hosts []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		hosts = append(hosts, id)
	}
	return hosts, rows.Err()
}

// Next 优先恢复主机正在执行的任务，再按入队顺序执行其余任务。
// 如果存在中断任务，必须先人工核对，不能执行后续变更。
func (s *TaskStore) Next(ctx context.Context, hostID string) (Task, bool, error) {
	var interrupted int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_tasks
		WHERE host_id=? AND type='application' AND state='interrupted'`, hostID).Scan(&interrupted); err != nil {
		return Task{}, false, err
	}
	if interrupted > 0 {
		return Task{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, "SELECT "+taskColumns+` FROM execution_tasks
		WHERE host_id=? AND type='application' AND state IN ('queued','preparing','running')
		ORDER BY CASE WHEN state='running' THEN 0 WHEN state='preparing' THEN 1 ELSE 2 END, rowid LIMIT 1`, hostID)
	task, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, false, nil
	}
	return task, err == nil, err
}

// SetState 与父任务的汇总状态在同一事务提交。
func (s *TaskStore) SetState(ctx context.Context, task Task, state, message string) error {
	if task.Type != "application" {
		return errors.New("只能直接更新应用任务")
	}
	switch state {
	case "queued", "preparing", "running", "succeeded", "failed", "interrupted":
	default:
		return errors.New("未知任务状态")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	finished := state == "succeeded" || state == "failed" || state == "interrupted"
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	finishAt := ""
	if finished {
		finishAt = now
	}
	_, err = tx.ExecContext(ctx, `UPDATE execution_tasks SET state=?, message=?,
		started_at=CASE WHEN started_at='' AND ? IN ('preparing','running') THEN ? ELSE started_at END,
		finished_at=CASE WHEN ?='' THEN '' ELSE ? END WHERE id=?`,
		state, message, state, now, finishAt, finishAt, task.ID)
	if err != nil {
		return err
	}
	var total, done, failures, interruptions, active int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN state IN ('succeeded','failed','interrupted') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN state='failed' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN state='interrupted' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN state IN ('running','preparing') THEN 1 ELSE 0 END),0)
		FROM execution_tasks WHERE parent_id=?`, task.ParentID).
		Scan(&total, &done, &failures, &interruptions, &active)
	if err != nil {
		return err
	}
	parentState, parentFinished := "queued", ""
	if total > 0 && done == total {
		parentState = "succeeded"
		if interruptions > 0 {
			parentState = "interrupted"
		} else if failures > 0 {
			parentState = "failed"
		}
		parentFinished = now
	} else if active > 0 || done > 0 {
		parentState = "running"
	}
	_, err = tx.ExecContext(ctx, `UPDATE execution_tasks SET state=?,
		started_at=CASE WHEN started_at='' AND ?='running' THEN ? ELSE started_at END,
		finished_at=? WHERE id=?`,
		parentState, parentState, now, parentFinished, task.ParentID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RecoverPreparation 仅回滚尚未向 Agent 发出更新命令的准备阶段。
func (s *TaskStore) RecoverPreparation(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE execution_tasks SET state='queued',message='服务重启后重新准备'
		WHERE type='application' AND state='preparing'`)
	return err
}

func (s *TaskStore) Get(ctx context.Context, id string) (Task, error) {
	return scanTask(s.db.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM execution_tasks WHERE id=?", id))
}

// ResolveInterrupted 仅在管理员确认后把无法判定的远端任务关闭为失败。
// 它不把未确认的部署伪造成成功；后续排队任务才可恢复。
func (s *TaskStore) ResolveInterrupted(ctx context.Context, id string) error {
	task, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if task.Type != "application" || task.State != "interrupted" {
		return errors.New("只有中断的应用任务可以人工关闭")
	}
	return s.SetState(ctx, task, "failed", "管理员确认远端已停止，关闭中断任务（未证明更新成功）")
}
