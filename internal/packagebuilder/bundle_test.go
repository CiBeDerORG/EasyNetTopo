package packagebuilder

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func fixture() (Identity, []byte, []Resource) {
	manifest := []byte("schema_version: 1\nid: sub2api\n")
	hash := sha256.Sum256(manifest)
	return Identity{
			SourceID: "11111111-1111-4111-8111-111111111111", ApplicationID: "sub2api",
			GitRevision: strings.Repeat("a", 40), ManifestSHA256: hex.EncodeToString(hash[:]),
		}, manifest, []Resource{
			{Source: "sub2api/deploy/install.sh", Destination: "sub2api/deploy/install.sh", Mode: "0755", Content: []byte("#!/bin/sh\nexit 0\n")},
			{Source: "lib/image-sources.sh", Destination: "deploy/lib/image-sources.sh", Mode: "0644", Content: []byte("# helper\n")},
			{Source: "sub2api/deploy/docker-compose.yaml", Destination: "sub2api/deploy/docker-compose.yaml", Content: []byte("services: {}\n")},
		}
}

func TestDeterministicBuildAndMetadata(t *testing.T) {
	id, manifest, resources := fixture()
	a, err := Build(id, manifest, resources)
	if err != nil {
		t.Fatal(err)
	}
	resources[0], resources[2] = resources[2], resources[0]
	b, err := Build(id, manifest, resources)
	if err != nil || !bytes.Equal(a.ZIP, b.ZIP) || a.SHA256 != b.SHA256 {
		t.Fatalf("输入顺序不应改变 ZIP: %v", err)
	}
	recomputed := sha256.Sum256(a.ZIP)
	if a.SHA256 != hex.EncodeToString(recomputed[:]) || a.SizeBytes != int64(len(a.ZIP)) {
		t.Fatal("ZIP 摘要或长度不一致")
	}
	reader, err := zip.NewReader(bytes.NewReader(a.ZIP), a.SizeBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != len(resources)+2 {
		t.Fatalf("ZIP 包文件数异常: %d", len(reader.File))
	}
	found := map[string][]byte{}
	for _, file := range reader.File {
		if file.Method != zip.Store || !file.Modified.Equal(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("ZIP 文件头不规范: %s %v", file.Name, file.Modified)
		}
		fd, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(fd)
		_ = fd.Close()
		if err != nil {
			t.Fatal(err)
		}
		found[file.Name] = data
		if file.Name == "sub2api/deploy/install.sh" && file.Mode().Perm() != 0755 {
			t.Fatalf("脚本权限未保留: %04o", file.Mode().Perm())
		}
	}
	if !bytes.Equal(found["sub2api/app.yaml"], manifest) {
		t.Fatal("Manifest 缺失")
	}
	var meta bundleMetadata
	if err := json.Unmarshal(found["bundle-manifest.json"], &meta); err != nil {
		t.Fatal(err)
	}
	if meta.SourceID != id.SourceID || meta.GitRevision != id.GitRevision || len(meta.Files) != len(resources) {
		t.Fatal("Bundle 元信息缺少固定身份或文件摘要")
	}
	for _, entry := range meta.Files {
		hash := sha256.Sum256(found[entry.Destination])
		if hex.EncodeToString(hash[:]) != entry.SHA256 || int64(len(found[entry.Destination])) != entry.SizeBytes {
			t.Fatal("元信息文件校验不一致")
		}
	}
}

func TestRejectInvalidMetadataAndUnsafeFiles(t *testing.T) {
	id, manifest, resources := fixture()
	cases := []struct {
		name string
		edit func(*Identity, *[]byte, *[]Resource)
	}{
		{"伪造清单摘要", func(i *Identity, _ *[]byte, _ *[]Resource) { i.ManifestSHA256 = strings.Repeat("0", 64) }},
		{"非法来源", func(i *Identity, _ *[]byte, _ *[]Resource) { i.SourceID = "fake" }},
		{"未固定 Git 版本", func(i *Identity, _ *[]byte, _ *[]Resource) { i.GitRevision = "main" }},
		{"空包", func(_ *Identity, _ *[]byte, r *[]Resource) { *r = nil }},
		{"路径穿越", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Destination = "../escape" }},
		{"隐藏敏感文件", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Destination = "sub2api/.secrets" }},
		{"运行数据目录", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Destination = "sub2api/data/payload" }},
		{"其他应用来源", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Source = "other/install.sh" }},
		{"共享库来源伪造", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Source = "lib/install.sh" }},
		{"共享库映射错误", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[1].Destination = "sub2api/deploy/lib.sh" }},
		{"非法权限", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Mode = "0777" }},
		{"重复目标", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Destination = (*r)[2].Destination }},
		{"忽略大小写重复", func(_ *Identity, _ *[]byte, r *[]Resource) {
			(*r)[0].Destination = "sub2api/deploy/DOCKER-COMPOSE.yaml"
		}},
		{"重复源文件", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Source = (*r)[2].Source }},
		{"覆盖清单", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Destination = "sub2api/app.yaml" }},
		{"文件目录碰撞", func(_ *Identity, _ *[]byte, r *[]Resource) { (*r)[0].Destination = "sub2api/deploy" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copied := append([]Resource(nil), resources...)
			copyID, copyManifest := id, append([]byte(nil), manifest...)
			tc.edit(&copyID, &copyManifest, &copied)
			if _, err := Build(copyID, copyManifest, copied); err == nil {
				t.Fatal("危险包被接受")
			}
		})
	}
}

func TestRejectExcessiveFileSize(t *testing.T) {
	id, manifest, resources := fixture()
	resources[0].Content = bytes.Repeat([]byte("x"), MaxFileBytes+1)
	if _, err := Build(id, manifest, resources); err == nil {
		t.Fatal("大文件被接受")
	}
}
