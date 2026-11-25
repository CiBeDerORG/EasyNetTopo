package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
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
func (s *Service) BuildCandidateBundle(ctx context.Context, sourceID, applicationID string) (packagebuilder.Artifact, error) {
	return s.BuildUpdateBundle(ctx, sourceID, applicationID, "")
}

// BuildUpdateBundle 对手动更新使用提交时确认的版本，Hook 空版本沿用执行时的目录快照。
func (s *Service) BuildUpdateBundle(ctx context.Context, sourceID, applicationID, requestedRevision string) (packagebuilder.Artifact, error) {
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
	// 当前索引控制是否还能创建包，包括使用历史版本的更新请求。
	app, err := s.store.CatalogAppByID(ctx, sourceID, applicationID)
	if err != nil || app.Revision != source.CurrentRevision {
		return packagebuilder.Artifact{}, errors.New("应用与来源快照版本不匹配")
	}
	if app.CatalogOnly || app.NativeInstall != "" {
		return packagebuilder.Artifact{}, errors.New("此应用未声明 Compose 部署包")
	}
	appRevision := app.Revision
	if requestedRevision != "" {
		// 目录只保留最新清单摘要。历史版本由该来源的固定 Git 引用锁定整棵树。
		appRevision = requestedRevision
	}
	repository, err := pinnedBundleRepository(ctx, filepath.Join(s.cacheDir, source.ID+".git"), appRevision)
	if err != nil {
		return packagebuilder.Artifact{}, err
	}
	manifestPath, err := manifestPathAtRevision(ctx, repository, sourceID, applicationID, app)
	if err != nil {
		return packagebuilder.Artifact{}, err
	}
	manifestRaw, err := repository.contents(ctx, manifestPath)
	if err != nil {
		return packagebuilder.Artifact{}, errors.New("固定版本 Manifest 不可读取")
	}
	if err := ValidateManifestSchema(manifestRaw); err != nil {
		return packagebuilder.Artifact{}, fmt.Errorf("固定版本 Manifest 未通过 Deploy JSON Schema 校验: %w", err)
	}
	manifest, actualDigest, err := validatePackageManifest(applicationID, manifestRaw, repository.objects)
	if err != nil || (appRevision == source.CurrentRevision && actualDigest != app.ManifestSHA256) {
		return packagebuilder.Artifact{}, errors.New("Manifest 与已索引快照不一致")
	}
	if !manifest.IsEnabled() {
		return packagebuilder.Artifact{}, errors.New("应用已在来源仓库中停用，不能创建新的安装包")
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
	expanded, fields, envExample, secretsExample, err := resolveConfiguration(manifestRaw, resources)
	if err != nil {
		return packagebuilder.Artifact{}, err
	}
	if manifest.Version == 1 {
		envExample, err = repository.contents(ctx, ".env.example")
		if err != nil {
			return packagebuilder.Artifact{}, errors.New("固定版本缺少普通配置模板")
		}
		secretsExample, err = repository.contents(ctx, ".secrets.example")
		if err != nil {
			return packagebuilder.Artifact{}, errors.New("固定版本缺少密钥模板")
		}
	}
	if manifest.Icon != "" {
		if _, _, err := readIcon(ctx, repository, path.Dir(manifestPath), manifest.Icon); err != nil {
			return packagebuilder.Artifact{}, err
		}
	}
	if err := ValidateManifestSemanticsV1(expanded, resources, envExample, secretsExample); err != nil {
		return packagebuilder.Artifact{}, fmt.Errorf("Manifest 未通过 Deploy 跨文件语义校验: %w", err)
	}

	// Deploy 的 options / visible_when 是 EasyNetTopo 表单展示元数据。
	// 保留 Artifact.Config 中的完整定义供 UI 使用，但打包给 Agent 前只移除这两个
	// Agent 契约不包含的展示字段；其余运行配置字段及原始 app.yaml 均保持不变。
	bundleResources := resources
	if manifest.Version == 2 {
		bundleResources, err = normalizeAgentConfigResources(applicationID, resources)
		if err != nil {
			return packagebuilder.Artifact{}, fmt.Errorf("无法将应用配置转换为 EasyHostAgent 契约: %w", err)
		}
	}
	artifact, err := packagebuilder.Build(packagebuilder.Identity{
		SourceID: sourceID, ApplicationID: applicationID, GitRevision: appRevision, ManifestSHA256: actualDigest,
	}, manifestRaw, bundleResources)
	artifact.Config = fields
	artifact.SchemaVersion = manifest.Version
	return artifact, err
}
