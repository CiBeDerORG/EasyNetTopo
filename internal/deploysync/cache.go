package deploysync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
)

// CandidateBuilder 只能是从固定 Git Commit 构建、且通过 Deploy v1 结构与语义校验的实现。
// 本接口不代表 Agent 对部署包的可信执行批准。
type CandidateBuilder interface {
	BuildCandidateBundle(context.Context, string, string) (packagebuilder.Artifact, error)
}

// BundleCache 通过已配对的 Agent 缓存字节；不得接受任意页面上传的 ZIP。
type BundleCache interface {
	CacheBundle(context.Context, string, string, []byte) (hosts.BundleReceipt, error)
}

// Result 是纯缓存回执；不包含任务编号、安装状态或执行授权。
type Result struct {
	SourceID      string `json:"source_id"`
	ApplicationID string `json:"application_id"`
	HostID        string `json:"host_id"`
	AgentID       string `json:"agent_id"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	AlreadyExists bool   `json:"already_exists"`
}

// EnsureCandidateCached 只连接“固定快照构建候选包”和“主机认证缓存”，不涉及执行路径。
// 在真实发布者签名/Agent 主机本地批准之前，绝不在 HTTP API 暴露为部署动作。
func EnsureCandidateCached(ctx context.Context, builder CandidateBuilder, cache BundleCache, hostID, sourceID, appID string) (Result, error) {
	if builder == nil || cache == nil {
		return Result{}, errors.New("候选包构建器或 Agent 缓存服务未配置")
	}
	candidate, err := builder.BuildCandidateBundle(ctx, sourceID, appID)
	if err != nil {
		return Result{}, err
	}
	if candidate.SizeBytes <= 0 || candidate.SizeBytes != int64(len(candidate.ZIP)) || len(candidate.ZIP) > hosts.MaxBundleBytes {
		return Result{}, errors.New("候选包长度与实际 ZIP 字节不一致")
	}
	digest := sha256.Sum256(candidate.ZIP)
	if candidate.SHA256 != hex.EncodeToString(digest[:]) {
		return Result{}, errors.New("候选包声明的摘要与构建字节不一致")
	}
	receipt, err := cache.CacheBundle(ctx, hostID, candidate.SHA256, candidate.ZIP)
	if err != nil {
		return Result{}, err
	}
	if receipt.AgentID == "" || receipt.SHA256 != candidate.SHA256 || receipt.SizeBytes != candidate.SizeBytes {
		return Result{}, errors.New("Agent 回执与提交字节不一致")
	}
	return Result{
		SourceID: sourceID, ApplicationID: appID, HostID: hostID, AgentID: receipt.AgentID,
		SHA256: receipt.SHA256, SizeBytes: receipt.SizeBytes, AlreadyExists: receipt.AlreadyExists,
	}, nil
}
