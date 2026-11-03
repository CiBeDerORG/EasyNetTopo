package storage

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"testing"
)

func TestAdminSessionAndControllerIdentityPersist(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state", "app.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	id := s.ControllerID()
	initialized, err := s.Initialized(ctx)
	if err != nil || initialized {
		t.Fatal("数据库初始化状态错误", err)
	}
	if err := s.CreateAdmin(ctx, "admin", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateAdmin(ctx, "admin", "change-without-auth"); !errors.Is(err, ErrInitialized) {
		t.Fatal("允许覆盖管理员", err)
	}
	if s.Verify(ctx, "admin", "wrong") == nil {
		t.Fatal("错误密码通过")
	}
	if err := s.Verify(ctx, "admin", "correct-horse-battery"); err != nil {
		t.Fatal(err)
	}
	token, _, err := s.NewSession(ctx)
	if err != nil || !s.HasSession(ctx, token) {
		t.Fatal("会话创建失败", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.ControllerID() != id || !s.HasSession(ctx, token) {
		t.Fatal("身份或会话未持久化")
	}
	if err := s.Revoke(ctx, token); err != nil {
		t.Fatal(err)
	}
	if s.HasSession(ctx, token) {
		t.Fatal("注销失败")
	}
}
func TestBootstrapGuards(t *testing.T) {
	if _, err := Open("relative.db"); err == nil {
		t.Fatal("允许相对路径")
	}
}

func TestAdminCredentialsNoCustomLimits(t *testing.T) {
	cases := []struct{ name, user, password string }{
		{"单字账号和短密码", "1", "1"},
		{"中文空格标点", "中文 用户 !@#$%^&*()", "密 码🗝️\"'!"},
		{"长账号和长密码", strings.Repeat("用", 150), strings.Repeat("密码", 300)},
		{"bcrypt 72 字节边界", "任意-账户", strings.Repeat("x", 72) + "末尾"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "app.db"))
			if err != nil { t.Fatal(err) }
			defer s.Close()
			ctx := context.Background()
			if err := s.CreateAdmin(ctx, tc.user, tc.password); err != nil { t.Fatal(err) }
			if err := s.Verify(ctx, tc.user, tc.password); err != nil { t.Fatalf("原样密码无法登录: %v", err) }
			if err := s.Verify(ctx, tc.user, tc.password+"错误后缀"); !errors.Is(err, ErrCredentials) {
				t.Fatalf("密码后缀改变仍通过验证: %v", err)
			}
		})
	}
}

func TestLegacyBcryptPasswordCanStillLogin(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil { t.Fatal(err) }
	defer s.Close()
	password := "legacy-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil { t.Fatal(err) }
	if _, err := s.db.Exec("INSERT INTO administrators(id,username,password_hash) VALUES(1,?,?)", "旧账户", string(hash)); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(context.Background(), "旧账户", password); err != nil {
		t.Fatalf("旧密码不能登录: %v", err)
	}
}
