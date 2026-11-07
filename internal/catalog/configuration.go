package catalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schema/app-manifest-v2.schema.json
var deploySchemaV2 []byte

//go:embed schema/app-fields-v2.schema.json
var fieldsSchemaV2 []byte
var configSchemasOnce sync.Once
var configSchemas map[string]*jsonschema.Schema
var configSchemasErr error

func validateConfigSchema(name string, data map[string]any) error {
	configSchemasOnce.Do(func() {
		configSchemas = map[string]*jsonschema.Schema{}
		for name, raw := range map[string][]byte{"manifest": deploySchemaV2, "fields": fieldsSchemaV2} {
			value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				configSchemasErr = err
				return
			}
			c := jsonschema.NewCompiler()
			c.AssertFormat()
			c.UseRegexpEngine(compileDeployRegexp)
			if err = c.AddResource(name+".json", value); err != nil {
				configSchemasErr = err
				return
			}
			configSchemas[name], configSchemasErr = c.Compile(name + ".json")
			if configSchemasErr != nil {
				return
			}
		}
	})
	if configSchemasErr != nil {
		return configSchemasErr
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return errors.New("配置数据不能序列化")
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if err := configSchemas[name].Validate(value); err != nil {
		return errors.New("应用配置不符合内置 Schema")
	}
	return nil
}

func ValidateManifestSchema(raw []byte) error {
	if len(raw) > 128*1024 {
		return errors.New("清单超过大小限制")
	}
	data, err := parseStrictYAML(raw)
	if err != nil {
		return err
	}
	if data["schema_version"] == 2 {
		return validateConfigSchema("manifest", data)
	}
	return ValidateManifestSchemaV1(raw)
}

// resolveConfiguration 保持包内原始清单不变，仅为已有语义校验与安装表单生成统一视图。
func resolveConfiguration(raw []byte, resources []packagebuilder.Resource) ([]byte, []map[string]any, []byte, []byte, error) {
	data, err := parseStrictYAML(raw)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var fields []map[string]any
	if data["schema_version"] != 2 {
		var old struct{ Config []map[string]any }
		err = yaml.Unmarshal(raw, &old)
		for _, f := range old.Config {
			f["custom_input"] = true
			if _, ok := f["editable"]; !ok {
				f["editable"] = true
			}
		}
		return raw, old.Config, nil, nil, err
	}
	app, _ := data["id"].(string)
	files := map[string][]byte{}
	for _, r := range resources {
		files[r.Destination] = r.Content
	}
	templates := map[string][]byte{}
	seen := map[string]bool{}
	for _, def := range []struct{ name, target string }{{"config", "env"}, {"secrets", "secrets"}} {
		content := files[app+"/"+def.name+".yaml"]
		if len(content) > 128*1024 {
			return nil, nil, nil, nil, errors.New("配置定义超过大小限制")
		}
		document, e := parseStrictYAML(content)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		if e = validateConfigSchema("fields", document); e != nil {
			return nil, nil, nil, nil, e
		}
		var expected strings.Builder
		expected.WriteString("# 由 config.yaml / secrets.yaml 生成，请勿手工维护；不是运行时配置。\n")
		for _, item := range document["fields"].([]any) {
			f := item.(map[string]any)
			key := f["key"].(string)
			if seen[key] || (f["type"] == "secret") != (def.target == "secrets") || (def.target == "secrets" && f["default"] != "") {
				return nil, nil, nil, nil, errors.New("配置键重复或密钥定义无效")
			}
			seen[key] = true
			editable := true
			if value, ok := f["editable"]; ok {
				editable = value.(bool)
			}
			if f["required"] == true && (f["custom_input"] == false || !editable) && f["default"] == "" && f["generated_by_script"] != true {
				return nil, nil, nil, nil, errors.New("必填只读字段缺少默认值或生成机制")
			}
			f["editable"] = editable
			f["target"] = def.target
			value := fmt.Sprint(f["default"])
			if strings.ContainsAny(value, "\r\n\x00") {
				return nil, nil, nil, nil, errors.New("默认值不能包含换行或 NUL")
			}
			fmt.Fprintf(&expected, "%s=\"%s\"\n", key, value)
			fields = append(fields, f)
		}
		actual := files[app+"/deploy/config."+def.target+".example"]
		if string(actual) != expected.String() {
			return nil, nil, nil, nil, errors.New("脚本默认值模板与配置定义不一致")
		}
		templates[def.target] = actual
	}
	// 复用 v1 的类型、默认值、Compose、端口和生命周期约束。
	normalized := make([]map[string]any, 0, len(fields))
	for _, f := range fields {
		item := map[string]any{}
		for k, v := range f {
			if k != "custom_input" {
				item[k] = v
			}
		}
		normalized = append(normalized, item)
	}
	data["config"] = normalized
	data["schema_version"] = 1
	delete(data, "configuration")
	delete(data, "icon")
	// enabled 仅用于商店发布；标准化为 v1 语义输入时移除展示字段。
	delete(data, "enabled")
	expanded, err := yaml.Marshal(data)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := ValidateManifestSchemaV1(expanded); err != nil {
		return nil, nil, nil, nil, err
	}
	return expanded, fields, templates["env"], templates["secrets"], nil
}
