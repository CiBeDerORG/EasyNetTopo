// Package tasks implements a durable, per-host serial dispatcher.
// Distinct hosts execute concurrently; an Agent never receives overlapping app updates.
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

type activeTask struct {
	id     string
	cancel context.CancelFunc
}

type Service struct {
	ctx     context.Context
	store   *storage.TaskStore
	catalog *catalog.Service
	hosts   *hosts.Service

	mu      sync.Mutex
	running map[string]bool
	active  map[string]activeTask
	wg      sync.WaitGroup
}

func New(ctx context.Context, store *storage.TaskStore, catalogSvc *catalog.Service, hostSvc *hosts.Service) (*Service, error) {
	if err := store.RecoverPreparation(ctx); err != nil {
		return nil, err
	}
	return &Service{ctx: ctx, store: store, catalog: catalogSvc, hosts: hostSvc,
		running: make(map[string]bool), active: make(map[string]activeTask)}, nil
}

func taskTargetKey(hostID, sourceID, appID string) string {
	return hostID + "\x00" + sourceID + "\x00" + appID
}

func (s *Service) Enqueue(ctx context.Context, trigger string, targets []storage.TaskTarget) ([]storage.Task, error) {
	created, err := s.store.Enqueue(ctx, trigger, targets)
	if err != nil {
		return nil, err
	}
	// 先由存储事务把旧任务标记为 cancelled，再中止对应的本地 Worker。
	// 只按主机和应用完整标识取消，不影响同主机其他应用或其他主机。
	s.mu.Lock()
	for _, target := range targets {
		if active, ok := s.active[taskTargetKey(target.HostID, target.SourceID, target.AppID)]; ok {
			active.cancel()
		}
	}
	s.mu.Unlock()
	return created, nil
}

func (s *Service) List(ctx context.Context) ([]storage.Task, error) {
	if err := s.store.CancelExpiredTemporaryTasks(ctx); err != nil {
		return nil, err
	}
	return s.store.ListLatest(ctx)
}

func (s *Service) ResolveInterrupted(ctx context.Context, id string) error {
	return s.store.ResolveInterrupted(ctx, id)
}

func (s *Service) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(1200 * time.Millisecond)
		defer ticker.Stop()
		for {
			s.schedule()
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) Wait() {
	s.wg.Wait()
}

func (s *Service) schedule() {
	if err := s.store.CancelExpiredTemporaryTasks(s.ctx); err != nil && s.ctx.Err() == nil {
		slog.Warn("清理过期临时主机任务失败", "error", err)
	}
	ids, err := s.store.PendingHosts(s.ctx)
	if err != nil {
		if s.ctx.Err() == nil {
			slog.Warn("查询待调度主机失败", "error", err)
		}
		return
	}
	for _, id := range ids {
		s.mu.Lock()
		if s.running[id] {
			s.mu.Unlock()
			continue
		}
		s.running[id] = true
		s.wg.Add(1)
		s.mu.Unlock()
		go func(hostID string) {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.running, hostID)
				s.mu.Unlock()
			}()
			task, ok, err := s.store.Next(s.ctx, hostID)
			if err != nil || !ok {
				return
			}
			s.process(task)
		}(id)
	}
}

func (s *Service) set(task storage.Task, state, message string) bool {
	if err := s.store.SetState(context.Background(), task, state, message); err != nil {
		if errors.Is(err, storage.ErrTaskSuperseded) {
			return false
		}
		slog.Error("任务状态写入失败", "task_id", task.ID, "error", err)
		return false
	}
	return true
}

func (s *Service) taskActive(task storage.Task) bool {
	current, err := s.store.Get(context.Background(), task.ID)
	if err != nil {
		return false
	}
	switch current.State {
	case "queued", "preparing", "running":
		return true
	default:
		return false
	}
}

