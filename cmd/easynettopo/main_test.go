package main

import (
	"testing"
)

func TestCheckLocalBind(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		allow   string
		wantErr bool
	}{
		{name: "默认允许本机 IPv4", addr: "127.0.0.1:8080"},
		{name: "默认允许本机 IPv6", addr: "[::1]:8080"},
		{name: "默认允许 localhost", addr: "localhost:8080"},
		{name: "默认拒绝容器所有接口", addr: "0.0.0.0:8080", wantErr: true},
		{name: "显式开启容器内监听", addr: "0.0.0.0:8080", allow: "true"},
		{name: "错误开关不开放", addr: "0.0.0.0:8080", allow: "TRUE", wantErr: true},
		{name: "开关不允许指定外部 IPv4", addr: "192.0.2.1:8080", allow: "true", wantErr: true},
		{name: "开关不允许 IPv6 通配", addr: "[::]:8080", allow: "true", wantErr: true},
		{name: "拒绝非法地址", addr: "not-an-address", allow: "true", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EASYNETTOPO_ALLOW_CONTAINER_BIND", tt.allow)
			err := checkLocalBind(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkLocalBind(%q) 错误=%v，期望错误=%v", tt.addr, err, tt.wantErr)
			}
		})
	}
}
