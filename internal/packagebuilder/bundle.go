// Package packagebuilder 仅创建确定性的 ZIP v1 候选部署包。
// 字节完整性不代表发布者真实性，也不授予脚本或 Docker 执行权限。
package packagebuilder

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	MaxFileBytes    = 5 << 20
	MaxPayloadBytes = 24 << 20
	MaxArchiveBytes = 64 << 20
	MaxFiles        = 128
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	appPattern      = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	sha1Pattern     = regexp.MustCompile(`^[a-f0-9]{40}$`)
	sha256Pattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
	pathPartPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	zipEpoch        = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
)

// Identity 是固定版本和来源的可审计标识，不是信任证明。
type Identity struct {
	SourceID       string
	ApplicationID  string
	GitRevision    string
	ManifestSHA256 string
}

// Resource 必须来自固定 Git Commit 的已验证普通文件 Blob。
type Resource struct {
	Source      string
	Destination string
	Mode        string
	Content     []byte
}

// Artifact 只供服务端内部流程使用，不得视为可执行授权。
type Artifact struct {
	Config        []map[string]any
	SchemaVersion int
	Revision      string
	Manifest      []byte
	ZIP           []byte
	SHA256        string
	SizeBytes     int64
}

type fileRecord struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
}

type bundleMetadata struct {
	FormatVersion  int          `json:"format_version"`
	SourceID       string       `json:"source_id"`
	ApplicationID  string       `json:"application_id"`
	GitRevision    string       `json:"git_revision"`
	ManifestSHA256 string       `json:"manifest_sha256"`
	Files          []fileRecord `json:"files"`
}

type archiveFile struct {
	name string
	mode int64
	data []byte
}

func safePath(value string) bool {
	if value == "" || len(value) > 256 || path.Clean(value) != value || strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\\x00:\r\n") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if !pathPartPattern.MatchString(part) || strings.HasPrefix(part, ".") {
			return false
		}
		switch strings.ToLower(part) {
		case "data", "state", "backup", "runtime", "workspace", "id_rsa", "id_ed25519":
			return false
		}
	}
	return true
}

func validateIdentity(id Identity, manifest []byte) error {
	if !uuidPattern.MatchString(id.SourceID) || !appPattern.MatchString(id.ApplicationID) || len(id.ApplicationID) < 2 || len(id.ApplicationID) > 64 || !sha1Pattern.MatchString(id.GitRevision) || !sha256Pattern.MatchString(id.ManifestSHA256) {
		return errors.New("部署包来源、应用或固定版本标识不合法")
	}
	if len(manifest) == 0 || len(manifest) > 128*1024 {
		return errors.New("Manifest 字节长度超限")
	}
	digest := sha256.Sum256(manifest)
	if hex.EncodeToString(digest[:]) != id.ManifestSHA256 {
		return errors.New("Manifest 与目录快照摘要不一致")
	}
	return nil
}

// Build 生成可复现 ZIP：仅依据固定输入内容、规范化路径和权限生成字节。
// 调用方必须另外执行 Deploy v1 完整 Schema/语义验证并验证 Git 来源，才可进入发布/执行准入。
// 本函数不读取本机文件、不运行代码、不授予信任，也不上传到 Agent。
func Build(id Identity, manifest []byte, resources []Resource) (Artifact, error) {
	if err := validateIdentity(id, manifest); err != nil {
		return Artifact{}, err
	}
	if len(resources) == 0 || len(resources) > MaxFiles {
		return Artifact{}, errors.New("部署文件数量必须为 1 至 128")
	}
	manifestPath := id.ApplicationID + "/app.yaml"
	usedNames := map[string]bool{"bundle-manifest.json": true, strings.ToLower(manifestPath): true}
	usedSources := map[string]bool{}
	files := make([]archiveFile, 0, len(resources)+2)
	records := make([]fileRecord, 0, len(resources))
	var total int64
	for _, resource := range resources {
		if !safePath(resource.Source) || !safePath(resource.Destination) {
			return Artifact{}, fmt.Errorf("资源路径不合法: %q", resource.Destination)
		}
		if resource.Source != resource.Destination {
			if !strings.HasPrefix(resource.Source, "lib/") || resource.Destination != "deploy/"+resource.Source {
				return Artifact{}, errors.New("共享库源路径与目标路径映射不一致")
			}
		} else if !strings.HasPrefix(resource.Source, id.ApplicationID+"/") {
			return Artifact{}, errors.New("部署文件必须属于目标应用或共享库")
		}
		if !strings.HasPrefix(resource.Destination, id.ApplicationID+"/") && !strings.HasPrefix(resource.Destination, "deploy/lib/") {
			return Artifact{}, errors.New("部署文件落点超出应用或共享库")
		}
		if len(resource.Content) > MaxFileBytes {
			return Artifact{}, errors.New("单文件超过 5 MiB")
		}
		total += int64(len(resource.Content))
		if total > MaxPayloadBytes {
			return Artifact{}, errors.New("部署文件累计超过 24 MiB")
		}
		mode := int64(0644)
		switch resource.Mode {
		case "", "0644":
		case "0755":
			mode = 0755
		default:
			return Artifact{}, errors.New("只允许 0644 或 0755 权限")
		}
		lowerDest := strings.ToLower(resource.Destination)
		lowerSrc := strings.ToLower(resource.Source)
		if usedNames[lowerDest] || usedSources[lowerSrc] {
			return Artifact{}, errors.New("重复资源路径（包括大小写碰撞）")
		}
		usedNames[lowerDest] = true
		usedSources[lowerSrc] = true
		hash := sha256.Sum256(resource.Content)
		records = append(records, fileRecord{Source: resource.Source, Destination: resource.Destination, Mode: fmt.Sprintf("%04o", mode), SizeBytes: int64(len(resource.Content)), SHA256: hex.EncodeToString(hash[:])})
		files = append(files, archiveFile{name: resource.Destination, mode: mode, data: resource.Content})
	}
	// ZIP 不接受同时存在文件及其子路径，以免目标路径解释产生歧义。
	for name := range usedNames {
		parts := strings.Split(name, "/")
		for index := 1; index < len(parts); index++ {
			if usedNames[strings.Join(parts[:index], "/")] {
				return Artifact{}, errors.New("部署包文件路径与目录冲突")
			}
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Destination < records[j].Destination })
	meta, err := json.Marshal(bundleMetadata{
		FormatVersion: 1, SourceID: id.SourceID, ApplicationID: id.ApplicationID,
		GitRevision: id.GitRevision, ManifestSHA256: id.ManifestSHA256, Files: records,
	})
	if err != nil {
		return Artifact{}, err
	}
	files = append(files, archiveFile{name: manifestPath, mode: 0644, data: manifest})
	files = append(files, archiveFile{name: "bundle-manifest.json", mode: 0644, data: append(meta, '\n')})
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, f := range files {
		header := &zip.FileHeader{Name: f.name, Method: zip.Store, Modified: zipEpoch}
		header.SetMode(0644)
		if f.mode == 0755 {
			header.SetMode(0755)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return Artifact{}, err
		}
		if _, err := entry.Write(f.data); err != nil {
			return Artifact{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Artifact{}, err
	}
	if output.Len() > MaxArchiveBytes {
		return Artifact{}, errors.New("ZIP 原始字节超出 Agent 64 MiB 限制")
	}
	digest := sha256.Sum256(output.Bytes())
	return Artifact{Revision: id.GitRevision, Manifest: manifest, ZIP: output.Bytes(), SHA256: hex.EncodeToString(digest[:]), SizeBytes: int64(output.Len())}, nil
}
