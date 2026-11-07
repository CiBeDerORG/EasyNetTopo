package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
	"gopkg.in/yaml.v3"
)

type semanticManifest struct {
	ID      string `yaml:"id"`
	Runtime struct {
		Project string `yaml:"compose_project"`
		File    string `yaml:"compose_file"`
	} `yaml:"runtime"`
	Compatibility struct {
		Docker    bool `yaml:"requires_docker"`
		ComposeV2 bool `yaml:"requires_compose_v2"`
	} `yaml:"compatibility"`
	Package struct {
		Files []struct {
			Source      string `yaml:"source"`
			Destination string `yaml:"destination"`
			Mode        string `yaml:"mode"`
		} `yaml:"files"`
	} `yaml:"package"`
	Config []struct {
		Key               string `yaml:"key"`
		Type              string `yaml:"type"`
		Target            string `yaml:"target"`
		Default           any    `yaml:"default"`
		Minimum           *int   `yaml:"minimum"`
		Maximum           *int   `yaml:"maximum"`
		MinLength         *int   `yaml:"min_length"`
		MaxLength         *int   `yaml:"max_length"`
		GeneratedByScript bool   `yaml:"generated_by_script"`
	} `yaml:"config"`
	Ports []struct {
		ID             string `yaml:"id"`
		Protocol       string `yaml:"protocol"`
		HostPortKey    string `yaml:"host_port_key"`
		BindAddressKey string `yaml:"bind_address_key"`
		ContainerPort  int    `yaml:"container_port"`
		Service        string `yaml:"service"`
	} `yaml:"ports"`
	Actions map[string]struct {
		Strategy      string   `yaml:"strategy"`
		Path          string   `yaml:"path"`
		Argv          []string `yaml:"argv"`
		RemoveOrphans bool     `yaml:"remove_orphans"`
	} `yaml:"actions"`
	Verification struct {
		Required []string `yaml:"required_services"`
		State    string   `yaml:"state"`
	} `yaml:"verification"`
	Data struct {
		Paths        []string `yaml:"paths"`
		NamedVolumes []string `yaml:"named_volumes"`
		Retain       bool     `yaml:"default_retain"`
	} `yaml:"data"`
}

// parseStrictYAML 不执行 Compose、Shell 或模板变量展开；不允许多文档、重复映射键与 Alias。
// 与 Deploy 校验器相比拒绝 YAML Alias 更严格，保证检视的字节语义唯一。
func parseStrictYAML(raw []byte) (map[string]any, error) {
	if len(raw) == 0 || len(raw) > packagebuilder.MaxFileBytes || !utf8.Valid(raw) {
		return nil, errors.New("Compose YAML 长度或编码异常")
	}
	reader := yaml.NewDecoder(bytes.NewReader(raw))
	var node yaml.Node
	if err := reader.Decode(&node); err != nil {
		return nil, errors.New("Compose YAML 格式无效")
	}
	var extra yaml.Node
	if err := reader.Decode(&extra); err != io.EOF {
		return nil, errors.New("Compose 不允许多个 YAML 文档")
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("Compose YAML 根节点必须是对象")
	}
	if err := rejectYAMLFeatures(&node); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := node.Decode(&result); err != nil {
		return nil, errors.New("Compose YAML 不能解释为对象")
	}
	return result, nil
}

func asMapping(value any) (map[string]any, bool) {
	data, ok := value.(map[string]any)
	return data, ok
}

func asList(value any) ([]any, bool) {
	data, ok := value.([]any)
	return data, ok
}

