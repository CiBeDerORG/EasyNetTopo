package catalog

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/packagebuilder"
	"gopkg.in/yaml.v3"
)

// 仅生成无真实密钥的测试资源；不得将私有 Deploy 的实际脚本拷贝到公共仓库。
func syntheticDeployFixture(t testing.TB) ([]byte, []packagebuilder.Resource, []byte, []byte) {
	t.Helper()
	raw, err := os.ReadFile("testdata/sub2api.app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest semanticManifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	var compose strings.Builder
	fmt.Fprintf(&compose, "name: %s\nservices:\n", manifest.Runtime.Project)
	for _, service := range manifest.Verification.Required {
		fmt.Fprintf(&compose, "  %s:\n    image: example/placeholder\n    healthcheck:\n      test: [CMD, true]\n", service)
		if service == "sub2api" {
			compose.WriteString("    ports:\n")
			for _, port := range manifest.Ports {
				if port.Service != service {
					continue
				}
				prefix := ""
				if port.BindAddressKey != "" {
					prefix = "${" + port.BindAddressKey + "}:"
				}
				fmt.Fprintf(&compose, "      - \"%s${%s}:%d\"\n", prefix, port.HostPortKey, port.ContainerPort)
			}
			compose.WriteString("    volumes:\n")
			for _, volume := range manifest.Data.Paths {
				sub := strings.TrimPrefix(volume, manifest.ID+"/data/")
				fmt.Fprintf(&compose, "      - \"../data/%s:/data/%s\"\n", sub, sub)
			}
		}
	}
	installScript := "#!/bin/sh\nsource \"$DEPLOY_DIR/lib.sh\"\n# deploy/lib/image-sources.sh\n"
	for _, config := range manifest.Config {
		if config.GeneratedByScript {
			installScript += "ensure_hex_secret " + config.Key + "\n"
		}
	}
	updateScript := "#!/bin/sh\nsource \"$DEPLOY_DIR/lib.sh\"\n"
	resources := make([]packagebuilder.Resource, 0, len(manifest.Package.Files))
	for _, file := range manifest.Package.Files {
		value := []byte("# generated test-only resource\n")
		switch file.Source {
		case manifest.ID + "/deploy/docker-compose.yaml":
			value = []byte(compose.String())
		case manifest.ID + "/deploy/install.sh":
			value = []byte(installScript)
		case manifest.ID + "/deploy/update.sh":
			value = []byte(updateScript)
		}
		resources = append(resources, packagebuilder.Resource{
			Source: file.Source, Destination: file.Destination, Mode: file.Mode, Content: value,
		})
	}
	var env, secrets strings.Builder
	for _, config := range manifest.Config {
		if config.Type == "secret" {
			fmt.Fprintf(&secrets, "%s=\n", config.Key)
		} else {
			fmt.Fprintf(&env, "%s=\n", config.Key)
		}
	}
	return raw, resources, []byte(env.String()), []byte(secrets.String())
}

func TestDeployV1SemanticAcceptsSyntheticSub2API(t *testing.T) {
	raw, resources, env, secrets := syntheticDeployFixture(t)
	if err := ValidateManifestSemanticsV1(raw, resources, env, secrets); err != nil {
		t.Fatal("合约一致的测试应用未通过语义验证: ", err)
	}
}

func TestDeployV1SemanticRejectsUnsafeContracts(t *testing.T) {
	raw, resources, env, secrets := syntheticDeployFixture(t)
	type fixture struct {
		manifest []byte
		files    []packagebuilder.Resource
		env      []byte
		secrets  []byte
	}
	cases := []struct {
		name   string
		mutate func(*fixture)
	}{
		{"端口默认值越界但 Schema 允许", func(v *fixture) {
			v.manifest = []byte(strings.Replace(string(v.manifest), "default: 7010", "default: 70000", 1))
		}},
		{"Compose 项目名被修改", func(v *fixture) {
			changeResource(v.files, "docker-compose.yaml", "name: sub2api", "name: other")
		}},
		{"健康检查服务不存在", func(v *fixture) {
			changeResource(v.files, "docker-compose.yaml", "  postgres:", "  missing-postgres:")
		}},
		{"Compose 端口与 Manifest 不一致", func(v *fixture) {
			changeResource(v.files, "docker-compose.yaml", "}:8080", "}:9999")
		}},
		{"Compose 数据挂载缺失", func(v *fixture) {
			changeResource(v.files, "docker-compose.yaml", "../data/redis:", "../data/unowned:")
		}},
		{"缺少共享环境变量配置", func(v *fixture) { v.env = []byte("UNRELATED=\n") }},
		{"缺少共享密钥配置", func(v *fixture) { v.secrets = []byte("UNRELATED=\n") }},
		{"共享模板不能代替运行时密钥", func(v *fixture) { v.secrets = nil }},
		{"自动生成密钥不在脚本内", func(v *fixture) {
			changeResource(v.files, "deploy/install.sh", "ensure_hex_secret SUB2API_JWT_SECRET", "echo skip")
		}},
		{"源脚本依赖未打包", func(v *fixture) {
			changeResource(v.files, "deploy/install.sh", "#!/bin/sh", "#!/bin/sh\nsource \"$DEPLOY_DIR/missing.sh\"")
		}},
		{"未批准的动作参数", func(v *fixture) {
			v.manifest = []byte(strings.Replace(string(v.manifest),
				"    path: deploy/update.sh", "    path: deploy/update.sh\n    argv: [unsafe]", 1))
		}},
		{"不允许移除孤儿容器", func(v *fixture) {
			v.manifest = []byte(strings.Replace(string(v.manifest),
				"    remove_orphans: false", "    remove_orphans: true", 1))
		}},
		{"资源包文件缺失", func(v *fixture) { v.files = v.files[:len(v.files)-1] }},
		{"声明不存在的 Compose 命名卷", func(v *fixture) {
			v.manifest = []byte(strings.Replace(string(v.manifest),
				"named_volumes: []", "named_volumes: [unknown-volume]", 1))
		}},
		{"Compose YAML 重复字段", func(v *fixture) {
			changeResource(v.files, "docker-compose.yaml", "services:", "services: {}\nservices:")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copyFiles := make([]packagebuilder.Resource, len(resources))
			for index, file := range resources {
				copyFiles[index] = file
				copyFiles[index].Content = append([]byte(nil), file.Content...)
			}
			value := fixture{manifest: append([]byte(nil), raw...), files: copyFiles,
				env: append([]byte(nil), env...), secrets: append([]byte(nil), secrets...)}
			tc.mutate(&value)
			if err := ValidateManifestSemanticsV1(value.manifest, value.files, value.env, value.secrets); err == nil {
				t.Fatal("不一致或危险的 Deploy 语义输入被放行")
			}
		})
	}
}

func changeResource(resources []packagebuilder.Resource, suffix, old, replacement string) {
	for index := range resources {
		if path.Base(resources[index].Source) == path.Base(suffix) && strings.HasSuffix(resources[index].Source, suffix) {
			resources[index].Content = []byte(strings.Replace(string(resources[index].Content), old, replacement, 1))
			return
		}
	}
}
