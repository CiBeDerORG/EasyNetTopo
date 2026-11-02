package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
)

var bundleSourceID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// pinnedBundleRepository 从已经保留的 Git Commit 读取对象，绝不 fetch 或 checkout。
func pinnedBundleRepository(ctx context.Context, cache, revision string) (*gitRepository, error) {
	if !commitID.MatchString(revision) {
		return nil, errors.New("固定 Commit 格式不合法")
	}
	ref := "refs/easynettopo/catalog/" + revision
	result, err := gitCommand(ctx, 256, "--git-dir="+cache, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil || strings.TrimSpace(string(result)) != revision {
		return nil, errors.New("固定 Git 对象未保留或与快照不符")
	}
	tree, err := gitCommand(ctx, 4*1024*1024, "--git-dir="+cache, "ls-tree", "-r", "-z", "--full-tree", revision)
	if err != nil {
		return nil, errors.New("固定 Git 树不可读取")
	}
	objects := make(map[string]gitObject)
	for _, row := range bytes.Split(tree, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		if len(objects) >= 10000 {
			return nil, errors.New("Git 树条目数量超过上限")
		}
		fields := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(fields) != 2 {
			return nil, errors.New("Git 树条目格式不合法")
		}
		attrs := strings.Fields(string(fields[0]))
		if len(attrs) != 3 || attrs[1] != "blob" {
			continue
		}
		objects[string(fields[1])] = gitObject{Mode: attrs[0], Object: attrs[2]}
	}
	return &gitRepository{path: cache, revision: revision, objects: objects}, nil
}

func (r *gitRepository) bundleContents(ctx context.Context, file string) ([]byte, error) {
	entry, exists := r.objects[file]
	if !exists || (entry.Mode != "100644" && entry.Mode != "100755") || !commitID.MatchString(entry.Object) {
		return nil, errors.New("候选包依赖不存在或不是普通 Git 文件")
	}
	result, err := gitCommand(ctx, packagebuilder.MaxFileBytes+1, "--git-dir="+r.path, "cat-file", "blob", entry.Object)
	if err != nil || len(result) > packagebuilder.MaxFileBytes {
		return nil, errors.New("候选包文件读取失败或超出上限")
	}
	return result, nil
}

// BuildCandidateBundle 只在后端内存中构建不可执行的候选包，不上送、不安装。
// previewManifest 不是 Deploy 完整 Schema+语义验证；本方法的结果不能作为可信发布授权。
func (s *Service) BuildCandidateBundle(ctx context.Context, sourceID, applicationID string) (packagebuilder.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !bundleSourceID.MatchString(sourceID) || !appID.MatchString(applicationID) {
		return packagebuilder.Artifact{}, errors.New("应用来源或 ID 格式错误")
	}
	source, err := s.store.CatalogSourceByID(ctx, sourceID)
	if err != nil {
		return packagebuilder.Artifact{}, err
	}
	if !commitID.MatchString(source.CurrentRevision) {
		return packagebuilder.Artifact{}, errors.New("来源尚无可用的固定版本")
	}
	apps, err := s.store.CatalogApps(ctx)
	if err != nil {
		return packagebuilder.Artifact{}, err
	}
	var appRevision, digest string
	for _, app := range apps {
		if app.SourceID == sourceID && app.AppID == applicationID {
			appRevision, digest = app.Revision, app.ManifestSHA256
			break
		}
	}
	if appRevision == "" || appRevision != source.CurrentRevision {
		return packagebuilder.Artifact{}, errors.New("应用与来源快照版本不匹配")
	}
	repository, err := pinnedBundleRepository(ctx, filepath.Join(s.cacheDir, source.ID+".git"), appRevision)
	if err != nil {
		return packagebuilder.Artifact{}, err
	}
	manifestRaw, err := repository.contents(ctx, applicationID+"/app.yaml")
	if err != nil {
		return packagebuilder.Artifact{}, errors.New("固定版本 Manifest 不可读取")
	}
	if err := ValidateManifestSchemaV1(manifestRaw); err != nil {
		return packagebuilder.Artifact{}, fmt.Errorf("固定版本 Manifest 未通过完整 Deploy v1 JSON Schema 校验: %w", err)
	}
	manifest, actualDigest, err := previewManifest(applicationID, manifestRaw, repository.objects)
	if err != nil || actualDigest != digest {
		return packagebuilder.Artifact{}, errors.New("Manifest 与已索引快照不一致")
	}
	resources := make([]packagebuilder.Resource, 0, len(manifest.Package.Files))
	for _, item := range manifest.Package.Files {
		blob, err := repository.bundleContents(ctx, item.Source)
		if err != nil {
			return packagebuilder.Artifact{}, fmt.Errorf("无法读取固定版本的包资源: %w", err)
		}
		resources = append(resources, packagebuilder.Resource{
			Source: item.Source, Destination: item.Destination, Mode: item.Mode, Content: blob,
		})
	}
	return packagebuilder.Build(packagebuilder.Identity{
		SourceID: sourceID, ApplicationID: applicationID, GitRevision: appRevision, ManifestSHA256: digest,
	}, manifestRaw, resources)
}
