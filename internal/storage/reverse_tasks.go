package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ErrReverseTaskUnavailable = errors.New("反向安装任务不存在、已过期或无权访问")
var ErrReverseTaskConflict = errors.New("该主机已有相同应用的预安装任务")

type ReverseTaskPayload struct {
	Task      Task              `json:"task"`
	Revision  string            `json:"revision"`
	BundleSHA string            `json:"bundle_sha256"`
	Config    map[string]string `json:"config"`
}

func (s *TaskStore) sealConfig(taskID string, config map[string]string) ([]byte, error) {
	plain, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(s.sealKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plain, []byte("reverse-task|"+taskID)), nil
}

func (s *TaskStore) openConfig(taskID string, sealed []byte) (map[string]string, error) {
	block, err := aes.NewCipher(s.sealKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize() {
		return nil, errors.New("反向任务配置密文无效")
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte("reverse-task|"+taskID))
	if err != nil {
		return nil, errors.New("反向任务配置解密失败")
	}
	var result map[string]string
	if err := json.Unmarshal(plain, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]string{}
	}
	return result, nil
}

func (s *TaskStore) saveReverseBundle(digest string, bundle []byte) (string, error) {
	if len(digest) != 64 || len(bundle) == 0 || len(bundle) > 64<<20 {
		return "", errors.New("反向安装包无效或超出大小限制")
	}
	sum := sha256.Sum256(bundle)
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(digest)) != 1 {
		return "", errors.New("反向安装包 SHA-256 不匹配")
	}
	dir := filepath.Join(s.dataDir, "reverse-bundles")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("反向安装包目录权限不安全")
	}
	path := filepath.Join(dir, digest+".zip")
	if existing, err := os.Lstat(path); err == nil {
		if !existing.Mode().IsRegular() || existing.Mode().Perm()&0077 != 0 {
			return "", errors.New("反向安装包缓存文件类型或权限不安全")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		check := sha256.Sum256(data)
		if hex.EncodeToString(check[:]) != digest {
			return "", errors.New("已有反向安装包缓存校验失败")
		}
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".bundle-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(bundle); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return path, nil
}

// EnqueueReverseInstall 保存固定版本 Bundle 与加密参数，并创建现有任务中心可见的预安装任务。
func (s *TaskStore) EnqueueReverseInstall(ctx context.Context, hostID, sourceID, appID, revision, digest string, config map[string]string, bundle []byte) ([]Task, error) {
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	var connectionMode, hostType, expires, revoked string
	if err := s.db.QueryRowContext(ctx, `SELECT connection_mode,host_type,expires_at,revoked_at FROM hosts WHERE id=?`, hostID).
		Scan(&connectionMode, &hostType, &expires, &revoked); err != nil {
		return nil, ErrReverseTaskUnavailable
	}
	deadline, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || connectionMode != "reverse" || hostType != "temporary" || revoked != "" || !now.Before(deadline) {
		return nil, ErrReverseTaskUnavailable
	}
	var active int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_tasks
		WHERE host_id=? AND source_id=? AND app_id=? AND action='install'
		AND state IN ('queued','preparing','running')`, hostID, sourceID, appID).Scan(&active); err != nil {
		return nil, err
	}
	if active != 0 {
		return nil, ErrReverseTaskConflict
	}
	bundlePath, err := s.saveReverseBundle(digest, bundle)
	if err != nil {
		return nil, err
	}
	parentUUID, err := uuid()
	if err != nil {
		return nil, err
	}
	taskUUID, err := uuid()
	if err != nil {
		return nil, err
	}
	parentID, taskID := "HT-"+parentUUID, "AT-"+taskUUID
	sealed, err := s.sealConfig(taskID, config)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var stillActive int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hosts WHERE id=? AND connection_mode='reverse'
		AND host_type='temporary' AND revoked_at='' AND expires_at>?`, hostID, stamp).Scan(&stillActive); err != nil {
		return nil, err
	}
	if stillActive != 1 {
		return nil, ErrReverseTaskUnavailable
	}
	parent := Task{ID: parentID, Type: "host", HostID: hostID, Action: "install", TriggerSource: "temporary_preinstall", State: "queued", CreatedAt: stamp}
	task := Task{ID: taskID, ParentID: parentID, Type: "application", HostID: hostID, SourceID: sourceID, AppID: appID,
		Action: "install", TriggerSource: "temporary_preinstall", State: "queued", CreatedAt: stamp}
	insert := func(t Task) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO execution_tasks
			(id,parent_id,type,host_id,source_id,app_id,action,trigger_source,state,message,created_at,started_at,finished_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, t.ID, t.ParentID, t.Type, t.HostID, t.SourceID, t.AppID,
			t.Action, t.TriggerSource, t.State, t.Message, t.CreatedAt, t.StartedAt, t.FinishedAt)
		return err
	}
	if err := insert(parent); err != nil {
		return nil, err
	}
	if err := insert(task); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reverse_task_payloads(task_id,config_sealed,bundle_sha256,bundle_path,revision)
		VALUES(?,?,?,?,?)`, taskID, sealed, digest, bundlePath, revision); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return []Task{parent, task}, nil
}

// ClaimReverseTask 只向有效临时主机交付未 ACK 的任务；短租约允许网络丢包后安全重领。
func (s *TaskStore) ClaimReverseTask(ctx context.Context, agentID string) (*ReverseTaskPayload, error) {
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	lease := now.Add(45 * time.Second).Format(time.RFC3339Nano)
	row := s.db.QueryRowContext(ctx, `SELECT t.id,t.parent_id,t.type,t.host_id,t.source_id,t.app_id,t.action,t.trigger_source,
		t.state,t.message,t.created_at,t.started_at,t.finished_at,p.config_sealed,p.bundle_sha256,p.revision
		FROM execution_tasks t
		JOIN reverse_task_payloads p ON p.task_id=t.id
		JOIN hosts h ON h.id=t.host_id
		WHERE h.agent_id=? AND h.connection_mode='reverse' AND h.host_type='temporary' AND h.revoked_at=''
		AND ((t.state='queued' AND h.expires_at>? AND p.acknowledged_at=''
			AND (p.claimed_at='' OR p.lease_expires_at<=?))
			OR (t.state='running' AND p.acknowledged_at<>'' ))
		ORDER BY CASE WHEN t.state='running' THEN 0 ELSE 1 END,t.rowid LIMIT 1`, agentID, stamp, stamp)
	var task Task
	var sealed []byte
	var result ReverseTaskPayload
	err := row.Scan(&task.ID, &task.ParentID, &task.Type, &task.HostID, &task.SourceID, &task.AppID,
		&task.Action, &task.TriggerSource, &task.State, &task.Message, &task.CreatedAt, &task.StartedAt,
		&task.FinishedAt, &sealed, &result.BundleSHA, &result.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if task.State == "queued" {
		update, err := s.db.ExecContext(ctx, `UPDATE reverse_task_payloads SET claimed_at=?,lease_expires_at=?
			WHERE task_id=? AND acknowledged_at='' AND (claimed_at='' OR lease_expires_at<=?)`, stamp, lease, task.ID, stamp)
		if err != nil {
			return nil, err
		}
		affected, err := update.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected != 1 {
			return nil, nil
		}
	}
	config, err := s.openConfig(task.ID, sealed)
	if err != nil {
		return nil, err
	}
	result.Task, result.Config = task, config
	return &result, nil
}

func (s *TaskStore) ReverseTaskOwned(ctx context.Context, agentID, taskID string) (Task, bool, error) {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return Task{}, false, err
	}
	var count int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_tasks t JOIN hosts h ON h.id=t.host_id
		WHERE t.id=? AND h.agent_id=? AND h.connection_mode='reverse' AND h.host_type='temporary' AND h.revoked_at=''`,
		taskID, agentID).Scan(&count)
	return task, count == 1, err
}