func (s *Service) process(task storage.Task) {
	taskCtx, cancel := context.WithCancel(s.ctx)
	key := taskTargetKey(task.HostID, task.SourceID, task.AppID)
	s.mu.Lock()
	if s.active == nil {
		s.active = make(map[string]activeTask)
	}
	s.active[key] = activeTask{id: task.ID, cancel: cancel}
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		if active, ok := s.active[key]; ok && active.id == task.ID {
			delete(s.active, key)
		}
		s.mu.Unlock()
	}()

	if taskCtx.Err() != nil || !s.taskActive(task) {
		return
	}
	if task.SourceID == storage.NativeAgentSourceID && task.AppID == storage.NativeAgentAppID {
		s.processNativeAgent(taskCtx, task)
		return
	}
	// running 任务可能在控制端重启前已下发。只恢复状态轮询，不重复执行更新。
	if task.State == "running" {
		s.monitor(taskCtx, task, "")
		return
	}
	if taskCtx.Err() != nil || !s.set(task, "preparing", "") {
		return
	}
	pkg, err := s.catalog.BuildCandidateBundle(taskCtx, task.SourceID, task.AppID)
	if err != nil {
		s.fail(taskCtx, task, "当前应用来源不可用或清单验证失败")
		return
	}
	if taskCtx.Err() != nil || !s.taskActive(task) {
		return
	}
	if err := s.hosts.RequireManifestVersion(taskCtx, task.HostID, pkg.SchemaVersion); err != nil {
		s.fail(taskCtx, task, "目标 Agent 不支持该应用清单，或连接不可用")
		return
	}
	if taskCtx.Err() != nil || !s.taskActive(task) {
		return
	}
	if _, err := s.hosts.CacheBundle(taskCtx, task.HostID, pkg.SHA256, pkg.ZIP); err != nil {
		if taskCtx.Err() == nil && s.taskActive(task) {
			s.fail(taskCtx, task, "向目标主机传送更新包失败")
		}
		return
	}
	if taskCtx.Err() != nil || !s.taskActive(task) {
		return
	}
	// 先持久化 running，再发命令。失联或重启后绝不盲目重复执行有副作用的更新。
	if !s.set(task, "running", "") {
		return
	}
	if taskCtx.Err() != nil || !s.taskActive(task) {
		return
	}
	raw, err := s.hosts.AppAction(taskCtx, task.HostID, "update", map[string]string{
		"source_id": task.SourceID, "app_id": task.AppID, "bundle_sha256": pkg.SHA256,
	})
	// 新任务可能在请求期间替代本任务。Agent 已受理的远端脚本无法撤回，
	// 但本地 Worker 必须停止；最新任务会受 Agent 独占锁保护并在旧操作结束后重试。
	if taskCtx.Err() != nil || !s.taskActive(task) {
		return
	}
	if err != nil {
		var agentErr *hosts.AppError
		if errors.As(err, &agentErr) {
			if agentErr.Status == http.StatusConflict {
				// Agent 的现有主机级独占锁依然有效。未接受的任务返回队列重试。
				s.set(task, "queued", "主机正在执行其他操作，等待重试")
			} else {
				// 明确的业务拒绝并未启动脚本，不能无限等待一个不存在的任务。
				s.fail(taskCtx, task, "Agent 拒绝应用更新请求")
			}
			return
		}
		// 请求可能已送达，但回应丢失；先用 Agent 实际状态核对，避免重复更新。
		slog.Warn("更新请求未获确认，等待 Agent 状态", "task_id", task.ID, "error", err)
	} else {
		var ack struct {
			Action string `json:"action"`
			Result string `json:"result"`
			Digest string `json:"bundle_sha256"`
		}
		if json.Unmarshal(raw, &ack) != nil || ack.Action != "update" || ack.Digest != pkg.SHA256 {
			s.set(task, "interrupted", "Agent 更新回执与目标任务不匹配，请人工核对")
			return
		}
	}
	s.monitor(taskCtx, task, pkg.SHA256)
}

func (s *Service) fail(ctx context.Context, task storage.Task, reason string) {
	if ctx.Err() == nil && s.taskActive(task) {
		s.set(task, "failed", reason)
	}
}

