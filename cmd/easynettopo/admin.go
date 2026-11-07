package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

// initAdminFromStdin 供受信任的本机部署脚本首次初始化。
// stdin 仅包含两行：用户名、密码。不能从环境变量、命令行参数或 HTTP 请求传递口令。
func initAdminFromStdin(db *storage.Store, input io.Reader) error {
	if db == nil || input == nil {
		return errors.New("初始化输入无效")
	}
	// stdin 是受信任的本地部署入口，仅限制传输总量以防止无界内存分配。
	const maxInputBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(input, maxInputBytes+1))
	if err != nil || len(raw) > maxInputBytes {
		return errors.New("初始化凭据输入超出传输容量或读取失败")
	}
	lines := bytes.Split(raw, []byte{'\n'})
	if len(lines) != 3 || len(lines[2]) != 0 || bytes.IndexByte(raw, '\r') >= 0 ||
		bytes.IndexByte(raw, 0) >= 0 || !utf8.Valid(raw) {
		return errors.New("初始化凭据格式错误（只允许用户名和密码各占一行）")
	}
	return db.CreateAdmin(context.Background(), string(lines[0]), string(lines[1]))
}
