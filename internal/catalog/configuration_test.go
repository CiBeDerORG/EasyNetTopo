package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
	"gopkg.in/yaml.v3"
)

func independentConfigurationFixture(t *testing.T) ([]byte, []packagebuilder.Resource, []byte, []byte) {
	t.Helper()
	raw, resources, _, _ := syntheticDeployFixture(t)
	// 使用平台没有任何特殊分支的应用名，验证契约由数据驱动。
	raw = []byte(strings.ReplaceAll(string(raw), "sub2api", "unregistered-demo"))
	for i := range resources {
		r := &resources[i]
		r.Source = strings.ReplaceAll(r.Source, "sub2api", "unregistered-demo")
		r.Destination = strings.ReplaceAll(r.Destination, "sub2api", "unregistered-demo")
		r.Content = []byte(strings.ReplaceAll(string(r.Content), "sub2api", "unregistered-demo"))
	}
	data, err := parseStrictYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	old := data["config"].([]any)
	data["schema_version"] = 2
	delete(data, "config")
	data["icon"] = "assets/icon.svg"
	data["configuration"] = map[string]any{"config": "config.yaml", "secrets": "secrets.yaml"}
	pkg := data["package"].(map[string]any)
	data["actions"].(map[string]any)["uninstall"] = map[string]any{"strategy": "script", "path": "deploy/uninstall.sh"}
	uninstall := "unregistered-demo/deploy/uninstall.sh"
	resources = append(resources, packagebuilder.Resource{Source: uninstall, Destination: uninstall, Mode: "0755", Content: []byte("#!/bin/sh\n# 测试卸载入口\n")})
	pkg["files"] = append(pkg["files"].([]any), map[string]any{"source": uninstall, "destination": uninstall, "mode": "0755"})
	for _, group := range []struct{ name, target string }{{"config", "env"}, {"secrets", "secrets"}} {
		fields := []any{}
		var defaults strings.Builder
		defaults.WriteString("# 由 config.yaml / secrets.yaml 生成，请勿手工维护；不是运行时配置。\n")
		for _, item := range old {
			f := item.(map[string]any)
			if f["target"] != group.target {
				continue
			}
			delete(f, "target")
			f["custom_input"] = true
			f["description"] = "测试字段"
			if _, ok := f["default"]; !ok {
				f["default"] = ""
			}
			fields = append(fields, f)
			fmt.Fprintf(&defaults, "%s=\"%v\"\n", f["key"], f["default"])
		}
		definition, err := yaml.Marshal(map[string]any{"fields": fields})
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range []struct {
			name string
			data []byte
		}{{group.name + ".yaml", definition}, {"deploy/config." + group.target + ".example", []byte(defaults.String())}} {
			name := "unregistered-demo/" + file.name
			resources = append(resources, packagebuilder.Resource{Source: name, Destination: name, Mode: "0644", Content: file.data})
			pkg["files"] = append(pkg["files"].([]any), map[string]any{"source": name, "destination": name, "mode": "0644"})
		}
	}
	raw, err = yaml.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return raw, resources, nil, nil
}

func TestIndependentDefinitionsAndTemplateConsistency(t *testing.T) {
	raw, resources, _, _ := independentConfigurationFixture(t)
	if err := ValidateManifestSchema(raw); err != nil {
		t.Fatal(err)
	}
	expanded, fields, env, secrets, err := resolveConfiguration(raw, resources)
	if err != nil || len(fields) == 0 {
		t.Fatalf("定义解析失败：%v", err)
	}
	if err := ValidateManifestSemanticsV1(expanded, resources, env, secrets); err != nil {
		t.Fatal(err)
	}
	for i := range resources {
		if strings.HasSuffix(resources[i].Destination, "config.env.example") {
			resources[i].Content = append(resources[i].Content, []byte("UNDECLARED=1\n")...)
			break
		}
	}
	if _, _, _, _, err := resolveConfiguration(raw, resources); err == nil {
		t.Fatal("接受了与定义不一致的脚本默认值")
	}
}

func TestDeployRepositoryV2Contract(t *testing.T) {
	root := os.Getenv("DEPLOY_CONTRACT_ROOT")
	if root == "" {
		t.Skip("未指定 Deploy 工作区")
	}
	paths, err := filepath.Glob(filepath.Join(root, "*/app.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatal("没有应用清单")
	}
	for _, path := range paths {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateManifestSchema(raw); err != nil {
				t.Fatal(err)
			}
			var m appManifest
			if err := yaml.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			var resources []packagebuilder.Resource
			for _, f := range m.Package.Files {
				content, err := os.ReadFile(filepath.Join(root, f.Source))
				if err != nil {
					t.Fatal(err)
				}
				resources = append(resources, packagebuilder.Resource{Source: f.Source, Destination: f.Destination, Mode: f.Mode, Content: content})
			}
			expanded, fields, env, secrets, err := resolveConfiguration(raw, resources)
			if err != nil || len(fields) == 0 {
				t.Fatalf("配置契约无效：%v", err)
			}
			if err := ValidateManifestSemanticsV1(expanded, resources, env, secrets); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConfigurationSchemasRejectUnsupportedControls(t *testing.T) {
	raw, resources, _, _ := independentConfigurationFixture(t)
	for _, bad := range []string{
		"fields: []\nfields: []\n",
		"fields:\n- key: APP_VALUE\n  label: 测试\n  type: string\n  required: true\n  description: 测试\n  custom_input: false\n  default: ''\n",
		"fields:\n- key: APP_VALUE\n  label: 测试\n  type: string\n  required: false\n  description: 测试\n  custom_input: true\n  default: ''\n  target: secrets\n",
	} {
		modified := append([]packagebuilder.Resource(nil), resources...)
		for i := range modified {
			if strings.HasSuffix(modified[i].Destination, "/config.yaml") {
				modified[i].Content = []byte(bad)
			}
		}
		if _, _, _, _, err := resolveConfiguration(raw, modified); err == nil {
			t.Fatal("接受了非法字段权限或重复定义")
		}
	}
}