func (s *Service) monitor(ctx context.Context, task storage.Task, digest string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	// 启动后立即查一次，后续以固定间隔轮询，直到收到实际完成结果。
	everSeenRunning := false
	missingCount := 0
	for {
		if ctx.Err() != nil || !s.taskActive(task) {
			return
		}
		raw, err := s.hosts.AppAction(ctx, task.HostID, "status", map[string]any{
			"apps":            []map[string]string{{"source_id": task.SourceID, "app_id": task.AppID}},
			"include_managed": false,
		})
		if ctx.Err() != nil || !s.taskActive(task) {
			return
		}
		if err == nil {
			var response struct {
				Apps []struct {
					Source string `json:"source_id"`
					App    string `json:"app_id"`
					Digest string `json:"bundle_sha256"`
					Action string `json:"action"`
					Result string `json:"result"`
				} `json:"apps"`
			}
			if json.Unmarshal(raw, &response) == nil {
				matched := false
				for _, row := range response.Apps {
					if row.Source != task.SourceID || row.App != task.AppID || row.Action != "update" {
						continue
					}
					if digest != "" && row.Digest != digest {
						continue
					}
					matched = true
					switch row.Result {
					case "running":
						everSeenRunning = true
					case "succeeded":
						// 没有 Agent 操作唯一标识时，不能把重启前旧的成功回执当作本次成功。
						if digest == "" && !everSeenRunning {
							s.set(task, "interrupted", "控制端重启，Agent 未提供本次更新的唯一执行回执")
						} else {
							s.set(task, "succeeded", "")
						}
						return
					case "failed":
						s.set(task, "failed", "Agent 更新脚本执行失败")
						return
					case "unknown":
						s.set(task, "interrupted", "Agent 上次更新已中断，请人工确认")
						return
					}
				}
				if matched {
					missingCount = 0
				} else {
					missingCount++
				}
				if missingCount >= 5 {
					s.set(task, "interrupted", "无法在 Agent 中确认本次更新记录，请核对后处理")
					return
				}
			}
		}
		// 主机暂时离线不是更新失败。持续轮询，保证同一主机不会运行下一条任务。
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Agent 自更新不经过 Git 应用包传输；只使用已配对的原生维护 API。
// 进程可能在收到 202 后重启，因此必须保留更新前的启动时间和构建 Commit。
type nativeAgentBaseline struct {
	StartedAt string `json:"agent_started_at"`
	Commit    string `json:"agent_commit"`
}

const nativeBaselinePrefix = "agent-baseline:"

// 同一次 Release 重装不会改变 Commit。只要更新指令已受理，已配对
// Agent 返回不同的进程启动标识，就表示更新后的服务已重启并恢复连接。
func nativeAgentRestarted(before, after nativeAgentBaseline) bool {
	if before.StartedAt == "" || after.StartedAt == "" || before.StartedAt == after.StartedAt {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, after.StartedAt)
	return err == nil
}

// 中断时旧版本任务会把基线覆盖成故障描述；使用任务实际启动时间
// 作为兼容性证据，且必须由已配对 Agent 的实时快照证明进程在此之后启动。
func nativeAgentRestartedAfterTask(task storage.Task, startedAt string) bool {
	current, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return false
	}
	started, err := time.Parse(time.RFC3339Nano, task.StartedAt)
	return err == nil && current.After(started)
}

// ResolveNativeInterrupted 仅供已人工核实停止操作、并已通过身份验证
// 取得实时 Agent 快照的请求使用；确认重启就补记成功，否则关闭为失败。
// 无需查询 Docker 应用状态，原生 Agent 并不是一个容器应用。
func (s *Service) ResolveNativeInterrupted(ctx context.Context, id, startedAt string) (bool, error) {
	task, err := s.store.Get(ctx, id)
	if err != nil {
		return false, err
	}
	if task.Type != "application" || task.State != "interrupted" ||
		task.SourceID != storage.NativeAgentSourceID || task.AppID != storage.NativeAgentAppID {
		return false, errors.New("不是可处理的中断原生 Agent 任务")
	}
	if _, err := time.Parse(time.RFC3339Nano, startedAt); err != nil {
		return false, errors.New("当前 Agent 启动标识无效")
	}
	restarted := false
	if strings.HasPrefix(task.Message, nativeBaselinePrefix) {
		var before nativeAgentBaseline
		if json.Unmarshal([]byte(strings.TrimPrefix(task.Message, nativeBaselinePrefix)), &before) == nil {
			restarted = nativeAgentRestarted(before, nativeAgentBaseline{StartedAt: startedAt})
		}
	}
	if !restarted {
		restarted = nativeAgentRestartedAfterTask(task, startedAt)
	}
	if restarted {
		return true, s.store.SetState(ctx, task, "succeeded", "")
	}
	return false, s.store.ResolveInterrupted(ctx, id)
}

func (s *Service) nativeAgentSnapshot(parent context.Context, hostID string) (nativeAgentBaseline, error) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	raw, err := s.hosts.Query(ctx, hostID, "host")
	if err != nil {
		return nativeAgentBaseline{}, err
	}
	var snap nativeAgentBaseline
	if err := json.Unmarshal(raw, &snap); err != nil || snap.StartedAt == "" {
		return nativeAgentBaseline{}, errors.New("Agent 快照缺少启动标识")
	}
	return snap, nil
}

