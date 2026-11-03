package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CiBeDerORG/EasyNetTopo/internal/storage"
)

func TestAdminFromStdinCustomUserAndOneTimeOnly(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user, password := "中文 用户 @特殊账号", strings.Repeat("超长 密码 #$!", 50)
	if err := initAdminFromStdin(db, strings.NewReader(user+"\n"+password+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := db.Verify(context.Background(), user, password); err != nil {
		t.Fatalf("自定义管理员无法登录: %v", err)
	}
	if err := db.Verify(context.Background(), "admin", password); err == nil {
		t.Fatal("默认 admin 不应自动拥有同一密码")
	}
	if err := initAdminFromStdin(db, strings.NewReader("attacker\nOther-Strong-Password\n")); !errors.Is(err, storage.ErrInitialized) {
		t.Fatalf("第二次初始化必须拒绝覆盖，得到 %v", err)
	}
	if err := db.Verify(context.Background(), user, password); err != nil {
		t.Fatalf("重复初始化改变了已有管理员: %v", err)
	}
}

func TestAdminFromStdinInvalidInputsDoNotInitialize(t *testing.T) {
	for _, tc := range []string{
		"admin\nPassword-With-CR\r\n",
		"admin\nValid-Password-123\nextra",
		"admin\nValid-Password-123",
		"admin\nValid-Password-123\n\n",
		"admin\nValid-Password-123\x00\n",
		strings.Repeat("A", 1<<20+1),
	} {
		db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
		if err != nil {
			t.Fatal(err)
		}
		if err := initAdminFromStdin(db, strings.NewReader(tc)); err == nil {
			t.Fatalf("非法输入被接受: %q", tc[:min(len(tc), 25)])
		}
		ready, err := db.Initialized(context.Background())
		if err != nil || ready {
			t.Fatalf("失败后数据库应保持未初始化: %v %v", ready, err)
		}
		_ = db.Close()
	}
}

func TestAdminFromStdinAllowsShortAndLongCredentials(t *testing.T) {
	cases := []struct{ user, pass string }{
		{"1", "1"},
		{strings.Repeat("用", 150), strings.Repeat("密", 500)},
		{"包含空格 和标点@!", "空格&$# 中文!?"},
	}
	for _, tc := range cases {
		db, err := storage.Open(filepath.Join(t.TempDir(), "app.db"))
		if err != nil {
			t.Fatal(err)
		}
		input := tc.user + "\n" + tc.pass + "\n"
		if err := initAdminFromStdin(db, strings.NewReader(input)); err != nil {
			t.Fatal(err)
		}
		if err := db.Verify(context.Background(), tc.user, tc.pass); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
	}
}
