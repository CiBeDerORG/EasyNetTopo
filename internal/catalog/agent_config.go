package catalog

import (
	"errors"
	"fmt"

	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
	"gopkg.in/yaml.v3"
)

// normalizeAgentConfigResources creates an Agent-compatible copy of v2 configuration
// definitions. UI-only metadata stays in the catalog response and is not sent to Agent.
func normalizeAgentConfigResources(applicationID string, resources []packagebuilder.Resource) ([]packagebuilder.Resource, error) {
	if !appID.MatchString(applicationID) {
		return nil, errors.New("应用 ID 无效")
	}

	normalized := append([]packagebuilder.Resource(nil), resources...)
	found := map[string]bool{}
	for index := range normalized {
		destination := normalized[index].Destination
		if destination != applicationID+"/config.yaml" && destination != applicationID+"/secrets.yaml" {
			continue
		}
		if found[destination] {
			return nil, errors.New("配置定义目标重复")
		}
		if len(normalized[index].Content) == 0 || len(normalized[index].Content) > 128*1024 {
			return nil, errors.New("配置定义缺失或超过大小限制")
		}
		document, err := parseStrictYAML(normalized[index].Content)
		if err != nil {
			return nil, fmt.Errorf("配置定义无法解析: %w", err)
		}
		rawFields, ok := document["fields"].([]any)
		if !ok {
			return nil, errors.New("配置定义必须包含 fields 数组")
		}
		for _, rawField := range rawFields {
			field, ok := rawField.(map[string]any)
			if !ok {
				return nil, errors.New("配置字段必须为对象")
			}
			// EasyHostAgent has no conditional-visibility contract. The control
			// plane enforces required inputs for visible fields; conditional fields
			// must be optional in the Agent's unconditional runtime definition.
			if _, conditional := field["visible_when"]; conditional {
				field["required"] = false
			}
			// These keys are presentation-only and are intentionally not part of
			// EasyHostAgent's strict runtime field contract.
			delete(field, "options")
			delete(field, "visible_when")
		}
		content, err := yaml.Marshal(document)
		if err != nil {
			return nil, errors.New("无法生成 Agent 兼容配置定义")
		}
		normalized[index].Content = content
		found[destination] = true
	}

	for _, name := range []string{"config.yaml", "secrets.yaml"} {
		if !found[applicationID+"/"+name] {
			return nil, fmt.Errorf("部署包缺少配置定义: %s", name)
		}
	}
	return normalized, nil
}
