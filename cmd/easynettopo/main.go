package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CiBeDerORG/EasyNetTopo/internal/catalog"
	"github.com/CiBeDerORG/EasyNetTopo/internal/hosts"
	"github.com/CiBeDerORG/EasyNetTopo/internal/httpapi"
	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		slog.Error("EasyNetTopo 运行失败", "error", err)
		os.Exit(1)
	}
}
func run() error {
	dataPath := os.Getenv("EASYNETTOPO_DATABASE_PATH")
	if dataPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		dataPath = filepath.Join(dir, "easynettopo", "app.db")
	}
	db, err := storage.Open(dataPath)
	if err != nil {
		return fmt.Errorf("打开数据库: %w", err)
	}
	defer db.Close()
	if len(os.Args) > 1 {
		if len(os.Args) != 3 || os.Args[1] != "admin" || os.Args[2] != "init" {
			return errors.New("支持命令：easynettopo admin init")
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("管理员初始化需要交互式终端")
		}
		fmt.Fprint(os.Stderr, "输入 admin 密码（12–72 字节）：")
		pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stderr, "再次输入密码：")
		confirm, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		if string(pwd) != string(confirm) {
			return errors.New("两次密码不一致")
		}
		if err := db.CreateAdmin(context.Background(), "admin", string(pwd)); err != nil {
			return err
		}
		fmt.Println("管理员初始化成功")
		return nil
	}
	listen := os.Getenv("EASYNETTOPO_LISTEN_ADDRESS")
	if listen == "" {
		listen = "127.0.0.1:8080"
	}
	if err := checkLocalBind(listen); err != nil {
		return err
	}
	webDir := os.Getenv("EASYNETTOPO_WEB_DIR")
	if webDir == "" {
		webDir = "web/dist"
	}
	catalogService, err := catalog.NewService(context.Background(), db)
	if err != nil {
		return fmt.Errorf("初始化应用商店失败: %w", err)
	}
	hostService, err := hosts.NewService(context.Background(), db)
	if err != nil {
		return fmt.Errorf("初始化主机管理失败: %w", err)
	}
	server := &http.Server{Addr: listen, Handler: httpapi.NewHandlerWithHosts(webDir, db, catalogService, hostService), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(cancelCtx); err != nil {
			slog.Error("HTTP 退出失败", "error", err)
		}
	}()
	slog.Info("EasyNetTopo 启动", "listen", listen, "database", dataPath)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func checkLocalBind(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return errors.New("仅支持环回监听，请通过受保护的 HTTPS 反向代理访问")
}
