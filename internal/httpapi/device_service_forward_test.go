package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

const (
	testForwardHost     = "38fd6fdd-1252-4813-9029-b703268a8297"
	testForwardInstance = "1ebbe409-9817-432e-be00-1ed978b1adc0"
	testForwardRoute    = `{"data":{"node_info":{"inst_id":"` + testForwardInstance + `","hostname":"yun","peer_id":1},"peer_routes":[{"route":{"hostname":"OWRT","peer_id":2,"ipv4_addr":{"address":{"addr":168430083}},"proxy_cidrs":["10.10.30.0/24"]}}]}}`
	testForwardTCP      = `{"bind_addr":{"port":7017,"ip":{"Ipv4":{"addr":0}}},"dst_addr":{"port":7017,"ip":{"Ipv4":{"addr":168435362}}},"socket_type":0}`
)

type testForwardAgent struct {
	selected string
	all      string
	commands []string
	failUDP  bool
	readErr  bool
}

func (a *testForwardAgent) NetworkData(context.Context, string, string) (json.RawMessage, error) {
	return json.RawMessage(testForwardRoute), nil
}

func (a *testForwardAgent) EasyTierPortForwards(_ context.Context, _, method, instance, _ string, change *hosts.PortForwardChange) (json.RawMessage, error) {
	if method == http.MethodGet {
		if a.readErr {
			return nil, errors.New("Agent 未连接")
		}
		rules := a.selected
		if instance == "" {
			rules = a.all
		}
		return json.RawMessage(`{"data":{"cfgs":[` + rules + `]}}`), nil
	}
	a.commands = append(a.commands, method+" "+change.Protocol+" "+change.Listen+" "+change.Destination)
	if a.failUDP && method == http.MethodPost && change.Protocol == "udp" {
		return nil, errors.New("监听失败")
	}
	return json.RawMessage(`{"applied":true}`), nil
}

func TestEnableDeviceServiceForwardRequiresCommandSuccess(t *testing.T) {
	device := storage.Device{NodeHostname: "OWRT", NetworkIP: "10.10.30.162"}
	service := storage.DeviceService{DeviceServiceInput: storage.DeviceServiceInput{Port: 7017, Protocols: []string{"tcp", "udp"}}}
	input := serviceForwardInput{Enabled: true, HostID: testForwardHost, InstanceID: testForwardInstance,
		Listen: "0.0.0.0:7017", AllowPublicBind: true}
	t.Run("执行两条协议后返回开启状态", func(t *testing.T) {
		agent := &testForwardAgent{}
		cfg, err := updateDeviceServiceForward(context.Background(), agent, device, service, input)
		if err != nil || cfg == nil || !cfg.Enabled || cfg.Destination != "10.10.30.162:7017" ||
			!reflect.DeepEqual(agent.commands, []string{"POST tcp 0.0.0.0:7017 10.10.30.162:7017", "POST udp 0.0.0.0:7017 10.10.30.162:7017"}) {
			t.Fatalf("开启状态或执行命令不符: %+v %v %v", cfg, err, agent.commands)
		}
	})
	t.Run("部分失败恢复本次新增并保持关闭", func(t *testing.T) {
		agent := &testForwardAgent{failUDP: true}
		cfg, err := updateDeviceServiceForward(context.Background(), agent, device, service, input)
		if err == nil || cfg != nil || len(agent.commands) != 3 || !strings.HasPrefix(agent.commands[2], "DELETE tcp ") {
			t.Fatalf("未回滚部分规则: %+v %v %v", cfg, err, agent.commands)
		}
	})
	t.Run("不同实例或手工规则占用的端口不可写", func(t *testing.T) {
		agent := &testForwardAgent{all: testForwardTCP}
		cfg, err := updateDeviceServiceForward(context.Background(), agent, device, service, input)
		if !errors.Is(err, errForwardConflict) || cfg != nil || len(agent.commands) != 0 {
			t.Fatalf("端口冲突时执行了写入: %+v %v %v", cfg, err, agent.commands)
		}
	})
}

func TestEditServiceChecksActualDestination(t *testing.T) {
	service := storage.DeviceService{
		DeviceServiceInput: storage.DeviceServiceInput{Port: 7017, Protocols: []string{"tcp"}},
		Forward: &storage.DeviceServiceForward{Enabled: true, HostID: testForwardHost, InstanceID: testForwardInstance,
			Listen: "0.0.0.0:7017", Destination: "10.10.30.162:7017", Protocols: []string{"tcp"}},
	}
	for _, tc := range []struct {
		name, ip, rules  string
		offline, enabled bool
	}{
		{"回读与当前目标一致", "10.10.30.162", testForwardTCP, false, true},
		{"设备地址改变", "10.10.30.163", testForwardTCP, false, false},
		{"转发指向其他目标", "10.10.30.162", strings.Replace(testForwardTCP, "168435362", "168435363", 1), false, false},
		{"规则已删除", "10.10.30.162", "", false, false},
		{"Agent 不在线不能判断", "10.10.30.162", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := &testForwardAgent{selected: tc.rules, readErr: tc.offline}
			device := storage.Device{NodeHostname: "OWRT", NetworkIP: tc.ip}
			cfg, err := checkDeviceServiceForward(context.Background(), agent, device, service)
			if tc.offline {
				if !errors.Is(err, errForwardNotReady) || cfg != nil || !service.Forward.Enabled {
					t.Fatalf("查询失败时不应覆盖原状态: %+v %v", cfg, err)
				}
				return
			}
			if err != nil || cfg == nil || cfg.Enabled != tc.enabled || len(agent.commands) != 0 {
				t.Fatalf("编辑校验状态不符或触发了远端写入: %+v %v %v", cfg, err, agent.commands)
			}
		})
	}
}
