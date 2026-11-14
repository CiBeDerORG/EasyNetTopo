package httpapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGzipCompressesFrontendAssets(t *testing.T) {
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0600); err != nil {
		t.Fatal(err)
	}

	payload := []byte(strings.Repeat("export const appName = 'EasyNetTopo';\n", 256))
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), payload, 0600); err != nil {
		t.Fatal(err)
	}

	handler := testHandler(t, dir)
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip;q=1")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("静态资源状态码：%d", res.Code)
	}
	if got := res.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("静态 JS 应使用 Gzip，Content-Encoding=%q", got)
	}
	if got := res.Header().Get("Content-Length"); got != "" {
		t.Fatalf("压缩响应不应保留原始 Content-Length，得到 %q", got)
	}
	if !strings.Contains(strings.ToLower(res.Header().Get("Vary")), "accept-encoding") {
		t.Fatalf("缺少 Vary: Accept-Encoding：%q", res.Header().Get("Vary"))
	}

	reader, err := gzip.NewReader(bytes.NewReader(res.Body.Bytes()))
	if err != nil {
		t.Fatalf("创建 Gzip Reader：%v", err)
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("解压静态 JS：%v", err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatal("解压后的 JS 与原始文件不一致")
	}
	if len(res.Body.Bytes()) >= len(payload) {
		t.Fatalf("测试资源没有得到压缩：原始=%d，压缩=%d", len(payload), res.Body.Len())
	}
}

func TestGzipCompressesJSONAPI(t *testing.T) {
	handler := testHandler(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("健康接口状态码：%d", res.Code)
	}
	if got := res.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("JSON API 应使用 Gzip，Content-Encoding=%q", got)
	}

	reader, err := gzip.NewReader(bytes.NewReader(res.Body.Bytes()))
	if err != nil {
		t.Fatalf("创建 Gzip Reader：%v", err)
	}
	defer reader.Close()

	var body map[string]any
	if err := json.NewDecoder(reader).Decode(&body); err != nil {
		t.Fatalf("解压并解析 JSON：%v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("健康接口内容异常：%v", body)
	}
}

func TestGzipRespectsQualityAndRangeRequests(t *testing.T) {
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0600); err != nil {
		t.Fatal(err)
	}
	payload := []byte(strings.Repeat("const value = 'range-safe';\n", 128))
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	handler := testHandler(t, dir)

	t.Run("explicit gzip q zero overrides wildcard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		req.Header.Set("Accept-Encoding", "gzip;q=0, *;q=1")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		if got := res.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("gzip;q=0 时不应压缩，Content-Encoding=%q", got)
		}
		if !strings.Contains(strings.ToLower(res.Header().Get("Vary")), "accept-encoding") {
			t.Fatalf("缺少 Vary: Accept-Encoding：%q", res.Header().Get("Vary"))
		}
		if !bytes.Equal(res.Body.Bytes(), payload) {
			t.Fatal("未压缩响应内容与原始 JS 不一致")
		}
	})

	t.Run("range response remains uncompressed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Range", "bytes=0-31")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		if res.Code != http.StatusPartialContent {
			t.Fatalf("Range 请求应返回 206，得到 %d", res.Code)
		}
		if got := res.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Range 响应不应压缩，Content-Encoding=%q", got)
		}
		if got := res.Header().Get("Content-Range"); got == "" {
			t.Fatal("Range 响应缺少 Content-Range")
		}
	})
}
