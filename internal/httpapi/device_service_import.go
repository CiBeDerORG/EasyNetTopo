package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

const maxServiceImportItems = 100

var (
	errServiceImportInventory = errors.New("无法读取主机的完整应用和端口清单，请检查主机与 Docker 状态后重试")
	errServiceImportHost      = errors.New("请选择已配对的主机")
	errServiceImportChanged   = errors.New("所选应用或发布端口已变化，请刷新应用列表后重试")
)

type serviceImportAgent interface {
	Query(context.Context, string, string) (json.RawMessage, error)
	AppAction(context.Context, string, string, any) (json.RawMessage, error)
}

type serviceImportPort struct {
	Port       int      `json:"port"`
	Protocols  []string `json:"protocols"`
	Registered bool     `json:"registered"`
}

type serviceImportApp struct {
	marketRef
	Name  string              `json:"name"`
	State string              `json:"state"`
	Ports []serviceImportPort `json:"ports"`
}

type serviceImportSelection struct {
	marketRef
	Port      int      `json:"port"`
	Protocols []string `json:"protocols"`
}

type serviceImportRequest struct {
	Items              []serviceImportSelection `json:"items"`
	ConfirmHostAddress bool                     `json:"confirm_host_address"`
}

func serviceImportIP(value string) netip.Addr {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Addr().Unmap()
	}
	ip, _ := netip.ParseAddr(value)
	return ip.Unmap()
}

func serviceImportAddressCheck(host storage.Host, device storage.Device) string {
	raw := host.Endpoint
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	endpoint, err := url.Parse(raw)
	if err != nil {
		return "unknown"
	}
	hostIP := serviceImportIP(endpoint.Hostname())
	management := serviceImportIP(device.IPAddress)
	network := serviceImportIP(device.NetworkIP)
	if !hostIP.IsValid() || (!management.IsValid() && !network.IsValid()) {
		return "unknown"
	}
	if hostIP == management || hostIP == network {
		return "matched"
	}
	return "different"
}

type serviceImportInventory struct {
	Complete  bool `json:"complete"`
	Resources []struct {
		Kind    string `json:"kind"`
		Project string `json:"compose_project"`
		Ports   []struct {
			Port     int    `json:"host_port"`
			Binding  string `json:"host_ip"`
			Protocol string `json:"protocol"`
		} `json:"ports"`
	} `json:"resources"`
}

