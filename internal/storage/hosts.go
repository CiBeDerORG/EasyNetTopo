package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ErrHostNotFound = errors.New("主机不存在")
var ErrHostBinding = errors.New("主机已绑定其他 Agent 或身份指纹")

type Host struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	AgentID    string `json:"agent_id"`
	SPKISHA256 string `json:"spki_sha256"`
	PairedAt   string `json:"paired_at,omitempty"`
}
type HostStore struct {
	store *Store
	key   []byte
}

func NewHostStore(ctx context.Context, db *Store) (*HostStore, error) {
	for _, stmt := range []string{
		"CREATE TABLE IF NOT EXISTS hosts(id TEXT PRIMARY KEY,name TEXT NOT NULL,endpoint TEXT NOT NULL,agent_id TEXT NOT NULL DEFAULT '',spki_sha256 TEXT NOT NULL DEFAULT '',token_sealed BLOB,paired_at TEXT NOT NULL DEFAULT '')",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_binding ON hosts(agent_id) WHERE agent_id <> ''",
	} {
		if _, err := db.db.ExecContext(ctx, stmt); err != nil {
			return nil, err
		}
	}
	keyPath := filepath.Join(db.DataDir(), ".agent-sealing-key")
	info, err := os.Lstat(keyPath)
	if errors.Is(err, os.ErrNotExist) {
		fd, e := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		b := make([]byte, 32)
		_, e = rand.Read(b)
		if e == nil {
			_, e = fd.Write(b)
		}
		if syncErr := fd.Sync(); e == nil {
			e = syncErr
		}
		if closeErr := fd.Close(); e == nil {
			e = closeErr
		}
		if e != nil {
			_ = os.Remove(keyPath)
			return nil, e
		}
		info, err = os.Lstat(keyPath)
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("Agent 密钥文件权限或文件类型不安全")
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("Agent 密钥内容损坏；禁止自动重置")
	}
	return &HostStore{store: db, key: key}, nil
}
func (s *HostStore) Create(ctx context.Context, name, endpoint string) (Host, error) {
	id, err := uuid()
	if err != nil {
		return Host{}, err
	}
	_, err = s.store.db.ExecContext(ctx, "INSERT INTO hosts(id,name,endpoint) VALUES(?,?,?)", id, name, endpoint)
	return Host{ID: id, Name: name, Endpoint: endpoint}, err
}
func (s *HostStore) List(ctx context.Context) ([]Host, error) {
	rows, err := s.store.db.QueryContext(ctx, "SELECT id,name,endpoint,agent_id,spki_sha256,paired_at FROM hosts ORDER BY name,id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Host, 0)
	for rows.Next() {
		var h Host
		if err := rows.Scan(&h.ID, &h.Name, &h.Endpoint, &h.AgentID, &h.SPKISHA256, &h.PairedAt); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}
func (s *HostStore) Get(ctx context.Context, id string) (Host, error) {
	var h Host
	err := s.store.db.QueryRowContext(ctx, "SELECT id,name,endpoint,agent_id,spki_sha256,paired_at FROM hosts WHERE id=?", id).
		Scan(&h.ID, &h.Name, &h.Endpoint, &h.AgentID, &h.SPKISHA256, &h.PairedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, ErrHostNotFound
	}
	return h, err
}
// UpdateMetadata only changes the user-editable fields. It never rewrites
// the pinned identity, sealed token or original pairing timestamp.
func (s *HostStore) UpdateMetadata(ctx context.Context, id, name, endpoint string) (Host, error) {
	result, err := s.store.db.ExecContext(ctx,
		"UPDATE hosts SET name=?,endpoint=? WHERE id=?", name, endpoint, id)
	if err != nil {
		return Host{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Host{}, err
	}
	if count == 0 {
		return Host{}, ErrHostNotFound
	}
	return s.Get(ctx, id)
}

// Delete removes only this controller's local host registration and encrypted
// credentials. It never contacts or deletes the remote Agent or its resources.
func (s *HostStore) Delete(ctx context.Context, id string) error {
	result, err := s.store.db.ExecContext(ctx, "DELETE FROM hosts WHERE id=?", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrHostNotFound
	}
	return nil
}

func (s *HostStore) AssertAgentNotBoundElsewhere(ctx context.Context, hostID, agentID string) error {
	var count int
	err := s.store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM hosts WHERE agent_id=? AND id<>?", agentID, hostID).Scan(&count)
	if err != nil {
		return err
	}
	if count != 0 {
		return ErrHostBinding
	}
	return nil
}
func (s *HostStore) SavePairing(ctx context.Context, h Host, agentID, pin, token string) error {
	if len(token) != 64 {
		return errors.New("Agent Token 格式不符合契约")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	aad := []byte(h.ID + "|" + agentID + "|" + pin)
	ciphertext := aead.Seal(nonce, nonce, []byte(token), aad)
	res, err := s.store.db.ExecContext(ctx,
		"UPDATE hosts SET agent_id=?,spki_sha256=?,token_sealed=?,paired_at=? WHERE id=? AND (agent_id='' OR (agent_id=? AND spki_sha256=?))",
		agentID, pin, ciphertext, time.Now().UTC().Format(time.RFC3339), h.ID, agentID, pin)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrHostBinding
	}
	return nil
}
func (s *HostStore) Credentials(ctx context.Context, h Host) (string, error) {
	var data []byte
	if err := s.store.db.QueryRowContext(ctx, "SELECT token_sealed FROM hosts WHERE id=?", h.ID).Scan(&data); err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < aead.NonceSize() {
		return "", errors.New("主机未配对或凭据不可用")
	}
	nonce, ciphertext := data[:aead.NonceSize()], data[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(fmt.Sprintf("%s|%s|%s", h.ID, h.AgentID, h.SPKISHA256)))
	if err != nil {
		return "", errors.New("主机凭据校验失败")
	}
	return string(plaintext), nil
}