func (s *TaskStore) AcknowledgeReverseTask(ctx context.Context, agentID, taskID string) (Task, error) {
	task, owned, err := s.ReverseTaskOwned(ctx, agentID, taskID)
	if err != nil || !owned {
		return Task{}, ErrReverseTaskUnavailable
	}
	now := time.Now().UTC()
	var expires string
	if err := s.db.QueryRowContext(ctx, `SELECT h.expires_at FROM hosts h WHERE h.agent_id=?`, agentID).Scan(&expires); err != nil {
		return Task{}, ErrReverseTaskUnavailable
	}
	deadline, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || !now.Before(deadline) {
		return Task{}, ErrReverseTaskUnavailable
	}
	res, err := s.db.ExecContext(ctx, `UPDATE reverse_task_payloads SET acknowledged_at=?,lease_expires_at=''
		WHERE task_id=? AND claimed_at<>'' AND acknowledged_at=''`, now.Format(time.RFC3339Nano), taskID)
	if err != nil {
		return Task{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Task{}, err
	}
	if task.State == "cancelled" || task.State == "failed" || task.State == "succeeded" || task.State == "interrupted" {
		return Task{}, ErrReverseTaskUnavailable
	}
	if n == 0 && task.State != "running" {
		var acknowledged string
		if err := s.db.QueryRowContext(ctx, `SELECT acknowledged_at FROM reverse_task_payloads WHERE task_id=?`, taskID).Scan(&acknowledged); err != nil || acknowledged == "" {
			return Task{}, ErrReverseTaskUnavailable
		}
	}
	if task.State == "queued" {
		if err := s.SetState(ctx, task, "running", "Agent 已领取预安装任务"); err != nil {
			return Task{}, err
		}
		task.State = "running"
		task.StartedAt = now.Format(time.RFC3339Nano)
	}
	return task, nil
}

func (s *TaskStore) UpdateReverseTaskState(ctx context.Context, agentID, taskID, state, message string) error {
	task, owned, err := s.ReverseTaskOwned(ctx, agentID, taskID)
	if err != nil || !owned {
		return ErrReverseTaskUnavailable
	}
	var acknowledged string
	if err := s.db.QueryRowContext(ctx, `SELECT acknowledged_at FROM reverse_task_payloads WHERE task_id=?`, taskID).Scan(&acknowledged); err != nil || acknowledged == "" {
		return ErrReverseTaskUnavailable
	}
	if task.State == state {
		return nil
	}
	if task.State != "running" || state == "running" {
		return ErrReverseTaskUnavailable
	}
	return s.SetState(ctx, task, state, message)
}

func (s *TaskStore) ReverseBundle(ctx context.Context, agentID, taskID string) ([]byte, string, error) {
	task, owned, err := s.ReverseTaskOwned(ctx, agentID, taskID)
	if err != nil || !owned {
		return nil, "", ErrReverseTaskUnavailable
	}
	var acked, digest, path string
	if err := s.db.QueryRowContext(ctx, `SELECT p.acknowledged_at,p.bundle_sha256,p.bundle_path
		FROM reverse_task_payloads p WHERE p.task_id=?`, taskID).Scan(&acked, &digest, &path); err != nil || acked == "" {
		return nil, "", ErrReverseTaskUnavailable
	}
	if task.State != "running" && task.State != "succeeded" {
		return nil, "", ErrReverseTaskUnavailable
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, "", ErrReverseTaskUnavailable
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != digest {
		return nil, "", fmt.Errorf("反向安装包缓存校验失败")
	}
	return data, digest, nil
}

func (s *TaskStore) CancelExpiredTemporaryTasks(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT parent_id FROM execution_tasks t JOIN hosts h ON h.id=t.host_id
		WHERE h.host_type='temporary' AND (h.revoked_at<>'' OR h.expires_at<=?)
		AND t.type='application' AND t.state='queued'`, now)
	if err != nil {
		return err
	}
	parents := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if id != "" {
			parents = append(parents, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := tx.ExecContext(ctx, `UPDATE execution_tasks SET state='cancelled',message='临时主机已过期或被剔除',finished_at=?
		WHERE type='application' AND state='queued' AND host_id IN
		(SELECT id FROM hosts WHERE host_type='temporary' AND (revoked_at<>'' OR expires_at<=?))`, now, now); err != nil {
		return err
	}
	for _, parent := range parents {
		if err := updateParentSummary(ctx, tx, parent, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *TaskStore) CancelRevokedTemporaryTasks(ctx context.Context, hostID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT parent_id FROM execution_tasks
		WHERE host_id=? AND type='application' AND state IN ('queued','running')`, hostID)
	if err != nil {
		return err
	}
	parents := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if id != "" {
			parents = append(parents, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := tx.ExecContext(ctx, `UPDATE execution_tasks SET state='cancelled',message='临时主机已被管理员剔除',finished_at=?
		WHERE host_id=? AND type='application' AND state='queued'`, now, hostID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE execution_tasks SET state='interrupted',message='临时主机已被管理员剔除；远端执行结果未知',finished_at=?
		WHERE host_id=? AND type='application' AND state='running'`, now, hostID); err != nil {
		return err
	}
	for _, parent := range parents {
		if err := updateParentSummary(ctx, tx, parent, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
