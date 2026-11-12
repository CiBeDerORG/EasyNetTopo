package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ReversePollSeconds = 10
const ReverseTTL = 30 * time.Minute

var (
	ErrReverseDisabled = errors.New("主机发现未开启")
	ErrReverseInactive = errors.New("临时主机不存在、已拒绝或已过期")
	ErrReverseConflict = errors.New("Agent 身份不匹配或已被其他主机绑定")
)

type ReverseAgent struct {
	AgentID    string `json:"agent_id"`
	SPKI       string `json:"spki_sha256"`
	Hostname   string `json:"hostname"`
	Platform   string `json:"platform"`
	Arch       string `json:"arch"`
	Version    string `json:"version"`
	Status     string `json:"status"`
	HostID     string `json:"host_id,omitempty"`
	AcceptedAt int64  `json:"accepted_at,omitempty"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
	LastSeenAt int64  `json:"last_seen_at"`
	NextPollAt int64  `json:"next_poll_at"`
}

type ReverseTask struct {
	TaskID       string            `json:"task_id"`
	ControllerID string            `json:"controller_id"`
	SourceID     string            `json:"source_id"`
	AppID        string            `json:"app_id"`
	Revision     string            `json:"revision"`
	BundleSHA256 string            `json:"bundle_sha256"`
	Config       map[string]string `json:"config"`
}

func (s *Store) InitReverse(ctx context.Context) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS reverse_agents(agent_id TEXT PRIMARY KEY,spki TEXT NOT NULL,hostname TEXT NOT NULL,platform TEXT NOT NULL,arch TEXT NOT NULL,version TEXT NOT NULL,status TEXT NOT NULL,host_id TEXT NOT NULL DEFAULT '',accepted_at INTEGER NOT NULL DEFAULT 0,expires_at INTEGER NOT NULL DEFAULT 0,last_seen_at INTEGER NOT NULL,next_poll_at INTEGER NOT NULL)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_reverse_host ON reverse_agents(host_id) WHERE host_id <> ''`,
		`CREATE TABLE IF NOT EXISTS reverse_jobs(task_id TEXT PRIMARY KEY,host_id TEXT NOT NULL,agent_id TEXT NOT NULL,source_id TEXT NOT NULL,app_id TEXT NOT NULL,revision TEXT NOT NULL,bundle_sha256 TEXT NOT NULL,config_sealed BLOB NOT NULL,bundle BLOB NOT NULL,claimed_at INTEGER NOT NULL DEFAULT 0)`,
		`CREATE INDEX IF NOT EXISTS idx_reverse_jobs_host ON reverse_jobs(host_id,claimed_at)`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ReverseDiscoveryUntil(ctx context.Context) (int64, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='reverse_discovery_until'").Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var until int64
	_, err = fmt.Sscan(raw, &until)
	return until, err
}

func (s *Store) SetReverseDiscovery(ctx context.Context, enabled bool) (int64, error) {
	var until int64
	if enabled {
		until = time.Now().UTC().Add(30 * time.Minute).Unix()
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES('reverse_discovery_until',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", fmt.Sprint(until))
	return until, err
}

func reverseScan(row interface{ Scan(...any) error }) (ReverseAgent, error) {
	var a ReverseAgent
	err := row.Scan(&a.AgentID, &a.SPKI, &a.Hostname, &a.Platform, &a.Arch, &a.Version, &a.Status,
		&a.HostID, &a.AcceptedAt, &a.ExpiresAt, &a.LastSeenAt, &a.NextPollAt)
	return a, err
}

const reverseColumns = "agent_id,spki,hostname,platform,arch,version,status,host_id,accepted_at,expires_at,last_seen_at,next_poll_at"

func (s *Store) ReverseByAgent(ctx context.Context, agentID string) (ReverseAgent, error) {
	return reverseScan(s.db.QueryRowContext(ctx, "SELECT "+reverseColumns+" FROM reverse_agents WHERE agent_id=?", agentID))
}

// 到期判定以服务端 accepted_at 为唯一基准。重连和心跳不允许续期。
func (s *Store) ExpireReverse(ctx context.Context) error {
	now := time.Now().UTC().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE reverse_agents SET status='expired' WHERE status='accepted' AND expires_at<=?", now); err != nil {
		return err
	}
	// 某些只使用主机 API 的部署/测试尚未初始化任务存储。
	// 不应让临时功能阻断现有主机列表。
	var hasTasks int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='execution_tasks'").Scan(&hasTasks); err != nil {
		return err
	}
	if hasTasks > 0 {
		if _, err = tx.ExecContext(ctx, `UPDATE execution_tasks SET state='cancelled',message='临时主机已到期或撤销',finished_at=? WHERE trigger_source='reverse_discovery' AND state='queued' AND host_id IN (SELECT host_id FROM reverse_agents WHERE status IN ('expired','revoked'))`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM hosts WHERE id IN (SELECT host_id FROM reverse_agents WHERE status IN ('expired','revoked') AND host_id<>'')"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReverseReport(ctx context.Context, a ReverseAgent) (ReverseAgent, error) {
	if len(a.Hostname) == 0 || len(a.Hostname) > 100 || strings.ContainsAny(a.Hostname, "\r\n\x00") ||
		(a.Platform != "linux" && a.Platform != "fnos" && a.Platform != "macos") ||
		(a.Arch != "amd64" && a.Arch != "arm64") || len(a.Version) > 128 {
		return ReverseAgent{}, errors.New("Agent 报到信息无效")
	}
	if err := s.ExpireReverse(ctx); err != nil {
		return ReverseAgent{}, err
	}
	now := time.Now().UTC().Unix()
	old, err := s.ReverseByAgent(ctx, a.AgentID)
	if err == nil {
		if old.SPKI != a.SPKI {
			return ReverseAgent{}, ErrReverseConflict
		}
		if old.Status == "revoked" || old.Status == "rejected" || old.Status == "expired" {
			return old, ErrReverseInactive
		}
		_, err = s.db.ExecContext(ctx, "UPDATE reverse_agents SET hostname=?,platform=?,arch=?,version=?,last_seen_at=?,next_poll_at=? WHERE agent_id=?",
			a.Hostname, a.Platform, a.Arch, a.Version, now, now+ReversePollSeconds, a.AgentID)
		if err != nil {
			return ReverseAgent{}, err
		}
		return s.ReverseByAgent(ctx, a.AgentID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ReverseAgent{}, err
	}
	until, err := s.ReverseDiscoveryUntil(ctx)
	if err != nil {
		return ReverseAgent{}, err
	}
	if until <= now {
		return ReverseAgent{}, ErrReverseDisabled
	}
	// 仅登记发现候选，未经管理员接受绝无管理或领取任务权限。
	_, err = s.db.ExecContext(ctx, `INSERT INTO reverse_agents(agent_id,spki,hostname,platform,arch,version,status,last_seen_at,next_poll_at) VALUES(?,?,?,?,?,?,'discovered',?,?)`,
		a.AgentID, a.SPKI, a.Hostname, a.Platform, a.Arch, a.Version, now, now+ReversePollSeconds)
	if err != nil {
		return ReverseAgent{}, err
	}
	return s.ReverseByAgent(ctx, a.AgentID)
}

func (s *Store) ListReverseCandidates(ctx context.Context) ([]ReverseAgent, error) {
	if err := s.ExpireReverse(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+reverseColumns+" FROM reverse_agents WHERE status='discovered' ORDER BY last_seen_at DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ReverseAgent, 0)
	for rows.Next() {
		a, e := reverseScan(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (s *Store) AcceptReverse(ctx context.Context, agentID string) (Host, error) {
	if err := s.ExpireReverse(ctx); err != nil {
		return Host{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Host{}, err
	}
	defer tx.Rollback()
	a, err := reverseScan(tx.QueryRowContext(ctx, "SELECT "+reverseColumns+" FROM reverse_agents WHERE agent_id=?", agentID))
	if err != nil {
		return Host{}, err
	}
	if a.Status != "discovered" {
		return Host{}, ErrReverseInactive
	}
	id, err := uuid()
	if err != nil {
		return Host{}, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, "INSERT INTO hosts(id,name,endpoint,agent_id,spki_sha256,paired_at) VALUES(?,?,?,?,?,?)",
		id, a.Hostname, "reverse://"+agentID, agentID, a.SPKI, now.Format(time.RFC3339Nano))
	if err != nil {
		return Host{}, fmt.Errorf("%w: %v", ErrReverseConflict, err)
	}
	result, err := tx.ExecContext(ctx, "UPDATE reverse_agents SET status='accepted',host_id=?,accepted_at=?,expires_at=?,next_poll_at=? WHERE agent_id=? AND status='discovered'",
		id, now.Unix(), now.Add(ReverseTTL).Unix(), now.Unix()+ReversePollSeconds, agentID)
	if err != nil {
		return Host{}, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return Host{}, ErrReverseConflict
	}
	if err := tx.Commit(); err != nil {
		return Host{}, err
	}
	return Host{ID: id, Name: a.Hostname, Endpoint: "reverse://" + agentID, AgentID: agentID, SPKISHA256: a.SPKI,
		PairedAt: now.Format(time.RFC3339Nano), HostType: "temporary", ConnectionMode: "reverse",
		AcceptedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(ReverseTTL).Format(time.RFC3339),
		Platform: a.Platform, Arch: a.Arch, AgentVersion: a.Version}, nil
}

func (s *Store) RejectReverse(ctx context.Context, agentID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE reverse_agents SET status='rejected' WHERE agent_id=? AND status='discovered'", agentID)
	return err
}

func (s *Store) RevokeReverse(ctx context.Context, hostID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, "UPDATE reverse_agents SET status='revoked' WHERE host_id=? AND status='accepted'", hostID)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrReverseInactive
	}
	if _, err = tx.ExecContext(ctx, "UPDATE execution_tasks SET state='cancelled',message='临时主机已主动剔除',finished_at=? WHERE host_id=? AND trigger_source='reverse_discovery' AND state='queued'",
		time.Now().UTC().Format(time.RFC3339Nano), hostID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM hosts WHERE id=?", hostID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DecorateReverseHosts(ctx context.Context, hosts []Host) ([]Host, error) {
	if err := s.ExpireReverse(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+reverseColumns+" FROM reverse_agents WHERE status='accepted'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]ReverseAgent)
	for rows.Next() {
		a, e := reverseScan(rows)
		if e != nil {
			return nil, e
		}
		m[a.HostID] = a
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range hosts {
		hosts[i].HostType = "permanent"
		hosts[i].ConnectionMode = "direct"
		if a, ok := m[hosts[i].ID]; ok {
			hosts[i].HostType = "temporary"
			hosts[i].ConnectionMode = "reverse"
			hosts[i].Platform = a.Platform
			hosts[i].Arch = a.Arch
			hosts[i].AgentVersion = a.Version
			hosts[i].AcceptedAt = time.Unix(a.AcceptedAt, 0).UTC().Format(time.RFC3339)
			hosts[i].ExpiresAt = time.Unix(a.ExpiresAt, 0).UTC().Format(time.RFC3339)
			hosts[i].LastSeenAt = time.Unix(a.LastSeenAt, 0).UTC().Format(time.RFC3339)
			hosts[i].NextPollAt = time.Unix(a.NextPollAt, 0).UTC().Format(time.RFC3339)
		}
	}
	return hosts, nil
}

// Seal 使用已有 Agent 凭据加密密钥，仅后端可解开待下发安装参数。
func (s *Store) reverseCipher() (cipher.AEAD, error) {
	key, err := os.ReadFile(filepath.Join(s.DataDir(), ".agent-sealing-key"))
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("控制端密钥无效")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Store) sealReverse(taskID string, cfg map[string]string) ([]byte, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	aead, err := s.reverseCipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, raw, []byte(taskID)), nil
}
func (s *Store) openReverse(taskID string, sealed []byte) (map[string]string, error) {
	aead, err := s.reverseCipher()
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize() {
		return nil, errors.New("配置密文损坏")
	}
	raw, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(taskID))
	if err != nil {
		return nil, err
	}
	var cfg map[string]string
	err = json.Unmarshal(raw, &cfg)
	return cfg, err
}

func (s *Store) AddReverseInstall(ctx context.Context, hostID, sourceID, appID, revision, sha string, config map[string]string, bundle []byte) (string, error) {
	if len(bundle) == 0 || len(bundle) > 64<<20 {
		return "", errors.New("应用包超出允许范围")
	}
	hash := sha256.Sum256(bundle)
	if hex.EncodeToString(hash[:]) != sha {
		return "", errors.New("应用包摘要不一致")
	}
	if err := s.ExpireReverse(ctx); err != nil {
		return "", err
	}
	var agentID string
	var expiry int64
	err := s.db.QueryRowContext(ctx, "SELECT agent_id,expires_at FROM reverse_agents WHERE host_id=? AND status='accepted'", hostID).Scan(&agentID, &expiry)
	if err != nil || expiry <= time.Now().Unix() {
		return "", ErrReverseInactive
	}
	id, err := uuid()
	if err != nil {
		return "", err
	}
	taskID := "AT-" + id
	sealed, err := s.sealReverse(taskID, config)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var pending int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM execution_tasks WHERE host_id=? AND trigger_source='reverse_discovery' AND state IN ('queued','preparing','running')", hostID).Scan(&pending)
	if err != nil {
		return "", err
	}
	if pending >= 5 {
		return "", errors.New("该临时主机待执行任务达到上限（5个）")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO execution_tasks(id,parent_id,type,host_id,source_id,app_id,action,trigger_source,state,message,created_at,started_at,finished_at) VALUES(?,'','application',?,?,?,'install','reverse_discovery','queued','',?,'','')`,
		taskID, hostID, sourceID, appID, now); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO reverse_jobs(task_id,host_id,agent_id,source_id,app_id,revision,bundle_sha256,config_sealed,bundle) VALUES(?,?,?,?,?,?,?,?,?)",
		taskID, hostID, agentID, sourceID, appID, revision, sha, sealed, bundle); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return taskID, nil
}

func (s *Store) ClaimReverse(ctx context.Context, agentID string) (*ReverseTask, error) {
	if err := s.ExpireReverse(ctx); err != nil {
		return nil, err
	}
	a, err := s.ReverseByAgent(ctx, agentID)
	if err != nil || a.Status != "accepted" || a.ExpiresAt <= time.Now().Unix() {
		return nil, ErrReverseInactive
	}
	// 正在执行的任务优先返回原 task_id，让 Agent 恢复而非重复安装。
	var job ReverseTask
	var sealed []byte
	err = s.db.QueryRowContext(ctx, `SELECT j.task_id,j.source_id,j.app_id,j.revision,j.bundle_sha256,j.config_sealed FROM reverse_jobs j JOIN execution_tasks t ON t.id=j.task_id WHERE j.agent_id=? AND j.host_id=? AND t.state IN ('preparing','running','queued') ORDER BY CASE t.state WHEN 'running' THEN 0 WHEN 'preparing' THEN 1 ELSE 2 END,t.created_at LIMIT 1`,
		agentID, a.HostID).Scan(&job.TaskID, &job.SourceID, &job.AppID, &job.Revision, &job.BundleSHA256, &sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.ControllerID = s.ControllerID()
	job.Config, err = s.openReverse(job.TaskID, sealed)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE execution_tasks SET state='preparing' WHERE id=? AND state='queued'", job.TaskID)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE reverse_jobs SET claimed_at=CASE WHEN claimed_at=0 THEN ? ELSE claimed_at END WHERE task_id=?", time.Now().Unix(), job.TaskID)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) ReverseAck(ctx context.Context, agentID, taskID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE execution_tasks SET state='running',started_at=? WHERE id=? AND state='preparing' AND EXISTS (SELECT 1 FROM reverse_jobs WHERE task_id=? AND agent_id=? AND claimed_at>0)`,
		time.Now().UTC().Format(time.RFC3339Nano), taskID, taskID, agentID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var state string
		if s.db.QueryRowContext(ctx, "SELECT t.state FROM execution_tasks t JOIN reverse_jobs j ON j.task_id=t.id WHERE t.id=? AND j.agent_id=?", taskID, agentID).Scan(&state) == nil && state == "running" {
			return nil
		}
		return ErrReverseInactive
	}
	return nil
}

func (s *Store) ReverseComplete(ctx context.Context, agentID, taskID, state, message string) error {
	if state != "succeeded" && state != "failed" && state != "interrupted" {
		return errors.New("不允许的任务结束状态")
	}
	if len(message) > 256 {
		message = message[:256]
	}
	res, err := s.db.ExecContext(ctx, `UPDATE execution_tasks SET state=?,message=?,finished_at=? WHERE id=? AND state IN ('preparing','running') AND EXISTS (SELECT 1 FROM reverse_jobs WHERE task_id=? AND agent_id=? AND claimed_at>0)`,
		state, message, time.Now().UTC().Format(time.RFC3339Nano), taskID, taskID, agentID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var existing string
		if s.db.QueryRowContext(ctx, "SELECT t.state FROM execution_tasks t JOIN reverse_jobs j ON j.task_id=t.id WHERE t.id=? AND j.agent_id=?", taskID, agentID).Scan(&existing) == nil && existing == state {
			return nil
		}
		return ErrReverseInactive
	}
	// 已完成的安装任务只保留必要的索引与审计记录，不长期保存大 ZIP 与明文可还原的密钥密文。
	_, err = s.db.ExecContext(ctx, "UPDATE reverse_jobs SET bundle=x'',config_sealed=x'' WHERE task_id=? AND agent_id=?", taskID, agentID)
	return err
}

func (s *Store) ReverseBundle(ctx context.Context, agentID, taskID string) (string, []byte, error) {
	var hash string
	var blob []byte
	err := s.db.QueryRowContext(ctx, `SELECT j.bundle_sha256,j.bundle FROM reverse_jobs j JOIN execution_tasks t ON t.id=j.task_id WHERE j.agent_id=? AND j.task_id=? AND j.claimed_at>0 AND t.state IN ('preparing','running')`,
		agentID, taskID).Scan(&hash, &blob)
	if err != nil {
		return "", nil, ErrReverseInactive
	}
	digest := sha256.Sum256(blob)
	if len(blob) == 0 || hex.EncodeToString(digest[:]) != hash {
		return "", nil, errors.New("安装包校验失败")
	}
	return hash, blob, nil
}

// TouchReverse 记录实际领取请求时间；此操作从不改变固定的 expires_at。
func (s *Store) TouchReverse(ctx context.Context, agentID string) error {
	now := time.Now().UTC().Unix()
	_, err := s.db.ExecContext(ctx, "UPDATE reverse_agents SET last_seen_at=?,next_poll_at=? WHERE agent_id=? AND status='accepted' AND expires_at>?",
		now, now+ReversePollSeconds, agentID, now)
	return err
}

 // IsReverseHost blocks legacy direct-only APIs from touching a temporary Agent.
 // Revoked/expired records remain recognizable to avoid accidental fallbacks.
 func (s *Store) IsReverseHost(ctx context.Context,hostID string)(bool,error){
    var count int
    err:=s.db.QueryRowContext(ctx,"SELECT COUNT(*) FROM reverse_agents WHERE host_id=?",hostID).Scan(&count)
    return count>0,err
 }
