package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/httpapi"
)

func main() {
	addr := os.Getenv("EASYNETTOPO_LISTEN_ADDRESS")
	if addr == "" {
		// 在身份认证尚未实现前，仅允许本机访问，避免暴露未受保护的管理界面。
		addr = "127.0.0.1:8080"
	}
	webDir := os.Getenv("EASYNETTOPO_WEB_DIR")
	if webDir == "" {
		webDir = "web/dist"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandler(webDir),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("服务退出失败", "error", err)
		}
	}()

	slog.Info("EasyNetTopo 启动", "listen", addr, "web_dir", webDir)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("HTTP 服务异常退出", "error", err)
		os.Exit(1)
	}
}