var composeVariable = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)(?::[^}]*)?\}`)
var shellDependency = regexp.MustCompile(`source\s+["']?\$DEPLOY_DIR/([A-Za-z0-9_.-]+\.sh)`)

func containsVariable(value any, key string) bool {
	text := fmt.Sprint(value)
	for _, match := range composeVariable.FindAllStringSubmatch(text, -1) {
		if match[1] == key {
			return true
		}
	}
	return false
}

func portMappingMatches(value any, hostKey, bindKey, protocol string, target int) bool {
	if line, ok := value.(string); ok {
		var pattern strings.Builder
		pattern.WriteString("^")
		if bindKey != "" {
			pattern.WriteString(`\$\{` + regexp.QuoteMeta(bindKey) + `(?::[^}]*)?\}:`)
		}
		pattern.WriteString(`\$\{` + regexp.QuoteMeta(hostKey) + `(?::[^}]*)?\}:`)
		pattern.WriteString(strconv.Itoa(target))
		pattern.WriteString(`(?:/` + regexp.QuoteMeta(protocol) + `)?$`)
		matched, err := regexp.MatchString(pattern.String(), line)
		return err == nil && matched
	}
	mapping, ok := asMapping(value)
	if !ok {
		return false
	}
	if fmt.Sprint(mapping["target"]) != strconv.Itoa(target) {
		return false
	}
	p, exists := mapping["protocol"]
	if exists && p != protocol {
		return false
	}
	if !exists && protocol != "tcp" {
		return false
	}
	if !containsVariable(mapping["published"], hostKey) {
		return false
	}
	return bindKey == "" || containsVariable(mapping["host_ip"], bindKey)
}

func hasTemplateKey(raw []byte, key string) bool {
	if !utf8.Valid(raw) {
		return false
	}
	prefix := key + "="
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSuffix(line, "\r"), prefix) {
			return true
		}
	}
	return false
}

