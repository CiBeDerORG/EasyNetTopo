package httpapi

import (
	"bufio"
	"compress/gzip"
	"errors"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// gzipMiddleware 为可压缩的 HTTP 响应启用 Gzip。
// 它同时覆盖前端静态资源和 API 响应，不改变业务 Handler。
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &gzipResponseWriter{
			ResponseWriter: w,
			request:        r,
			acceptsGzip:    acceptsGzip(r.Header.Get("Accept-Encoding")),
		}
		defer wrapped.finish()
		next.ServeHTTP(wrapped, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	request     *http.Request
	acceptsGzip bool
	wroteHeader bool
	gzipWriter  *gzip.Writer
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	// 允许先发送 1xx 提示响应，再发送最终响应。
	if statusCode >= 100 && statusCode < 200 && statusCode != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(statusCode)
		return
	}
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	header := w.Header()
	contentType := header.Get("Content-Type")
	if isCompressibleContentType(contentType) && header.Get("Content-Encoding") == "" {
		addVary(header, "Accept-Encoding")
	}

	if w.shouldCompress(statusCode, contentType) {
		header.Set("Content-Encoding", "gzip")
		header.Del("Content-Length")
		// 压缩后的字节范围与原文件不同，避免声明支持原始文件的 Range。
		header.Set("Accept-Ranges", "none")
	}

	w.ResponseWriter.WriteHeader(statusCode)
	if header.Get("Content-Encoding") == "gzip" && w.acceptsGzip {
		w.gzipWriter = gzip.NewWriter(w.ResponseWriter)
	}
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.gzipWriter != nil {
		return w.gzipWriter.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *gzipResponseWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.gzipWriter != nil {
		_ = w.gzipWriter.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.gzipWriter != nil {
		if err := w.gzipWriter.Close(); err != nil {
			return nil, nil, err
		}
		w.gzipWriter = nil
	}
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

// Unwrap 使 http.ResponseController 能访问底层 ResponseWriter。
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *gzipResponseWriter) shouldCompress(statusCode int, contentType string) bool {
	if !w.acceptsGzip || w.request.Method != http.MethodGet {
		return false
	}
	if statusCode < 200 || statusCode == http.StatusNoContent ||
		statusCode == http.StatusNotModified || statusCode == http.StatusPartialContent {
		return false
	}
	if w.request.Header.Get("Range") != "" || w.Header().Get("Content-Range") != "" {
		return false
	}
	if w.Header().Get("Content-Encoding") != "" {
		return false
	}
	if strings.Contains(strings.ToLower(w.Header().Get("Cache-Control")), "no-transform") {
		return false
	}
	if !isCompressibleContentType(contentType) {
		return false
	}
	// 已知很小的响应不压缩，避免 Gzip 头部开销反而增加传输量。
	if value := w.Header().Get("Content-Length"); value != "" {
		if size, err := strconv.ParseInt(value, 10, 64); err == nil && size < 256 {
			return false
		}
	}
	return true
}

func acceptsGzip(header string) bool {
	gzipQuality := -1.0
	wildcardQuality := -1.0

	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		coding := strings.ToLower(strings.TrimSpace(parts[0]))
		if coding == "" {
			continue
		}

		quality := 1.0
		for _, parameter := range parts[1:] {
			name, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				quality = 0
			} else {
				quality = parsed
			}
		}

		switch coding {
		case "gzip":
			gzipQuality = quality
		case "*":
			wildcardQuality = quality
		}
	}

	if gzipQuality >= 0 {
		return gzipQuality > 0
	}
	return wildcardQuality > 0
}

func isCompressibleContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)

	if strings.HasPrefix(mediaType, "text/") {
		return mediaType != "text/event-stream"
	}

	switch mediaType {
	case "application/javascript",
		"application/x-javascript",
		"application/json",
		"application/ld+json",
		"application/manifest+json",
		"application/problem+json",
		"application/xml",
		"application/xhtml+xml",
		"application/wasm",
		"image/svg+xml":
		return true
	default:
		return false
	}
}

func addVary(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for _, item := range strings.Split(existing, ",") {
			item = strings.TrimSpace(item)
			if item == "*" || strings.EqualFold(item, value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func (w *gzipResponseWriter) finish() {
	if w.gzipWriter != nil {
		_ = w.gzipWriter.Close()
		w.gzipWriter = nil
	}
}
