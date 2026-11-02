package deploysync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
)

type stubBuilder struct {
	artifact packagebuilder.Artifact
	err      error
	calls    int
}

func (x *stubBuilder) BuildCandidateBundle(_ context.Context, _, _ string) (packagebuilder.Artifact, error) {
	x.calls++
	return x.artifact, x.err
}

type stubCache struct {
	receipt hosts.BundleReceipt
	err     error
	calls   int
	digest  string
	payload []byte
}

func (x *stubCache) CacheBundle(_ context.Context, _, digest string, contents []byte) (hosts.BundleReceipt, error) {
	x.calls++
	x.digest = digest
	x.payload = append([]byte(nil), contents...)
	return x.receipt, x.err
}

func testedArtifact() packagebuilder.Artifact {
	data := []byte("synthetic immutable candidate bytes")
	hash := sha256.Sum256(data)
	return packagebuilder.Artifact{ZIP: data, SHA256: hex.EncodeToString(hash[:]), SizeBytes: int64(len(data))}
}

func TestEnsureCandidateCachedOnlyTransportsVerifiedBytes(t *testing.T) {
	a := testedArtifact()
	builder := &stubBuilder{artifact: a}
	cache := &stubCache{receipt: hosts.BundleReceipt{
		AgentID: "11111111-1111-4111-8111-111111111111",
		SHA256:  a.SHA256, SizeBytes: a.SizeBytes,
	}}
	result, err := EnsureCandidateCached(context.Background(), builder, cache, "host", "source", "sub2api")
	if err != nil || result.AgentID != cache.receipt.AgentID || result.SHA256 != a.SHA256 ||
		builder.calls != 1 || cache.calls != 1 || cache.digest != a.SHA256 || string(cache.payload) != string(a.ZIP) {
		t.Fatalf("只缓存已验证的候选包失败: %+v %v", result, err)
	}
}

func TestEnsureCandidateCachedFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*stubBuilder, *stubCache)
		calls   int
	}{
		{"构建失败不触发 Agent", func(b *stubBuilder, _ *stubCache) { b.err = errors.New("invalid manifest") }, 0},
		{"大小声明不一致", func(b *stubBuilder, _ *stubCache) { b.artifact.SizeBytes++ }, 0},
		{"摘要不一致", func(b *stubBuilder, _ *stubCache) { b.artifact.SHA256 = strings.Repeat("0", 64) }, 0},
		{"空候选包", func(b *stubBuilder, _ *stubCache) {
			b.artifact.ZIP = nil
			b.artifact.SizeBytes = 0
		}, 0},
		{"Agent 上传失败", func(_ *stubBuilder, c *stubCache) { c.err = errors.New("not paired") }, 1},
		{"Agent 回执摘要不一致", func(_ *stubBuilder, c *stubCache) { c.receipt.SHA256 = strings.Repeat("0", 64) }, 1},
		{"Agent 回执长度不一致", func(_ *stubBuilder, c *stubCache) { c.receipt.SizeBytes++ }, 1},
		{"Agent 回执没有身份", func(_ *stubBuilder, c *stubCache) { c.receipt.AgentID = "" }, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			a := testedArtifact()
			builder := &stubBuilder{artifact: a}
			cache := &stubCache{receipt: hosts.BundleReceipt{
				AgentID: "11111111-1111-4111-8111-111111111111", SHA256: a.SHA256, SizeBytes: a.SizeBytes,
			}}
			test.prepare(builder, cache)
			if _, err := EnsureCandidateCached(context.Background(), builder, cache, "host", "source", "sub2api"); err == nil {
				t.Fatal("不可信的候选包或回执被当作缓存成功")
			}
			if cache.calls != test.calls {
				t.Fatalf("失败路径仍进行了意料之外的 Agent 访问次数：%d", cache.calls)
			}
		})
	}
}
