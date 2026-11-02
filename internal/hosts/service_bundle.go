package hosts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// CacheBundle 在已经配对且 TLS SPKI 固定的 Agent 上缓存不可变 ZIP 字节。
// 仅执行 HEAD/PUT 和回执检查，不提供脚本执行、解压或安装能力。
// 调用方须先完成固定 Git Commit、Schema 和跨文件语义校验。
func (s *Service) CacheBundle(ctx context.Context, hostID, expectedSHA256 string, archive []byte) (BundleReceipt, error) {
	if !uuidRE.MatchString(hostID) || !hashRE.MatchString(expectedSHA256) {
		return BundleReceipt{}, errors.New("主机或候选包摘要格式不合法")
	}
	if len(archive) == 0 || len(archive) > MaxBundleBytes {
		return BundleReceipt{}, errors.New("候选包超过 Agent 接收尺寸限制")
	}
	computed := sha256.Sum256(archive)
	if hex.EncodeToString(computed[:]) != expectedSHA256 {
		return BundleReceipt{}, errors.New("上传的候选包与构建摘要不一致")
	}
	host, err := s.hosts.Get(ctx, hostID)
	if err != nil {
		return BundleReceipt{}, err
	}
	if host.AgentID == "" || host.SPKISHA256 == "" {
		return BundleReceipt{}, errors.New("目标主机尚未绑定有效 Agent 身份")
	}
	token, err := s.hosts.Credentials(ctx, host)
	if err != nil {
		return BundleReceipt{}, errors.New("目标 Agent 授权凭据不可用")
	}
	client, err := NewClient(host.Endpoint, host.AgentID, host.SPKISHA256)
	if err != nil {
		return BundleReceipt{}, err
	}
	has, err := client.HasBundle(ctx, s.db.ControllerID(), token, expectedSHA256)
	if err != nil {
		return BundleReceipt{}, err
	}
	if has {
		return BundleReceipt{
			AgentID: host.AgentID, SHA256: expectedSHA256, SizeBytes: int64(len(archive)), AlreadyExists: true,
		}, nil
	}
	receipt, err := client.UploadBundle(ctx, s.db.ControllerID(), token, archive)
	if err != nil {
		return BundleReceipt{}, err
	}
	has, err = client.HasBundle(ctx, s.db.ControllerID(), token, expectedSHA256)
	if err != nil || !has {
		return BundleReceipt{}, errors.New("Agent 上传回执后没有确认缓存可读")
	}
	return receipt, nil
}
