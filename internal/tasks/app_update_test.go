package tasks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestAppUpdateRequiresAcknowledgedDelivery(t *testing.T) {
	const agentID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const source = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	const application = "demo"
	const beforeReceipt = "before_receipt"
	const afterReceipt = "after_receipt"
	digest := strings.Repeat("1", 64)
	for _, tc := range []struct {
		name              string
		receipt           string
		statuses          []string
		restart           string
		wrongReceiptID    bool
		wrongStatusField  string
		rejection         hosts.AppError
		statusUnsupported bool
		wantState         string
		wantPolls         int32
		wantMessage       string
	}{
		{name: "受理后立即完成", receipt: "running", statuses: []string{"succeeded"}, wantState: "succeeded", wantPolls: 1},
		{name: "本次失败且被同包强更成功覆盖", receipt: "running", statuses: []string{"running", "failed"}, wantState: "failed", wantPolls: 2},
		{name: "未受理且仅有相同包的旧成功记录", wantState: "interrupted"},
		{name: "更新回执返回相同包的旧结果", receipt: "succeeded", wantState: "interrupted"},
		{name: "更新回执的执行编号不匹配", receipt: "running", wrongReceiptID: true, wantState: "interrupted"},
		{name: "未收回执重启", restart: beforeReceipt, wantState: "interrupted"},
		{name: "已收回执重启后确认成功", receipt: "running", statuses: []string{"succeeded"}, restart: afterReceipt, wantState: "succeeded", wantPolls: 1},
		{name: "已收回执重启后读到本次失败", receipt: "running", statuses: []string{"failed"}, restart: afterReceipt, wantState: "failed", wantPolls: 1},
		{name: "精确记录丢失", receipt: "running", wantState: "interrupted", wantPolls: 1},
		{name: "同包成功记录编号不同", receipt: "running", statuses: []string{"succeeded"}, wrongStatusField: "operation_id", wantState: "interrupted", wantPolls: 1},
		{name: "已收回执重启后包摘要不同", receipt: "running", statuses: []string{"succeeded"}, restart: afterReceipt, wrongStatusField: "bundle_sha256", wantState: "interrupted", wantPolls: 1},
		{name: "恢复时旧 Agent 不认识执行编号", receipt: "running", restart: afterReceipt, statusUnsupported: true, wantState: "interrupted", wantPolls: 1},
		{name: "旧 Agent 拒绝带编号的更新", rejection: hosts.AppError{Status: 400, Code: "invalid_request"}, wantState: "failed", wantMessage: "升级"},
		{name: "已有更新占用主机", rejection: hosts.AppError{Status: 409, Code: "app_busy"}, wantState: "queued"},
		{name: "永久归属冲突放行其他应用", rejection: hosts.AppError{Status: 409, Code: "app_conflict"}, wantState: "failed"},
		{name: "未分类冲突不重试", rejection: hosts.AppError{Status: 409, Code: "app_action"}, wantState: "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			workerCtx, stopWorker := context.WithCancel(ctx)
			defer stopWorker()
			var updates, polls atomic.Int32
			var taskID string
			record := func(id, result string) map[string]string {
				return map[string]string{"operation_id": id, "source_id": source, "app_id": application,
					"bundle_sha256": digest, "action": "update", "result": result}
			}
			agent := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-EHA-Agent-ID", agentID)
				switch r.URL.Path {
				case "/v1/apps/update":
					updates.Add(1)
					var command map[string]string
					if err := json.NewDecoder(r.Body).Decode(&command); err != nil || command["source_id"] != source ||
						command["app_id"] != application || command["bundle_sha256"] != digest || command["operation_id"] != taskID {
						t.Errorf("更新命令的目标、包或编号不匹配：%+v，%v", command, err)
					}
					if tc.rejection.Status != 0 {
						w.WriteHeader(tc.rejection.Status)
						_ = json.NewEncoder(w).Encode(map[string]string{"code": tc.rejection.Code})
						return
					}
					if tc.receipt == "" {
						// Agent 在受理前断开连接，已有的状态记录仍然属于上次更新。
						panic(http.ErrAbortHandler)
					}
					id := command["operation_id"]
					if tc.wrongReceiptID {
						id = "other"
					}
					_ = json.NewEncoder(w).Encode(record(id, tc.receipt))
				case "/v1/apps/status":
					var query struct {
						OperationID string              `json:"operation_id"`
						Apps        []map[string]string `json:"apps"`
					}
					if err := json.NewDecoder(r.Body).Decode(&query); err != nil || len(query.Apps) != 1 ||
						query.Apps[0]["source_id"] != source || query.Apps[0]["app_id"] != application {
						t.Errorf("状态请求的应用目标不匹配：%+v，%v", query, err)
					}
					if tc.restart == afterReceipt && workerCtx.Err() == nil {
						// 首次轮询说明回执已经处理，此刻模拟控制端退出。
						stopWorker()
						return
					}
					poll := int(polls.Add(1))
					if query.OperationID != taskID {
						// 普通查询的最新记录属于后来成功的同包强更，与本次任务的终态无关。
						t.Errorf("轮询使用了错误的执行编号：%q", query.OperationID)
						_ = json.NewEncoder(w).Encode(map[string]any{"apps": []map[string]string{record("", "succeeded")}})
						return
					}
					if tc.statusUnsupported {
						w.WriteHeader(http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]string{"code": "invalid_request"})
						return
					}
					rows := []map[string]string{}
					if len(tc.statuses) > 0 {
						row := record(query.OperationID, tc.statuses[min(poll, len(tc.statuses))-1])
						if tc.wrongStatusField != "" {
							row[tc.wrongStatusField] = "other"
						}
						rows = append(rows, row)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"apps": rows})
				default:
					http.NotFound(w, r)
				}
			}))
			defer agent.Close()

			dbPath := filepath.Join(t.TempDir(), "tasks.db")
			db, svc := openAppUpdateTestService(t, ctx, dbPath)
			defer db.Close()
			hostStore, err := storage.NewHostStore(ctx, db)
			if err != nil {
				t.Fatal(err)
			}
			host, err := svc.hosts.Register(ctx, "测试主机", "http://"+agent.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			if err := hostStore.SavePairing(ctx, host, agentID, strings.Repeat("2", 64), strings.Repeat("3", 64)); err != nil {
				t.Fatal(err)
			}
			_, err = svc.store.Enqueue(ctx, "manual", []storage.TaskTarget{
				{HostID: host.ID, SourceID: source, AppID: application, Action: "update"},
				{HostID: host.ID, SourceID: source, AppID: "next", Action: "update"},
			})
			if err != nil {
				t.Fatal(err)
			}
			task, ok, err := svc.store.Next(ctx, host.ID)
			if err != nil || !ok || task.AppID != application {
				t.Fatalf("首个任务 = %+v，err = %v", task, err)
			}
			taskID = task.ID
			agent.Start()
			if tc.restart == beforeReceipt {
				if err := svc.store.SetState(ctx, task, "running", ""); err != nil {
					t.Fatal(err)
				}
			} else {
				svc.dispatchUpdate(workerCtx, task, digest)
			}
			if tc.restart != "" {
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				db, svc = openAppUpdateTestService(t, ctx, dbPath)
				defer db.Close()
				svc.schedule()
				svc.Wait()
			}
			task, err = svc.store.Get(ctx, task.ID)
			if err != nil || task.State != tc.wantState || !strings.Contains(task.Message, tc.wantMessage) {
				t.Fatalf("任务状态 = %q，消息 = %q，预期 %q / %q，err = %v", task.State, task.Message, tc.wantState, tc.wantMessage, err)
			}
			wantUpdates := int32(1)
			if tc.restart == beforeReceipt {
				wantUpdates = 0
			}
			if updates.Load() != wantUpdates || polls.Load() != tc.wantPolls {
				t.Fatalf("更新/查询次数 = %d/%d，预期 %d/%d", updates.Load(), polls.Load(), wantUpdates, tc.wantPolls)
			}
			next, ok, err := svc.store.Next(ctx, host.ID)
			wantNext := tc.wantState != "interrupted"
			wantApp := "next"
			if tc.wantState == "queued" {
				wantApp = application
			}
			if err != nil || ok != wantNext || (ok && next.AppID != wantApp) {
				t.Fatalf("后续任务 = %+v，可调度 = %v，预期 %v，err = %v", next, ok, wantNext, err)
			}
		})
	}
}

func openAppUpdateTestService(t *testing.T, ctx context.Context, path string) (*storage.Store, *Service) {
	t.Helper()
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hostSvc, err := hosts.NewService(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewTaskStore(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := New(ctx, store, nil, hostSvc)
	if err != nil {
		t.Fatal(err)
	}
	return db, svc
}
