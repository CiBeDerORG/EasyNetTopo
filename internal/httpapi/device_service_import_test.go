package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

type testServiceImportAgent struct {
	docker json.RawMessage
	status json.RawMessage
}

func (agent testServiceImportAgent) Query(context.Context, string, string) (json.RawMessage, error) {
	return agent.docker, nil
}
func (agent testServiceImportAgent) AppAction(context.Context, string, string, any) (json.RawMessage, error) {
	return agent.status, nil
}

func TestServiceImportUsesPublishedHostPorts(t *testing.T) {
	ctx := context.Background()
	agent := testServiceImportAgent{
		docker: json.RawMessage(`{"complete":true,"resources":[
			{"kind":"compose_project","compose_project":"nas","ports":[
				{"host_port":15666,"container_port":5666,"host_ip":"0.0.0.0","protocol":"tcp"},
				{"host_port":15666,"container_port":5666,"host_ip":"::","protocol":"tcp"},
				{"host_port":15666,"host_ip":"192.168.10.2","protocol":"udp"},
				{"host_port":8888,"host_ip":"10.10.30.2","protocol":"tcp"},
				{"host_port":9999,"host_ip":"192.168.10.3","protocol":"udp"},
				{"host_port":1234,"host_ip":"127.0.0.1","protocol":"tcp"}]},
			{"kind":"compose_project","compose_project":"photos","ports":[]}]}`),
		status: json.RawMessage(`{"apps":[
			{"source_id":"other","app_id":"nas","state":"running","external":true},
			{"source_id":"source","app_id":"nas","state":"running","managed":true},
			{"source_id":"source","app_id":"photos","state":"stopped","managed":true},
			{"source_id":"source","app_id":"missing","state":"missing","managed":true}]}`),
	}
	host := storage.Host{ID: "host", Name: "测试主机", Endpoint: "192.168.10.2:7007"}
	device := storage.Device{IPAddress: "192.168.10.2", NetworkIP: "10.10.30.2"}
	catalog := []storage.CatalogApp{{SourceID: "source", AppID: "nas", Name: "文件服务"}}
	apps, err := loadServiceImportApps(ctx, agent, host, device, catalog)
	if err != nil || len(apps) != 3 {
		t.Fatalf("已安装应用识别错误: %+v %v", apps, err)
	}
	index := slices.IndexFunc(apps, func(app serviceImportApp) bool { return app.App == "nas" })
	if index < 0 {
		t.Fatalf("缺少 NAS 应用: %+v", apps)
	}
	nas := apps[index]
	if nas.Source != "source" || nas.Name != "文件服务" || len(nas.Ports) != 2 || nas.Ports[0].Port != 8888 ||
		nas.Ports[1].Port != 15666 || !slices.Equal(nas.Ports[1].Protocols, []string{"tcp", "udp"}) {
		t.Fatalf("主机端口或应用来源错误: %+v", nas)
	}
	if serviceImportAddressCheck(host, device) != "matched" {
		t.Fatal("主机与设备的地址应匹配")
	}
	selection := serviceImportSelection{marketRef: nas.marketRef, Port: 15666, Protocols: []string{"tcp", "udp"}}
	items, err := resolveServiceImports(host, apps, []serviceImportSelection{selection})
	if err != nil || len(items) != 1 || items[0].Port != 15666 || items[0].Source.HostID != "host" {
		t.Fatalf("导入项没有保留实际端口和来源: %+v %v", items, err)
	}
	selection.Port = 5666
	if _, err = resolveServiceImports(host, apps, []serviceImportSelection{selection}); !errors.Is(err, errServiceImportChanged) {
		t.Fatalf("容器内部端口不应通过导入校验: %v", err)
	}
	agent.docker = json.RawMessage(`{"complete":false,"resources":[]}`)
	if _, err := loadServiceImportApps(ctx, agent, host, device, catalog); !errors.Is(err, errServiceImportInventory) {
		t.Fatalf("清单不完整应报错: %v", err)
	}
}
