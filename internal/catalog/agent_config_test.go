package catalog

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
	"gopkg.in/yaml.v3"
)

// Keep this shape aligned with EasyHostAgent/internal/market.field.
// KnownFields(true) reproduces the Agent's strict decoding behavior.
type agentFieldContract struct {
	Key, Type, Target  string
	Label, Description string
	CustomInput        *bool `yaml:"custom_input"`
	GeneratedByScript  bool  `yaml:"generated_by_script"`
	Required           bool
	Editable           *bool
	Default            *string
	Minimum, Maximum   *int
	MinLength          *int `yaml:"min_length"`
	MaxLength          *int `yaml:"max_length"`
	Pattern            string
}

func TestNormalizeAgentConfigResources(t *testing.T) {
	config := []byte(`fields:
  - key: EASYTIER_ROLE
    label: 运行角色
    type: string
    required: true
    default: cloud
    description: 选择运行角色
    custom_input: true
    editable: true
    options:
      - {label: 云端配置中心, value: cloud}
      - {label: 普通组网节点, value: intranet}
  - key: EASYTIER_WEB_PORT
    label: Web 端口
    type: port
    required: true
    default: 7007
    description: 云端 Web 服务端口
    custom_input: true
    editable: true
    visible_when: {field: EASYTIER_ROLE, equals: cloud}
`)
	secrets := []byte("fields: []\n")
	resources := []packagebuilder.Resource{
		{Source: "easytier/config.yaml", Destination: "easytier/config.yaml", Mode: "0644", Content: config},
		{Source: "easytier/secrets.yaml", Destination: "easytier/secrets.yaml", Mode: "0644", Content: secrets},
		{Source: "easytier/deploy/install.sh", Destination: "easytier/deploy/install.sh", Mode: "0755", Content: []byte("#!/bin/sh\n")},
	}

	normalized, err := normalizeAgentConfigResources("easytier", resources)
	if err != nil {
		t.Fatalf("配置转换失败: %v", err)
	}

	// The source resource is immutable: UI metadata remains available to catalog/UI.
	if !bytes.Equal(resources[0].Content, config) {
		t.Fatal("配置转换修改了原始 Deploy 资源")
	}
	sourceDoc, err := parseStrictYAML(resources[0].Content)
	if err != nil {
		t.Fatal(err)
	}
	sourceFields := sourceDoc["fields"].([]any)
	if _, ok := sourceFields[0].(map[string]any)["options"]; !ok {
		t.Fatal("原始表单定义丢失 options")
	}
	if _, ok := sourceFields[1].(map[string]any)["visible_when"]; !ok {
		t.Fatal("原始表单定义丢失 visible_when")
	}

	var runtime struct {
		Fields []agentFieldContract `yaml:"fields"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(normalized[0].Content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&runtime); err != nil {
		t.Fatalf("转换结果仍不符合 EasyHostAgent 严格字段契约: %v", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("Agent 配置必须只有一个 YAML 文档: %v", err)
	}
	if len(runtime.Fields) != 2 {
		t.Fatalf("字段数量改变: got %d", len(runtime.Fields))
	}
	role, port := runtime.Fields[0], runtime.Fields[1]
	if role.Key != "EASYTIER_ROLE" || role.Default == nil || *role.Default != "cloud" ||
		role.CustomInput == nil || !*role.CustomInput || role.Editable == nil || !*role.Editable {
		t.Fatalf("角色运行配置字段丢失: %+v", role)
	}
	if port.Key != "EASYTIER_WEB_PORT" || port.Default == nil || *port.Default != "7007" ||
		port.CustomInput == nil || !*port.CustomInput || port.Editable == nil || !*port.Editable {
		t.Fatalf("条件字段的运行配置字段丢失: %+v", port)
	}
	for _, raw := range normalized[:2] {
		if strings.Contains(string(raw.Content), "options:") || strings.Contains(string(raw.Content), "visible_when:") {
			t.Fatalf("Agent 包仍包含 UI 展示字段: %s", raw.Destination)
		}
	}
}

func TestNormalizeAgentConfigResourcesRejectsIncompleteDefinitions(t *testing.T) {
	_, err := normalizeAgentConfigResources("easytier", []packagebuilder.Resource{
		{Source: "easytier/config.yaml", Destination: "easytier/config.yaml", Content: []byte("fields: []\n")},
	})
	if err == nil {
		t.Fatal("缺少 secrets.yaml 时应拒绝生成 Agent 包")
	}

	_, err = normalizeAgentConfigResources("easytier", []packagebuilder.Resource{
		{Source: "easytier/config.yaml", Destination: "easytier/config.yaml", Content: []byte("fields: {}\n")},
		{Source: "easytier/secrets.yaml", Destination: "easytier/secrets.yaml", Content: []byte("fields: []\n")},
	})
	if err == nil {
		t.Fatal("fields 不是数组时应拒绝生成 Agent 包")
	}
}