func (s *Service) processNativeAgent(ctx context.Context, task storage.Task) {
	if task.State == "running" {
		s.monitorNativeAgent(ctx, task)
		return
	}
	if ctx.Err() != nil || !s.taskActive(task) || !s.set(task, "preparing", "") {
		return
	}
	before, err := s.nativeAgentSnapshot(ctx, task.HostID)
	if err != nil {
		s.fail(ctx, task, "无法读取 Agent 更新前状态")
		return
	}
	if ctx.Err() != nil || !s.taskActive(task) {
		return
	}
	baseline, err := json.Marshal(before)
	if err != nil {
		s.fail(ctx, task, "无法准备 Agent 更新前状态")
		return
	}
	// 先持久化 running 和基线，再下发一次固定的 Release 强制更新指令。
	// 更新失败不能退回 queued，否则网络丢包可能导致重复更新。
	task.Message = nativeBaselinePrefix + string(baseline)
	if !s.set(task, "running", task.Message) {
		return
	}
	if ctx.Err() != nil || !s.taskActive(task) {
		return
	}
	updateCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	raw, err := s.hosts.NativeAgentUpdate(updateCtx, task.HostID, "force-update")
	cancel()
	if ctx.Err() != nil || !s.taskActive(task) {
		return
	}
	if err != nil {
		if errors.Is(err, hosts.ErrAgentUpdateBusy) {
			s.set(task, "queued", "Agent 正在执行其他操作，等待重试")
			return
		}
		if errors.Is(err, hosts.ErrAgentUpdateUnsupported) ||
			errors.Is(err, hosts.ErrAgentUpdateEnvironment) ||
			errors.Is(err, hosts.ErrAgentUpdateUnauthorized) {
			s.fail(ctx, task, "目标 Agent 拒绝原生更新，请核对版本、配对和部署环境")
			return
		}
		s.set(task, "interrupted", "Agent 更新请求是否已受理无法确认；禁止自动重发")
		return
	}
	var ack struct {
		State  string `json:"state"`
		Action string `json:"action"`
	}
	if json.Unmarshal(raw, &ack) != nil || ack.State != "queued" || ack.Action != "force-update" {
		s.set(task, "interrupted", "Agent 原生更新回执与请求不匹配")
		return
	}
	s.monitorNativeAgent(ctx, task)
}

func (s *Service) monitorNativeAgent(ctx context.Context, task storage.Task) {
	if !s.taskActive(task) {
		return
	}
	if !strings.HasPrefix(task.Message, nativeBaselinePrefix) {
		s.set(task, "interrupted", "无法恢复更新前的 Agent 启动信息，需人工核对")
		return
	}
	var before nativeAgentBaseline
	if json.Unmarshal([]byte(strings.TrimPrefix(task.Message, nativeBaselinePrefix)), &before) != nil ||
		before.StartedAt == "" {
		s.set(task, "interrupted", "Agent 更新前的状态记录损坏，需人工核对")
		return
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(10 * time.Minute)
	defer deadline.Stop()
	for {
		if ctx.Err() != nil || !s.taskActive(task) {
			return
		}
		after, err := s.nativeAgentSnapshot(ctx, task.HostID)
		if ctx.Err() != nil || !s.taskActive(task) {
			return
		}
		if err == nil && nativeAgentRestarted(before, after) {
			s.set(task, "succeeded", "")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			s.set(task, "interrupted", "10 分钟内未确认 Agent 新版本重启，请检查原生更新日志")
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) HasActiveTemporaryTasks(ctx context.Context, hostID string) (bool, error) {
	return s.store.HasActiveTemporaryTasks(ctx, hostID)
}

func (s *Service) EnqueueTemporaryInstall(ctx context.Context, hostID, sourceID, appID, revision, digest string, config map[string]string, bundle []byte) ([]storage.Task, error) {
	h, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if h.HostType != "temporary" || h.ConnectionMode != "reverse" || !s.hosts.TemporaryActive(h) {
		return nil, storage.ErrReverseTaskUnavailable
	}
	return s.store.EnqueueReverseInstall(ctx, hostID, sourceID, appID, revision, digest, config, bundle)
}

func (s *Service) ClaimReverseTask(ctx context.Context, agentID string) (*storage.ReverseTaskPayload, error) {
	return s.store.ClaimReverseTask(ctx, agentID)
}

func (s *Service) AcknowledgeReverseTask(ctx context.Context, agentID, taskID string) (storage.Task, error) {
	return s.store.AcknowledgeReverseTask(ctx, agentID, taskID)
}

func (s *Service) UpdateReverseTaskState(ctx context.Context, agentID, taskID, state, message string) error {
	return s.store.UpdateReverseTaskState(ctx, agentID, taskID, state, message)
}

func (s *Service) ReverseBundle(ctx context.Context, agentID, taskID string) ([]byte, string, error) {
	return s.store.ReverseBundle(ctx, agentID, taskID)
}

func (s *Service) CancelRevokedTemporaryTasks(ctx context.Context, hostID string) error {
	return s.store.CancelRevokedTemporaryTasks(ctx, hostID)
}