func loadServiceImportApps(ctx context.Context, agent serviceImportAgent, host storage.Host, device storage.Device, catalog []storage.CatalogApp) ([]serviceImportApp, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	raw, err := agent.Query(ctx, host.ID, "docker")
	var inventory serviceImportInventory
	if err != nil || json.Unmarshal(raw, &inventory) != nil || !inventory.Complete {
		return nil, errServiceImportInventory
	}
	portsByProject := make(map[string]map[int][]string)
	management := serviceImportIP(device.IPAddress)
	network := serviceImportIP(device.NetworkIP)
	for _, resource := range inventory.Resources {
		if resource.Kind != "compose_project" || resource.Project == "" {
			continue
		}
		ports := portsByProject[resource.Project]
		if ports == nil {
			ports = make(map[int][]string)
			portsByProject[resource.Project] = ports
		}
		for _, published := range resource.Ports {
			if published.Port < 1 || published.Port > 65535 || (published.Protocol != "tcp" && published.Protocol != "udp") {
				continue
			}
			ip := serviceImportIP(published.Binding)
			if !ip.IsValid() || ip.IsLoopback() || (!ip.IsUnspecified() && ip != management && ip != network) {
				continue
			}
			if !slices.Contains(ports[published.Port], published.Protocol) {
				ports[published.Port] = append(ports[published.Port], published.Protocol)
			}
		}
	}

	refs := make([]marketRef, 0, len(catalog))
	names := make(map[marketRef]string, len(catalog))
	for _, app := range catalog {
		// 只查询此主机实际存在的目录项目，已受管但已下架的应用由 include_managed 补齐。
		if _, exists := portsByProject[app.AppID]; !exists {
			continue
		}
		ref := marketRef{Source: app.SourceID, App: app.AppID}
		refs = append(refs, ref)
		names[ref] = app.Name
	}
	type appStatus struct {
		marketRef
		State    string `json:"state"`
		Managed  bool   `json:"managed"`
		External bool   `json:"external"`
	}
	byProject := make(map[string]map[marketRef]appStatus)
	for start := 0; start < len(refs) || start == 0; start += 256 {
		payload := struct {
			Apps           []marketRef `json:"apps"`
			IncludeManaged bool        `json:"include_managed"`
		}{Apps: refs[start:min(start+256, len(refs))], IncludeManaged: start == 0}
		raw, err := agent.AppAction(ctx, host.ID, "status", payload)
		var response struct {
			Apps []appStatus `json:"apps"`
		}
		if err != nil || json.Unmarshal(raw, &response) != nil || response.Apps == nil {
			return nil, errServiceImportInventory
		}
		for _, app := range response.Apps {
			if app.State == "unknown" {
				return nil, errServiceImportInventory
			}
			if !(app.Managed || app.External) || app.Source == "" || app.App == "" ||
				(!slices.Contains([]string{"running", "stopped", "partial"}, app.State) && !(app.Managed && app.State == "missing")) {
				continue
			}
			// 外部项目须由两份快照确认同一 Compose 项目。受管应用即使缺少项目也可显示状态。
			if _, exists := portsByProject[app.App]; !exists && !app.Managed {
				continue
			}
			if byProject[app.App] == nil {
				byProject[app.App] = make(map[marketRef]appStatus)
			}
			byProject[app.App][app.marketRef] = app
		}
	}
	apps := make([]serviceImportApp, 0, len(byProject))
	for project, statuses := range byProject {
		rows := make([]appStatus, 0, len(statuses))
		managed := make([]appStatus, 0, 1)
		for _, row := range statuses {
			rows = append(rows, row)
			if row.Managed && !row.External {
				managed = append(managed, row)
			}
		}
		if len(managed) > 0 {
			rows = managed
		}
		if len(rows) != 1 {
			continue
		}
		row := rows[0]
		name := strings.TrimSpace(names[row.marketRef])
		if name == "" {
			name = project
		}
		app := serviceImportApp{marketRef: row.marketRef, Name: name, State: row.State, Ports: []serviceImportPort{}}
		if app.State != "missing" {
			for port, protocols := range portsByProject[project] {
				slices.Sort(protocols)
				app.Ports = append(app.Ports, serviceImportPort{Port: port, Protocols: protocols})
			}
		}
		sort.Slice(app.Ports, func(i, j int) bool { return app.Ports[i].Port < app.Ports[j].Port })
		apps = append(apps, app)
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Name == apps[j].Name {
			return apps[i].App < apps[j].App
		}
		return apps[i].Name < apps[j].Name
	})
	return apps, nil
}

func serviceImportItem(host storage.Host, app serviceImportApp, port serviceImportPort) storage.DeviceServiceImport {
	return storage.DeviceServiceImport{
		DeviceServiceInput: storage.DeviceServiceInput{Name: app.Name, Port: port.Port, Protocols: port.Protocols},
		Source:             storage.DeviceServiceSource{HostID: host.ID, HostName: host.Name, SourceID: app.Source, AppID: app.App, OriginalPort: port.Port},
	}
}

func resolveServiceImports(host storage.Host, apps []serviceImportApp, selected []serviceImportSelection) ([]storage.DeviceServiceImport, error) {
	items := make([]storage.DeviceServiceImport, 0, len(selected))
	for _, choice := range selected {
		appIndex := slices.IndexFunc(apps, func(app serviceImportApp) bool { return app.marketRef == choice.marketRef })
		if appIndex < 0 {
			return nil, errServiceImportChanged
		}
		app := apps[appIndex]
		portIndex := slices.IndexFunc(app.Ports, func(port serviceImportPort) bool {
			return port.Port == choice.Port && slices.Equal(port.Protocols, choice.Protocols)
		})
		if portIndex < 0 {
			return nil, errServiceImportChanged
		}
		items = append(items, serviceImportItem(host, app, app.Ports[portIndex]))
	}
	return items, nil
}
