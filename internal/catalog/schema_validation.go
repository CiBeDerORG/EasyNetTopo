package catalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schema/app-manifest-v1.schema.json
var deploySchemaV1 []byte

var (
	deploySchemaOnce sync.Once
	deploySchema     *jsonschema.Schema
	deploySchemaErr  error
)

// Deploy v1 Schema 使用 ECMAScript 负向前瞻，Go 标准 regexp 无法支持。
// 使用有超时的正则引擎，Schema 规则仍固定在控制端本地。
type deployRegexp regexp2.Regexp

func (r *deployRegexp) MatchString(value string) bool {
	matched, err := (*regexp2.Regexp)(r).MatchString(value)
	return err == nil && matched
}
func (r *deployRegexp) String() string { return (*regexp2.Regexp)(r).String() }

func compileDeployRegexp(pattern string) (jsonschema.Regexp, error) {
	compiled, err := regexp2.Compile(pattern, regexp2.ECMAScript)
	if err != nil {
		return nil, err
	}
	compiled.MatchTimeout = 50 * time.Millisecond
	return (*deployRegexp)(compiled), nil
}

func compileDeploySchema() (*jsonschema.Schema, error) {
	deploySchemaOnce.Do(func() {
		// Schema 固定在控制端可信源码中，不允许 Git 来源自带的 Schema 替换规则。
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(deploySchemaV1))
		if err != nil {
			deploySchemaErr = err
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		compiler.UseRegexpEngine(compileDeployRegexp)
		if err := compiler.AddResource("deploy-v1.json", document); err != nil {
			deploySchemaErr = err
			return
		}
		deploySchema, deploySchemaErr = compiler.Compile("deploy-v1.json")
	})
	return deploySchema, deploySchemaErr
}

// rejectYAMLFeatures 阻止重复键、Alias、合并键和非 JSON 类型标记，
// 保证 YAML 解析器与 JSON Schema 对同一输入不会产生不同的字段解释。
func rejectYAMLFeatures(node *yaml.Node) error {
	switch node.Kind {
	case yaml.AliasNode:
		return errors.New("应用清单不允许 YAML Alias")
	case yaml.MappingNode:
		keys := map[string]bool{}
		for index := 0; index+1 < len(node.Content); index += 2 {
			key, val := node.Content[index], node.Content[index+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" {
				return errors.New("应用清单映射键必须为普通字符串")
			}
			if keys[key.Value] {
				return errors.New("应用清单有重复字段")
			}
			keys[key.Value] = true
			if err := rejectYAMLFeatures(val); err != nil {
				return err
			}
		}
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, item := range node.Content {
			if err := rejectYAMLFeatures(item); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str", "!!bool", "!!int", "!!float", "!!null":
		default:
			return errors.New("应用清单出现不支持的 YAML 类型")
		}
	default:
		return errors.New("应用清单包含不支持的 YAML 结构")
	}
	return nil
}

// ValidateManifestSchemaV1 逐字段执行 Deploy v1 JSON Schema 2020-12 的校验。
// 此处只验证 Manifest 结构，不等于通过 Deploy 的跨文件语义验证，更不是发布或执行授权。
func ValidateManifestSchemaV1(raw []byte) error {
	if len(raw) == 0 || len(raw) > 128*1024 {
		return errors.New("应用清单长度不符合限制")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		return errors.New("应用清单 YAML 无法解析")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("应用清单必须只有一个 YAML 文档")
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return errors.New("应用清单根节点必须为对象")
	}
	if err := rejectYAMLFeatures(&root); err != nil {
		return err
	}
	var data any
	if err := root.Decode(&data); err != nil {
		return errors.New("应用清单 YAML 数据无效")
	}
	canonical, err := json.Marshal(data)
	if err != nil {
		return errors.New("应用清单不能转为 JSON Schema 输入")
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(canonical))
	if err != nil {
		return errors.New("应用清单 JSON Schema 输入无效")
	}
	schema, err := compileDeploySchema()
	if err != nil {
		return fmt.Errorf("内置 Deploy JSON Schema 初始化失败: %w", err)
	}
	if err := schema.Validate(value); err != nil {
		// 不输出原始字段值，避免通过错误消息泄露第三方清单中的敏感信息。
		return errors.New("应用清单不符合 Deploy v1 JSON Schema 约束")
	}
	return nil
}
