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

type Service struct {
	ctx     context.Context
	store   *storage.TaskStore
	catalog *catalog.Service
	hosts   *hosts.Service

	mu      sync.Mutex
	running map[string]bool
	wg      sync.WaitGroup
}

func New(ctx context.Context, store *storage.TaskStore, catalogSvc *catalog.Service, hostSvc *hosts.Service) (*Service, error) {
	if err := store.RecoverPreparation(ctx); err != nil {
		return nil, err
	}
	return &Service{ctx: ctx, store: store, catalog: catalogSvc, hosts: hostSvc,
		running: make(map[string]bool)}, nil
}

func (s *Service) Enqueue(ctx context.Context, trigger string, targets []storage.TaskTarget) ([]storage.Task, error) {
	return s.store.Enqueue(ctx, trigger, targets)
}

func (s *Service) List(ctx context.Context) ([]storage.Task, error) {
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
		slog.Error("任务状态写入失败", "task_id", task.ID, "error", err)
		return false
	}
	return true
}

func (s *Service) process(task storage.Task) {
	if task.SourceID == storage.NativeAgentSourceID && task.AppID == storage.NativeAgentAppID {
		s.processNativeAgent(task)
		return
	}
	// running 任务可能在控制端重启前已下发。只恢复状态轮询，不重复执行更新。
	if task.State == "running" {
		s.monitor(task, "")
		return
	}
	if s.ctx.Err() != nil {
		return
	}
	if !s.set(task, "preparing", "") {
		return
	}
	pkg, err := s.catalog.BuildCandidateBundle(s.ctx, task.SourceID, task.AppID)
	if err != nil {
		s.fail(task, "当前应用来源不可用或清单验证失败")
		return
	}
	if err := s.hosts.RequireManifestVersion(s.ctx, task.HostID, pkg.SchemaVersion); err != nil {
		s.fail(task, "目标 Agent 不支持该应用清单，或连接不可用")
		return
	}
	if _, err := s.hosts.CacheBundle(s.ctx, task.HostID, pkg.SHA256, pkg.ZIP); err != nil {
		if s.ctx.Err() == nil {
			s.fail(task, "向目标主机传送更新包失败")
		}
		return
	}
	if s.ctx.Err() != nil {
		return
	}
	// 先持久化 running，再发命令。失联或重启后绝不盲目重复执行有副作用的更新。
	if !s.set(task, "running", "") {
		return
	}
	raw, err := s.hosts.AppAction(s.ctx, task.HostID, "update", map[string]string{
		"source_id": task.SourceID, "app_id": task.AppID, "bundle_sha256": pkg.SHA256,
	})
	if err != nil {
		var agentErr *hosts.AppError
		if errors.As(err, &agentErr) {
			if agentErr.Status == http.StatusConflict {
				// Agent 的现有主机级独占锁依然有效。未接受的任务返回队列重试。
				s.set(task, "queued", "主机正在执行其他操作，等待重试")
			} else {
				// 明确的业务拒绝并未启动脚本，不能无限等待一个不存在的任务。
				s.fail(task, "Agent 拒绝应用更新请求")
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
	s.monitor(task, pkg.SHA256)
}

func (s *Service) fail(task storage.Task, reason string) {
	if s.ctx.Err() == nil {
		s.set(task, "failed", reason)
	}
}

func (s *Service) monitor(task storage.Task, digest string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	// 启动后立即查一次，后续以固定间隔轮询，直到收到实际完成结果。
	everSeenRunning := false
	missingCount := 0
	for {
		if s.ctx.Err() != nil {
			return
		}
		raw, err := s.hosts.AppAction(s.ctx, task.HostID, "status", map[string]any{
			"apps":            []map[string]string{{"source_id": task.SourceID, "app_id": task.AppID}},
			"include_managed": false,
		})
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
		case <-s.ctx.Done():
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

func (s *Service) nativeAgentSnapshot(hostID string) (nativeAgentBaseline, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
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

func (s *Service) processNativeAgent(task storage.Task) {
	if task.State == "running" {
		s.monitorNativeAgent(task)
		return
	}
	if s.ctx.Err() != nil || !s.set(task, "preparing", "") {
		return
	}
	before, err := s.nativeAgentSnapshot(task.HostID)
	if err != nil {
		s.fail(task, "无法读取 Agent 更新前状态")
		return
	}
	baseline, err := json.Marshal(before)
	if err != nil {
		s.fail(task, "无法准备 Agent 更新前状态")
		return
	}
	// 先持久化 running 和基线，再下发一次固定的 Release 强制更新指令。
	// 更新失败不能退回 queued，否则网络丢包可能导致重复更新。
	task.Message = nativeBaselinePrefix + string(baseline)
	if !s.set(task, "running", task.Message) {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 12*time.Second)
	raw, err := s.hosts.NativeAgentUpdate(ctx, task.HostID, "force-update")
	cancel()
	if err != nil {
		if errors.Is(err, hosts.ErrAgentUpdateBusy) {
			s.set(task, "queued", "Agent 正在执行其他操作，等待重试")
			return
		}
		if errors.Is(err, hosts.ErrAgentUpdateUnsupported) ||
			errors.Is(err, hosts.ErrAgentUpdateEnvironment) ||
			errors.Is(err, hosts.ErrAgentUpdateUnauthorized) {
			s.fail(task, "目标 Agent 拒绝原生更新，请核对版本、配对和部署环境")
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
	s.monitorNativeAgent(task)
}

func (s *Service) monitorNativeAgent(task storage.Task) {
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
		if s.ctx.Err() != nil {
			return
		}
		after, err := s.nativeAgentSnapshot(task.HostID)
		if err == nil && after.StartedAt != before.StartedAt {
			if after.Commit != "" && after.Commit != "unknown" && after.Commit != before.Commit {
				s.set(task, "succeeded", "")
			} else {
				s.set(task, "interrupted", "Agent 已重启，但构建 Commit 未变化或不可验证；请核对 Release 和更新日志")
			}
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case <-deadline.C:
			s.set(task, "interrupted", "10 分钟内未确认 Agent 新版本重启，请检查原生更新日志")
			return
		case <-ticker.C:
		}
	}
}