func validOwnedDataPath(app, value string) bool {
	if len(value) == 0 || len(value) > 256 || !safeFile.MatchString(value) || path.Clean(value) != value {
		return false
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 || parts[0] != app {
		return false
	}
	hasData := false
	for _, part := range parts {
		if part == "" || strings.HasPrefix(part, ".") || part == ".." {
			return false
		}
		if part == "data" {
			hasData = true
		}
	}
	return hasData
}

func lowerBound(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}

// ValidateManifestSemanticsV1 对齐 Deploy v1 跨文件语义约束。
// 调用者只能提供来自同一个固定 Git Commit 的 Manifest、明示资源和共享示例模板。
// 本函数不执行脚本、不连接 Docker、不生成密码、不检查真实设备状态，也不授予执行权限。
func ValidateManifestSemanticsV1(raw []byte, resources []packagebuilder.Resource, envExample, secretsExample []byte) error {
	if err := ValidateManifestSchemaV1(raw); err != nil {
		return err
	}
	var m semanticManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return errors.New("结构合法的 Manifest 无法解析")
	}
	if !m.Compatibility.Docker || !m.Compatibility.ComposeV2 {
		return errors.New("应用必须显式要求 Docker 与 Compose v2")
	}
	byDest := make(map[string][]byte, len(resources))
	sourceKeys, destinationKeys := map[string]bool{}, map[string]bool{}
	var aggregate int64
	if len(resources) != len(m.Package.Files) {
		return errors.New("包资源数量与 Manifest 声明不一致")
	}
	for index, entry := range m.Package.Files {
		resource := resources[index]
		if entry.Source != resource.Source || entry.Destination != resource.Destination || entry.Mode != resource.Mode {
			return errors.New("资源与 Manifest 文件顺序、权限或路径不一致")
		}
		if !safePath(entry.Source) || !safePath(entry.Destination) {
			return errors.New("包资源包含不安全路径")
		}
		if !strings.HasPrefix(entry.Source, m.ID+"/") && !strings.HasPrefix(entry.Source, "lib/") {
			return errors.New("包资源不属于应用或受控共享库")
		}
		if !strings.HasPrefix(entry.Destination, m.ID+"/") && !strings.HasPrefix(entry.Destination, "deploy/lib/") {
			return errors.New("部署目标不在应用或共享库中")
		}
		if strings.HasPrefix(entry.Destination, "deploy/lib/") {
			if entry.Source != strings.TrimPrefix(entry.Destination, "deploy/") {
				return errors.New("共享库目标必须保留原始文件路径")
			}
		} else if entry.Source != entry.Destination {
			return errors.New("应用资源不允许重映射路径")
		}
		sourceKey, destKey := strings.ToLower(entry.Source), strings.ToLower(entry.Destination)
		if sourceKeys[sourceKey] || destinationKeys[destKey] {
			return errors.New("重复的源或部署目标")
		}
		sourceKeys[sourceKey], destinationKeys[destKey] = true, true
		if len(resource.Content) > packagebuilder.MaxFileBytes {
			return errors.New("包资源单文件超限")
		}
		aggregate += int64(len(resource.Content))
		if aggregate > packagebuilder.MaxPayloadBytes {
			return errors.New("包资源总体积超限")
		}
		byDest[entry.Destination] = resource.Content
	}
	composeDestination := m.ID + "/" + m.Runtime.File
	composeRaw, ok := byDest[composeDestination]
	if !ok {
		return errors.New("Compose 必须列入部署包")
	}
	compose, err := parseStrictYAML(composeRaw)
	if err != nil {
		return err
	}
	if compose["name"] != m.Runtime.Project {
		return errors.New("Compose 项目名称与 Manifest 不一致")
	}
	services, ok := asMapping(compose["services"])
	if !ok || len(services) == 0 {
		return errors.New("Compose 必须声明服务对象")
	}
	for serviceName, rawService := range services {
		if _, ok := asMapping(rawService); !ok || serviceName == "" {
			return errors.New("Compose 服务必须是非空名称的对象")
		}
	}
	for _, name := range m.Verification.Required {
		value, ok := services[name]
		if !ok {
			return errors.New("健康检查引用不存在的 Compose 服务")
		}
		service, _ := asMapping(value)
		if m.Verification.State == "healthy" {
			if _, ok := service["healthcheck"]; !ok {
				return errors.New("要求 healthy 的 Compose 服务没有 healthcheck")
			}
		}
	}

	type configField struct {
		kind string
	}
	fields := map[string]configField{}
	install := m.Actions["install"]
	for _, field := range m.Config {
		if _, duplicate := fields[field.Key]; duplicate {
			return errors.New("Manifest 配置字段键重复")
		}
		fields[field.Key] = configField{kind: field.Type}
		if (field.Type == "secret") != (field.Target == "secrets") {
			return errors.New("密钥配置与存储目标不匹配")
		}
		if field.Type == "secret" && field.Default != nil && field.Default != "" {
			return errors.New("密钥字段不得声明明文默认值")
		}
		if field.GeneratedByScript {
			if field.Type != "secret" || install.Strategy != "script" {
				return errors.New("自动生成配置仅允许安装脚本创建密钥")
			}
			script, exists := byDest[m.ID+"/"+install.Path]
			if !exists || !utf8.Valid(script) {
				return errors.New("自动生成密钥未包含可用的安装脚本")
			}
			if !regexp.MustCompile(`\b`+regexp.QuoteMeta(field.Key)+`\b`).Match(script) ||
				(!bytes.Contains(script, []byte("ensure_hex_secret")) && !bytes.Contains(script, []byte("generate"))) {
				return errors.New("自动生成密钥在安装脚本中没有可观察实现")
			}
		}
		if field.MinLength != nil && field.MaxLength != nil && *field.MinLength > *field.MaxLength {
			return errors.New("字符串长度上下界不合法")
		}
		if field.Minimum != nil && field.Maximum != nil && *field.Minimum > *field.Maximum {
			return errors.New("数值上下界不合法")
		}
		if field.Default != nil {
			switch field.Type {
			case "port":
				num, ok := field.Default.(int)
				if !ok || num < 1 || num > 65535 || num < lowerBound(field.Minimum, 1) || num > lowerBound(field.Maximum, 65535) {
					return errors.New("端口默认值超出有效范围")
				}
			case "ipv4":
				ip, ok := field.Default.(string)
				if !ok || net.ParseIP(ip) == nil || net.ParseIP(ip).To4() == nil {
					return errors.New("IPv4 默认值不合法")
				}
			case "number":
				if _, ok := field.Default.(int); !ok {
					return errors.New("数值默认值必须是整数")
				}
			case "boolean":
				if _, ok := field.Default.(bool); !ok {
					return errors.New("布尔默认值类型不正确")
				}
			}
		}
		if field.Type == "secret" {
			if !hasTemplateKey(secretsExample, field.Key) {
				return errors.New("共享密钥模板缺少已声明的配置键")
			}
		} else if !hasTemplateKey(envExample, field.Key) {
			return errors.New("共享环境变量模板缺少已声明的配置键")
		}
	}
	if len(envExample) == 0 || len(secretsExample) == 0 || !utf8.Valid(envExample) || !utf8.Valid(secretsExample) {
		return errors.New("固定版本中的共享配置模板缺失或编码无效")
	}

	portIDs := map[string]bool{}
	for _, port := range m.Ports {
		if portIDs[port.ID] {
			return errors.New("端口标识重复")
		}
		portIDs[port.ID] = true
		if kind, ok := fields[port.HostPortKey]; !ok || kind.kind != "port" {
			return errors.New("端口引用不存在或非 port 的配置键")
		}
		if port.BindAddressKey != "" {
			if kind, ok := fields[port.BindAddressKey]; !ok || kind.kind != "ipv4" {
				return errors.New("监听地址引用不存在或非 ipv4 的配置键")
			}
		}
		value, ok := services[port.Service]
		if !ok {
			return errors.New("端口所属 Compose 服务不存在")
		}
		service, _ := asMapping(value)
		portMappings, ok := asList(service["ports"])
		if !ok {
			return errors.New("Compose 缺少声明的端口映射")
		}
		found := false
		for _, mapping := range portMappings {
			if portMappingMatches(mapping, port.HostPortKey, port.BindAddressKey, port.Protocol, port.ContainerPort) {
				found = true
				break
			}
		}
		if !found {
			return errors.New("Compose 端口映射与 Manifest 不一致")
		}
	}
	for name, action := range m.Actions {
		if action.RemoveOrphans || (action.Strategy != "script" && !(name == "uninstall" && action.Strategy == "compose_down")) {
			return errors.New("不允许的应用生命周期策略")
		}
		if len(action.Argv) > 1 {
			return errors.New("脚本参数数量超过 v1 约束")
		}
		for _, arg := range action.Argv {
			if action.Strategy != "script" || arg != name {
				return errors.New("安装脚本参数不在固定允许列表")
			}
		}
		if action.Strategy != "script" {
			continue
		}
		if name == "uninstall" && action.Path == install.Path && len(action.Argv) == 0 {
			return errors.New("共用安装脚本时必须显式传入 uninstall")
		}
		scriptDest := m.ID + "/" + action.Path
		script, exists := byDest[scriptDest]
		if !exists || !strings.HasSuffix(action.Path, ".sh") || !utf8.Valid(script) {
			return errors.New("安装或更新动作的 Shell 脚本缺失或路径不合法")
		}
		executable := false
		for _, item := range m.Package.Files {
			if item.Destination == scriptDest {
				executable = item.Mode == "0755"
				break
			}
		}
		if !executable {
			return errors.New("动作脚本没有声明 0755 可执行权限")
		}
		if bytes.Contains(script, []byte("deploy/lib/image-sources.sh")) {
			if _, ok := byDest["deploy/lib/image-sources.sh"]; !ok {
				return errors.New("缺少脚本实际引用的共享依赖")
			}
		}
		for _, match := range shellDependency.FindAllSubmatch(script, -1) {
			if _, ok := byDest[m.ID+"/deploy/"+string(match[1])]; !ok {
				return errors.New("应用动作依赖的相对脚本没有列入包")
			}
		}
	}

	mountPaths := map[string]bool{}
	for _, value := range services {
		service, _ := asMapping(value)
		volumes, ok := asList(service["volumes"])
		if !ok {
			continue
		}
		for _, item := range volumes {
			var src string
			switch v := item.(type) {
			case string:
				src = strings.SplitN(v, ":", 2)[0]
			case map[string]any:
				src, _ = v["source"].(string)
			}
			if strings.HasPrefix(src, "../") {
				mountPaths[path.Clean(path.Join(m.ID, path.Dir(m.Runtime.File), src))] = true
			}
		}
	}
	for _, item := range m.Data.Paths {
		if !validOwnedDataPath(m.ID, item) {
			return errors.New("用户数据路径必须位于应用的 data 目录")
		}
		if !mountPaths[item] {
			return errors.New("用户数据路径没有 Compose Bind Mount 对应")
		}
	}
	if !m.Data.Retain {
		return errors.New("应用卸载必须默认保留用户数据")
	}
	named, _ := asMapping(compose["volumes"])
	for _, name := range m.Data.NamedVolumes {
		if _, ok := named[name]; !ok {
			return errors.New("声明的 Docker 命名卷在 Compose 中不存在")
		}
	}
	return nil
}
