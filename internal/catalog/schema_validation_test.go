package catalog

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDeployV1SchemaRejectsInvalidManifest(t *testing.T) {
	good, err := os.ReadFile("testdata/sub2api.app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateManifestSchemaV1(good); err != nil {
		t.Fatalf("Deploy Sub2API v1 官方样本未通过: %v", err)
	}
	macos := bytes.Replace(good, []byte("  - fnos"), []byte("  - fnos\n  - macos"), 1)
	if err := ValidateManifestSchemaV1(macos); err != nil {
		t.Fatalf("macOS 平台声明应通过应用清单 Schema：%v", err)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{"必填属性缺失", bytes.Replace(good, []byte("schema_version: 1"), nil, 1)},
		{"非法版本", bytes.Replace(good, []byte("schema_version: 1"), []byte("schema_version: 2"), 1)},
		{"多余字段", append(append([]byte(nil), good...), []byte("\nunknown_root: true\n")...)},
		{"非法权限", bytes.Replace(good, []byte("mode: \"0755\""), []byte("mode: \"0777\""), 1)},
		{"非法端口下界声明", bytes.Replace(good, []byte("minimum: 1"), []byte("minimum: 0"), 1)},
		{"重复键", append(append([]byte(nil), good...), []byte("\nid: malicious\n")...)},
		{"多 YAML 文档", append(append([]byte(nil), good...), []byte("\n---\nid: malicious\n")...)},
		{"YAML Alias", []byte("id: &id sub2api\nname: *id\n")},
		{"YAML 合并键", []byte("base: &base {id: sub2api}\nmanifest:\n  <<: *base\n")},
		{"自定义 YAML 标签", []byte("id: !Custom sub2api\n")},
		{"超过大小限制", bytes.Repeat([]byte("x"), 128*1024+1)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateManifestSchemaV1(test.data); err == nil {
				t.Fatal("非法 Manifest 被当作有效发布描述")
			}
		})
	}
}

func TestDeployV1SchemaDoesNotAcceptMinimalPreview(t *testing.T) {
	minimal := []byte(strings.Join([]string{
		"schema_version: 1",
		"id: sub2api",
		"name: Sub2API",
		"summary: minimal",
		"category: developer",
		"runtime:",
		"  kind: docker_compose",
		"  compose_project: sub2api",
		"  compose_file: deploy/docker-compose.yaml",
		"package:",
		"  files:",
		"    - source: sub2api/deploy/docker-compose.yaml",
		"      destination: sub2api/deploy/docker-compose.yaml",
		"verification:",
		"  state: healthy",
		"  required_services: [sub2api]",
		"data:",
		"  default_retain: true",
	}, "\n") + "\n")
	if err := ValidateManifestSchemaV1(minimal); err == nil {
		t.Fatal("只读预览需要的最小字段不能直接当发布 Manifest")
	}
}
