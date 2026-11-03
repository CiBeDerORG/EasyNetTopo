package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var ErrInitialized = errors.New("管理员已创建")
var ErrCredentials = errors.New("账号或密码错误")
const passwordHashPrefix = "sha256-bcrypt:"

type Store struct {
	db           *sql.DB
	controllerID string
	path         string
}

func Open(path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("数据库路径必须为绝对路径")
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if f, err := os.Lstat(path); err == nil {
		if !f.Mode().IsRegular() {
			return nil, errors.New("数据库目标必须是普通文件")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	queries := []string{
		"PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL",
		"CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS administrators(id INTEGER PRIMARY KEY CHECK(id=1),username TEXT UNIQUE NOT NULL,password_hash TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS sessions(token_hash TEXT PRIMARY KEY,expires_at INTEGER NOT NULL)",
		"CREATE INDEX IF NOT EXISTS sessions_expiry ON sessions(expires_at)",
	}
	for _, q := range queries {
		if _, err = db.Exec(q); err != nil {
			return fail(err)
		}
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fail(err)
	}
	id, err := uuid()
	if err != nil {
		return fail(err)
	}
	if _, err = db.Exec("INSERT OR IGNORE INTO settings(key,value) VALUES('controller_id',?)", id); err != nil {
		return fail(err)
	}
	if err = db.QueryRow("SELECT value FROM settings WHERE key='controller_id'").Scan(&id); err != nil {
		return fail(err)
	}
	return &Store{db: db, controllerID: id, path: path}, nil
}

func (s *Store) Close() error         { return s.db.Close() }
func (s *Store) ControllerID() string { return s.controllerID }
func (s *Store) Initialized(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM administrators").Scan(&n)
	return n > 0, err
}
func (s *Store) CreateAdmin(ctx context.Context, user, password string) error {
	// 完整密码先做 SHA-256 再交给 bcrypt，避免 bcrypt 的 72 字节输入上限。
	// 用户名和密码均不额外限制长度、字符种类或格式。
	sum := sha256.Sum256([]byte(password))
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(sum[:])), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO administrators(id,username,password_hash) VALUES(1,?,?)", user, passwordHashPrefix+string(hash))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInitialized
	}
	return nil
}
func (s *Store) Verify(ctx context.Context, user, password string) error {
	var hash string
	if s.db.QueryRowContext(ctx, "SELECT password_hash FROM administrators WHERE id=1 AND username=?", user).Scan(&hash) != nil {
		return ErrCredentials
	}
	if strings.HasPrefix(hash, passwordHashPrefix) {
		// 新账号校验完整密码；旧版本的原始 bcrypt 哈希仍可直接验证。
		sum := sha256.Sum256([]byte(password))
		hash = strings.TrimPrefix(hash, passwordHashPrefix)
		password = hex.EncodeToString(sum[:])
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return ErrCredentials
	}
	return nil
}
func (s *Store) NewSession(ctx context.Context) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expires := time.Now().UTC().Add(12 * time.Hour)
	_, err := s.db.ExecContext(ctx, "INSERT INTO sessions(token_hash,expires_at) VALUES(?,?)", digest(token), expires.Unix())
	return token, expires, err
}
func (s *Store) HasSession(ctx context.Context, token string) bool {
	if len(token) != 43 {
		return false
	}
	var n int
	return s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE token_hash=? AND expires_at>?", digest(token), time.Now().Unix()).Scan(&n) == nil && n == 1
}
func (s *Store) Revoke(ctx context.Context, token string) error {
	if len(token) != 43 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash=?", digest(token))
	return err
}
func digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func uuid() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
