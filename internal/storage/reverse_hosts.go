package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var ErrDiscoveryClosed = errors.New("主机发现未开启或已过期")
var ErrDiscoveryIdentity = errors.New("Agent 身份证书或指纹不匹配")
var ErrTemporaryHostExpired = errors.New("临时主机已过期或已撤销")

type DiscoveredAgent struct {
	AgentID     string `json:"agent_id"`
	Hostname    string `json:"hostname"`
	SPKISHA256  string `json:"spki_sha256"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	Version     string `json:"version"`
	FirstSeenAt string `json:"first_seen_at"`
	LastSeenAt  string `json:"last_seen_at"`
	Status      string `json:"status"`
	HostID      string `json:"host_id,omitempty"`
}

type DiscoveryState struct {
	Enabled   bool   `json:"enabled"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

func ensureReverseHostColumns(ctx context.Context, db *sql.DB) error {
	columns := []struct{ name, definition string }{
		{"connection_mode", "TEXT NOT NULL DEFAULT 'direct'"},
		{"host_type", "TEXT NOT NULL DEFAULT 'permanent'"},
		{"accepted_at", "TEXT NOT NULL DEFAULT ''"},
		{"expires_at", "TEXT NOT NULL DEFAULT ''"},
		{"last_seen_at", "TEXT NOT NULL DEFAULT ''"},
		{"next_poll_at", "TEXT NOT NULL DEFAULT ''"},
		{"platform", "TEXT NOT NULL DEFAULT ''"},
		{"arch", "TEXT NOT NULL DEFAULT ''"},
		{"agent_version", "TEXT NOT NULL DEFAULT ''"},
		{"revoked_at", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('hosts') WHERE name=?", column.name).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := db.ExecContext(ctx, "ALTER TABLE hosts ADD COLUMN "+column.name+" "+column.definition); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *HostStore) initReverseSchema(ctx context.Context) error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS agent_discovery_config (
			id INTEGER PRIMARY KEY CHECK(id=1),
			enabled INTEGER NOT NULL DEFAULT 0,
			token_hash TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS agent_discoveries (
			agent_id TEXT PRIMARY KEY,
			hostname TEXT NOT NULL,
			spki_sha256 TEXT NOT NULL,
			certificate_der BLOB NOT NULL,
			platform TEXT NOT NULL,
			arch TEXT NOT NULL,
			version TEXT NOT NULL,
			first_seen_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			host_id TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_discoveries_status ON agent_discoveries(status,last_seen_at)`,
	} {
		if _, err := s.store.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *HostStore) OpenDiscovery(ctx context.Context) (string, DiscoveryState, error) {
	token, err := randomToken()
	if err != nil {
		return "", DiscoveryState{}, err
	}
	expires := time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339Nano)
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO agent_discovery_config(id,enabled,token_hash,expires_at)
		VALUES(1,1,?,?) ON CONFLICT(id) DO UPDATE SET enabled=1,token_hash=excluded.token_hash,expires_at=excluded.expires_at`,
		digest(token), expires)
	return token, DiscoveryState{Enabled: err == nil, ExpiresAt: expires}, err
}

func (s *HostStore) CloseDiscovery(ctx context.Context) error {
	// 保留短时凭据至发现窗口自然过期，已接受但暂时离线的 Agent 仍可取回自己的专属 Token。
	_, err := s.store.db.ExecContext(ctx, `UPDATE agent_discovery_config SET enabled=0 WHERE id=1`)
	return err
}

func (s *HostStore) Discovery(ctx context.Context) (DiscoveryState, error) {
	var enabled int
	var expires string
	err := s.store.db.QueryRowContext(ctx, `SELECT enabled,expires_at FROM agent_discovery_config WHERE id=1`).Scan(&enabled, &expires)
	if err != nil {
		return DiscoveryState{}, err
	}
	deadline, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || !time.Now().UTC().Before(deadline) {
		_, _ = s.store.db.ExecContext(ctx, `UPDATE agent_discovery_config SET enabled=0,token_hash='' WHERE id=1`)
		return DiscoveryState{Enabled: false}, nil
	}
	return DiscoveryState{Enabled: enabled == 1, ExpiresAt: expires}, nil
}

func validAgentCertificate(agentID, fingerprint string, der []byte) error {
	if len(agentID) != 36 || len(fingerprint) != 64 || len(der) == 0 || len(der) > 16<<10 {
		return ErrDiscoveryIdentity
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil || cert.Subject.CommonName != "EasyHostAgent "+agentID ||
		cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 || cert.CheckSignatureFrom(cert) != nil {
		return ErrDiscoveryIdentity
	}
	spki, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return ErrDiscoveryIdentity
	}
	sum := sha256.Sum256(spki)
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(fingerprint)) != 1 {
		return ErrDiscoveryIdentity
	}
	return nil
}

func (s *HostStore) RegisterDiscovery(ctx context.Context, token string, item DiscoveredAgent, certificateDER []byte) (string, error) {
	if err := validAgentCertificate(item.AgentID, item.SPKISHA256, certificateDER); err != nil {
		return "", err
	}
	var enabled int
	var tokenHash, expires string
	if err := s.store.db.QueryRowContext(ctx, `SELECT enabled,token_hash,expires_at FROM agent_discovery_config WHERE id=1`).
		Scan(&enabled, &tokenHash, &expires); err != nil {
		return "", ErrDiscoveryClosed
	}
	deadline, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || !time.Now().UTC().Before(deadline) ||
		subtle.ConstantTimeCompare([]byte(tokenHash), []byte(digest(token))) != 1 {
		return "", ErrDiscoveryClosed
	}
	var existingPin, status string
	err = s.store.db.QueryRowContext(ctx, `SELECT spki_sha256,status FROM agent_discoveries WHERE agent_id=?`, item.AgentID).
		Scan(&existingPin, &status)
	if err == nil && existingPin != item.SPKISHA256 {
		return "", ErrHostBinding
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if enabled != 1 && status != "accepted" {
		return "", ErrDiscoveryClosed
	}
	if item.Hostname == "" || len(item.Hostname) > 255 || item.Platform == "" || len(item.Platform) > 32 ||
		item.Arch == "" || len(item.Arch) > 32 || len(item.Version) > 80 {
		return "", errors.New("Agent 主机信息无效")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	firstSeen := now
	if err == nil {
		_ = s.store.db.QueryRowContext(ctx, `SELECT first_seen_at FROM agent_discoveries WHERE agent_id=?`, item.AgentID).Scan(&firstSeen)
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO agent_discoveries
		(agent_id,hostname,spki_sha256,certificate_der,platform,arch,version,first_seen_at,last_seen_at,status)
		VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(agent_id) DO UPDATE SET hostname=excluded.hostname,certificate_der=excluded.certificate_der,
		platform=excluded.platform,arch=excluded.arch,version=excluded.version,last_seen_at=excluded.last_seen_at`,
		item.AgentID, item.Hostname, item.SPKISHA256, certificateDER, item.Platform, item.Arch, item.Version, firstSeen, now, "pending")
	if err != nil {
		return "", err
	}
	if status == "accepted" {
		return "accepted", nil
	}
	return "pending", nil
}

func (s *HostStore) ListDiscoveries(ctx context.Context) ([]DiscoveredAgent, error) {
	rows, err := s.store.db.QueryContext(ctx, `SELECT agent_id,hostname,spki_sha256,platform,arch,version,first_seen_at,last_seen_at,status,host_id
		FROM agent_discoveries WHERE status='pending' ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]DiscoveredAgent, 0)
	for rows.Next() {
		var item DiscoveredAgent
		if err := rows.Scan(&item.AgentID, &item.Hostname, &item.SPKISHA256, &item.Platform, &item.Arch,
			&item.Version, &item.FirstSeenAt, &item.LastSeenAt, &item.Status, &item.HostID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *HostStore) AcceptDiscovery(ctx context.Context, agentID string) (Host, string, error) {
	var item DiscoveredAgent
	err := s.store.db.QueryRowContext(ctx, `SELECT agent_id,hostname,spki_sha256,platform,arch,version,first_seen_at,last_seen_at,status,host_id
		FROM agent_discoveries WHERE agent_id=?`, agentID).Scan(&item.AgentID, &item.Hostname, &item.SPKISHA256,
		&item.Platform, &item.Arch, &item.Version, &item.FirstSeenAt, &item.LastSeenAt, &item.Status, &item.HostID)
	if err != nil {
		return Host{}, "", ErrHostNotFound
	}
	if item.Status == "accepted" && item.HostID != "" {
		h, err := s.Get(ctx, item.HostID)
		if err != nil {
			return Host{}, "", err
		}
		token, err := s.Credentials(ctx, h)
		return h, token, err
	}
	var count int
	if err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM hosts WHERE agent_id=?`, agentID).Scan(&count); err != nil {
		return Host{}, "", err
	}
	if count != 0 {
		return Host{}, "", ErrHostBinding
	}
	id, err := uuid()
	if err != nil {
		return Host{}, "", err
	}
	token, err := randomToken()
	if err != nil {
		return Host{}, "", err
	}
	now := time.Now().UTC()
	expires := now.Add(30 * time.Minute).Format(time.RFC3339Nano)
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO hosts
		(id,name,endpoint,connection_mode,host_type,accepted_at,expires_at,last_seen_at,next_poll_at,platform,arch,agent_version,revoked_at)
		VALUES(?,?, '', 'reverse','temporary',?,?,?,?,?,?,?,'')`,
		id, item.Hostname, now.Format(time.RFC3339Nano), expires, item.LastSeenAt,
		now.Add(10*time.Second).Format(time.RFC3339Nano), item.Platform, item.Arch, item.Version)
	if err != nil {
		return Host{}, "", err
	}
	h := Host{ID: id, Name: item.Hostname, AgentID: "", SPKISHA256: item.SPKISHA256}
	if err := s.SavePairing(ctx, h, item.AgentID, item.SPKISHA256, token); err != nil {
		_, _ = s.store.db.ExecContext(ctx, `DELETE FROM hosts WHERE id=?`, id)
		return Host{}, "", err
	}
	_, err = s.store.db.ExecContext(ctx, `UPDATE agent_discoveries SET status='accepted',host_id=? WHERE agent_id=?`, id, agentID)
	if err != nil {
		return Host{}, "", err
	}
	h, err = s.Get(ctx, id)
	return h, token, err
}

func (s *HostStore) GetByAgentID(ctx context.Context, agentID string) (Host, error) {
	var h Host
	err := s.store.db.QueryRowContext(ctx, `SELECT id,name,endpoint,agent_id,spki_sha256,paired_at,
		EXISTS(SELECT 1 FROM network_entries WHERE host_id=hosts.id),connection_mode,host_type,accepted_at,expires_at,
		last_seen_at,next_poll_at,platform,arch,agent_version,revoked_at FROM hosts WHERE agent_id=?`, agentID).
		Scan(&h.ID, &h.Name, &h.Endpoint, &h.AgentID, &h.SPKISHA256, &h.PairedAt, &h.NetworkEntry,
			&h.ConnectionMode, &h.HostType, &h.AcceptedAt, &h.ExpiresAt, &h.LastSeenAt, &h.NextPollAt,
			&h.Platform, &h.Arch, &h.AgentVersion, &h.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, ErrHostNotFound
	}
	return h, err
}

func (s *HostStore) UpdateAgentPoll(ctx context.Context, agentID string, next time.Duration) (Host, error) {
	now := time.Now().UTC()
	nextAt := now.Add(next)
	res, err := s.store.db.ExecContext(ctx, `UPDATE hosts SET last_seen_at=?,next_poll_at=?
		WHERE agent_id=? AND connection_mode='reverse' AND host_type='temporary' AND revoked_at='' AND expires_at>?`,
		now.Format(time.RFC3339Nano), nextAt.Format(time.RFC3339Nano), agentID, now.Format(time.RFC3339Nano))
	if err != nil {
		return Host{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Host{}, err
	}
	if n == 0 {
		return Host{}, ErrTemporaryHostExpired
	}
	return s.GetByAgentID(ctx, agentID)
}

func (s *HostStore) RevokeTemporary(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.store.db.ExecContext(ctx, `UPDATE hosts SET revoked_at=?,token_sealed=NULL
		WHERE id=? AND host_type='temporary' AND revoked_at=''`, now, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTemporaryHostExpired
	}
	return nil
}

func (s *HostStore) TemporaryActive(h Host) bool {
	if h.HostType != "temporary" || h.ConnectionMode != "reverse" || h.RevokedAt != "" {
		return false
	}
	expires, err := time.Parse(time.RFC3339Nano, h.ExpiresAt)
	return err == nil && time.Now().UTC().Before(expires)
}

func (s *HostStore) ReverseToken(ctx context.Context, agentID string) (string, error) {
	h, err := s.GetByAgentID(ctx, agentID)
	if err != nil {
		return "", err
	}
	if !s.TemporaryActive(h) {
		return "", ErrTemporaryHostExpired
	}
	return s.Credentials(ctx, h)
}

func (s *HostStore) String() string { return fmt.Sprintf("HostStore(%d)", len(s.key)) }
