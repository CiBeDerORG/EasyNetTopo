package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrDeviceNotFound              = errors.New("设备不存在")
	ErrDeviceGroupNotFound         = errors.New("设备分组不存在")
	ErrDeviceGroupExists           = errors.New("设备分组名称已存在")
	ErrDeviceNodeConflict          = errors.New("该网络节点已关联其他设备")
	ErrGatewayGroupNotFound        = errors.New("网关组不存在")
	ErrInvalidGatewayGroup         = errors.New("网关组名称或节点范围不合法")
	ErrGatewayDeviceConflict       = errors.New("该网关组已经绑定网关设备")
		ErrNetworkNodeAutoSyncDisabled = errors.New("网络节点自动补充已关闭")
	ErrInvalidDevice               = errors.New("设备信息不合法")
	ErrInvalidDeviceGroup          = errors.New("设备分组名称不合法")
	ErrDeviceNotOpenWrt            = errors.New("该设备未标记为 OpenWrt")
	ErrOpenWrtConfigNotFound       = errors.New("OpenWrt 发现服务尚未配置")
	ErrInvalidOpenWrtConfig        = errors.New("OpenWrt 发现服务配置不合法")
)

type DeviceGroup struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DeviceCount int    `json:"device_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type GatewayGroup struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	NetworkKey        string `json:"network_key"`
	PeerID            string `json:"peer_id"`
	DeviceCount       int    `json:"device_count"`
	GatewayDeviceID   string `json:"gateway_device_id,omitempty"`
	GatewayDeviceName string `json:"gateway_device_name,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type Device struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	GroupID             string   `json:"group_id,omitempty"`
	GroupName           string   `json:"group_name,omitempty"`
	GatewayGroupID      string   `json:"gateway_group_id,omitempty"`
	GatewayGroupName    string   `json:"gateway_group_name,omitempty"`
	OwnedGatewayGroupID string   `json:"owned_gateway_group_id,omitempty"`
	DeviceType          string   `json:"device_type"`
	SemanticKinds       []string `json:"semantic_kinds"`
	IsNodeDevice        bool     `json:"is_node_device"`
	IsGateway           bool     `json:"is_gateway"`
	Manufacturer        string   `json:"manufacturer,omitempty"`
	Model               string   `json:"model,omitempty"`
	IPAddress           string   `json:"ip_address,omitempty"`
	MACAddress          string   `json:"mac_address,omitempty"`
	Note                string   `json:"note,omitempty"`
	NetworkKey          string   `json:"network_key,omitempty"`
	PeerID              string   `json:"peer_id,omitempty"`
	NodeHostname        string   `json:"node_hostname,omitempty"`
	OpenWrtConfigured   bool     `json:"openwrt_configured"`
	OpenWrtRPCDPort     int      `json:"openwrt_rpcd_port,omitempty"`
	OpenWrtHTTPS        bool     `json:"openwrt_https,omitempty"`
	OpenWrtUsername     string   `json:"openwrt_username,omitempty"`
	DiscoveryStatus     string   `json:"discovery_status"`
	DiscoveryLastSeenAt string   `json:"discovery_last_seen_at,omitempty"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}
type DeviceInput struct {
	Name               string   `json:"name"`
	GroupID            string   `json:"group_id"`
	GatewayGroupID     string   `json:"gateway_group_id"`
	DeviceType         string   `json:"device_type"`
	SemanticKinds      []string `json:"semantic_kinds"`
	IsGateway          bool     `json:"is_gateway"`
	CreateGatewayGroup bool     `json:"create_gateway_group,omitempty"`
	GatewayGroupName   string   `json:"gateway_group_name,omitempty"`
	Manufacturer       string   `json:"manufacturer"`
	Model              string   `json:"model"`
	IPAddress          string   `json:"ip_address"`
	MACAddress         string   `json:"mac_address"`
	Note               string   `json:"note"`
	NetworkKey         string   `json:"network_key"`
	PeerID             string   `json:"peer_id"`
	NodeHostname       string   `json:"node_hostname,omitempty"`
}

type NetworkNodeDeviceInput struct {
	Name       string `json:"name"`
	NetworkKey string `json:"network_key"`
	PeerID     string `json:"peer_id"`
	Hostname   string `json:"hostname"` // EasyTier 原始主机名，不是显示名称或别名
}

type NetworkNodeSyncResult struct {
	Created  int  `json:"created"`
	Existing int  `json:"existing"`
	Rebound  int  `json:"rebound"`
	Skipped  int  `json:"skipped"`
	Enabled  bool `json:"enabled"`
}

type DeviceOpenWrtConfig struct {
	DeviceID   string `json:"device_id"`
	Configured bool   `json:"configured"`
	RPCDPort   int    `json:"rpcd_port"`
	HTTPS      bool   `json:"https"`
	Username   string `json:"username"`
}

type DeviceOpenWrtSecret struct {
	URL      string
	Username string
	Password string
}

type DeviceStore struct {
	store *Store
	key   []byte
}

func NewDeviceStore(ctx context.Context, db *Store) (*DeviceStore, error) {
	for _, query := range []string{
		`CREATE TABLE IF NOT EXISTS device_groups (
			id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE COLLATE NOCASE,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, group_id TEXT REFERENCES device_groups(id) ON DELETE SET NULL,
			device_type TEXT NOT NULL DEFAULT '通用设备', semantic_kinds TEXT NOT NULL DEFAULT '[]',
			is_node_device INTEGER NOT NULL DEFAULT 0, is_gateway INTEGER NOT NULL DEFAULT 0,
			manufacturer TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
			ip_address TEXT NOT NULL DEFAULT '', mac_address TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '', is_openwrt INTEGER NOT NULL DEFAULT 0,
			network_key TEXT NOT NULL DEFAULT '', peer_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			CHECK ((network_key='' AND peer_id='') OR (network_key<>'' AND peer_id<>''))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_group ON devices(group_id,name)`,
		`CREATE TABLE IF NOT EXISTS device_gateway_groups (
			id TEXT PRIMARY KEY, name TEXT NOT NULL,
			network_key TEXT NOT NULL, peer_id TEXT NOT NULL,
			gateway_device_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			CHECK (network_key<>'' AND peer_id<>'')
		)`,
		`CREATE TABLE IF NOT EXISTS network_node_sync_seen (
			network_key TEXT NOT NULL, peer_id TEXT NOT NULL, first_seen_at TEXT NOT NULL,
			PRIMARY KEY(network_key,peer_id)
		)`,
		`CREATE TABLE IF NOT EXISTS network_node_sync_deleted (
			network_key TEXT NOT NULL, peer_id TEXT NOT NULL,
			hostname TEXT NOT NULL DEFAULT '', deleted_at TEXT NOT NULL,
			PRIMARY KEY(network_key,peer_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_network_node_deleted_hostname
			ON network_node_sync_deleted(network_key,hostname COLLATE NOCASE)
			WHERE hostname<>''`,
		`CREATE TABLE IF NOT EXISTS device_openwrt_configs (
			device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
			rpcd_port INTEGER NOT NULL DEFAULT 80, https INTEGER NOT NULL DEFAULT 0,
			username TEXT NOT NULL, credential_sealed BLOB NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS device_discovery_observations (
			device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			gateway_device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			source TEXT NOT NULL CHECK(source IN ('devices','adb')),
			status TEXT NOT NULL CHECK(status IN ('online','offline')),
			last_seen_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(device_id,gateway_device_id,source)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_device_discovery_gateway_source
			ON device_discovery_observations(gateway_device_id,source,status)`,
	} {
		if _, err := db.db.ExecContext(ctx, query); err != nil {
			return nil, fmt.Errorf("初始化设备管理数据失败: %w", err)
		}
	}
	for _, column := range []struct{ name, definition string }{
		{"semantic_kinds", `semantic_kinds TEXT NOT NULL DEFAULT '[]'`},
		{"is_node_device", "is_node_device INTEGER NOT NULL DEFAULT 0"},
		{"node_hostname", "node_hostname TEXT NOT NULL DEFAULT ''"},
		{"is_gateway", "is_gateway INTEGER NOT NULL DEFAULT 0"},
		{"gateway_group_id", "gateway_group_id TEXT REFERENCES device_gateway_groups(id) ON DELETE SET NULL"},
	} {
		if err := ensureDeviceColumn(ctx, db, column.name, column.definition); err != nil {
			return nil, fmt.Errorf("迁移设备字段 %s 失败: %w", column.name, err)
		}
	}
	if err := ensureGatewayGroupColumn(ctx, db, "gateway_device_id", "gateway_device_id TEXT NOT NULL DEFAULT ''"); err != nil {
		return nil, fmt.Errorf("迁移网关设备归属字段失败: %w", err)
	}
	// 旧版把网关设备也放在自己的网关组中；迁移后网关设备独立作为组所有者。
	if _, err := db.db.ExecContext(ctx, "UPDATE device_gateway_groups SET gateway_device_id=(SELECT d.id FROM devices d WHERE d.gateway_group_id=device_gateway_groups.id AND d.is_gateway=1 ORDER BY d.created_at,d.id LIMIT 1) WHERE gateway_device_id=''"); err != nil {
		return nil, fmt.Errorf("迁移历史网关组所有者失败: %w", err)
	}
	if _, err := db.db.ExecContext(ctx, "UPDATE devices SET gateway_group_id=NULL WHERE is_gateway=1 AND gateway_group_id IN (SELECT id FROM device_gateway_groups WHERE gateway_device_id=devices.id)"); err != nil {
		return nil, fmt.Errorf("迁移历史网关设备归属失败: %w", err)
	}
	// 无网关所有者的旧组不再形成层级，其成员回到节点直属层。
	if _, err := db.db.ExecContext(ctx, "UPDATE devices SET gateway_group_id=NULL WHERE gateway_group_id IN (SELECT id FROM device_gateway_groups WHERE gateway_device_id='')"); err != nil {
		return nil, fmt.Errorf("迁移无所有者网关组成员失败: %w", err)
	}
	if _, err := db.db.ExecContext(ctx, "DELETE FROM device_gateway_groups WHERE gateway_device_id=''"); err != nil {
		return nil, fmt.Errorf("清理无所有者历史网关组失败: %w", err)
	}
	if _, err := db.db.ExecContext(ctx, "DROP INDEX IF EXISTS idx_devices_gateway_group_gateway"); err != nil {
		return nil, fmt.Errorf("迁移网关设备索引失败: %w", err)
	}
	if _, err := db.db.ExecContext(ctx, "DROP INDEX IF EXISTS idx_devices_network_node"); err != nil {
		return nil, fmt.Errorf("迁移节点设备唯一索引失败: %w", err)
	}
	for _, stmt := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_node_device_unique
			ON devices(network_key,peer_id) WHERE is_node_device=1 AND network_key<>'' AND peer_id<>''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_node_hostname_unique
			ON devices(network_key,node_hostname COLLATE NOCASE)
			WHERE is_node_device=1 AND network_key<>'' AND node_hostname<>''`,
		`CREATE INDEX IF NOT EXISTS idx_devices_node_scope
			ON devices(network_key,peer_id,is_node_device,gateway_group_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_device_gateway_groups_gateway_device
			ON device_gateway_groups(gateway_device_id) WHERE gateway_device_id<>''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_device_gateway_groups_scope_name
			ON device_gateway_groups(network_key,peer_id,name COLLATE NOCASE)`,
	} {
		if _, err := db.db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("初始化节点设备层级索引失败: %w", err)
		}
	}
	keyPath := filepath.Join(db.DataDir(), ".device-openwrt-sealing-key")
	if _, statErr := os.Lstat(keyPath); errors.Is(statErr, os.ErrNotExist) {
		var saved int
		if err := db.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_openwrt_configs").Scan(&saved); err != nil {
			return nil, err
		}
		if saved > 0 {
			return nil, errors.New("OpenWrt 凭据密钥缺失但数据库仍有已保存配置；拒绝生成新密钥，请从备份恢复密钥文件")
		}
	} else if statErr != nil {
		return nil, statErr
	}
	key, err := loadDeviceSealingKey(db.DataDir())
	if err != nil {
		return nil, err
	}
	return &DeviceStore{store: db, key: key}, nil
}

func ensureDeviceColumn(ctx context.Context, db *Store, name, definition string) error {
	rows, err := db.db.QueryContext(ctx, "PRAGMA table_info(devices)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notnull, primary int
		var columnName, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &columnType, &notnull, &defaultValue, &primary); err != nil {
			_ = rows.Close()
			return err
		}
		if columnName == name {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.db.ExecContext(ctx, "ALTER TABLE devices ADD COLUMN "+definition)
	return err
}

func ensureGatewayGroupColumn(ctx context.Context, db *Store, name, definition string) error {
	rows, err := db.db.QueryContext(ctx, "PRAGMA table_info(device_gateway_groups)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notnull, primary int
		var columnName, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &columnType, &notnull, &defaultValue, &primary); err != nil {
			_ = rows.Close()
			return err
		}
		if columnName == name {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.db.ExecContext(ctx, "ALTER TABLE device_gateway_groups ADD COLUMN "+definition)
	return err
}

func loadDeviceSealingKey(dir string) ([]byte, error) {
	path := filepath.Join(dir, ".device-openwrt-sealing-key")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if createErr != nil {
			return nil, createErr
		}
		key := make([]byte, 32)
		_, createErr = rand.Read(key)
		if createErr == nil {
			_, createErr = file.Write(key)
		}
		if syncErr := file.Sync(); createErr == nil {
			createErr = syncErr
		}
		if closeErr := file.Close(); createErr == nil {
			createErr = closeErr
		}
		if createErr != nil {
			_ = os.Remove(path)
			return nil, createErr
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("OpenWrt 凭据密钥文件权限或文件类型不安全")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("OpenWrt 凭据密钥内容损坏；禁止自动重置")
	}
	return key, nil
}

func (s *DeviceStore) seal(deviceID, secret string) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
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
	return aead.Seal(nonce, nonce, []byte(secret), []byte("openwrt:"+deviceID)), nil
}
func (s *DeviceStore) unseal(deviceID string, sealed []byte) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < aead.NonceSize() {
		return "", errors.New("OpenWrt 凭据损坏")
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, []byte("openwrt:"+deviceID))
	if err != nil {
		return "", errors.New("OpenWrt 凭据校验失败")
	}
	return string(plain), nil
}

func hasSemanticKind(kinds []string, target string) bool {
	for _, kind := range kinds {
		if kind == target {
			return true
		}
	}
	return false
}

func normalizeDevice(v *DeviceInput) error {
	v.Name = strings.TrimSpace(v.Name)
	v.GroupID = strings.TrimSpace(v.GroupID)
	v.GatewayGroupID = strings.TrimSpace(v.GatewayGroupID)
	v.GatewayGroupName = strings.TrimSpace(v.GatewayGroupName)
	v.DeviceType = strings.TrimSpace(v.DeviceType)
	v.Manufacturer = strings.TrimSpace(v.Manufacturer)
	v.Model = strings.TrimSpace(v.Model)
	v.IPAddress = strings.TrimSpace(v.IPAddress)
	v.MACAddress = strings.TrimSpace(v.MACAddress)
	v.Note = strings.TrimSpace(v.Note)
	v.NetworkKey = strings.TrimSpace(v.NetworkKey)
	v.PeerID = strings.TrimSpace(v.PeerID)
	v.NodeHostname = strings.TrimSpace(v.NodeHostname)
	if v.Name == "" || len([]rune(v.Name)) > 100 || strings.ContainsAny(v.Name, "\r\n\x00") {
		return ErrInvalidDevice
	}
	if v.DeviceType == "" {
		v.DeviceType = "通用设备"
	}
	if len([]rune(v.DeviceType)) > 100 || strings.ContainsAny(v.DeviceType, "\r\n\x00") {
		return ErrInvalidDevice
	}
	if v.CreateGatewayGroup {
		if v.GatewayGroupID != "" || v.NetworkKey == "" || v.PeerID == "" {
			return ErrInvalidDevice
		}
		if v.GatewayGroupName != "" {
			if _, err := normalizeGatewayGroup(v.GatewayGroupName, v.NetworkKey, v.PeerID); err != nil {
				return err
			}
		}
	} else {
		v.GatewayGroupName = ""
	}
	if v.IsGateway && (v.NetworkKey == "" || v.PeerID == "") {
		return ErrInvalidDevice
	}
	allowedKinds := map[string]bool{
		"router": true, "server": true, "openwrt": true, "phone": true, "computer": true,
		"tablet": true, "tv": true, "nas": true, "linux": true, "android": true, "other": true,
	}
	legacyKinds := map[string]string{
		"router": "router", "computer": "computer", "phone": "phone", "tablet": "tablet",
		"tv": "tv", "nas": "nas", "other": "other",
	}
	if kind := legacyKinds[v.DeviceType]; kind != "" {
		v.SemanticKinds = append(v.SemanticKinds, kind)
	}
	seenKinds := make(map[string]bool)
	kinds := make([]string, 0, len(v.SemanticKinds))
	for _, kind := range v.SemanticKinds {
		kind = strings.TrimSpace(kind)
		if kind == "" {
			continue
		}
		if !allowedKinds[kind] || len(kind) > 32 {
			return ErrInvalidDevice
		}
		if !seenKinds[kind] {
			seenKinds[kind] = true
			kinds = append(kinds, kind)
		}
	}
	if len(kinds) > 16 {
		return ErrInvalidDevice
	}
	v.SemanticKinds = kinds
	for _, field := range []struct {
		value string
		limit int
	}{
		{v.Manufacturer, 100}, {v.Model, 120}, {v.Note, 1000}, {v.NetworkKey, 256}, {v.PeerID, 128}, {v.NodeHostname, 100},
	} {
		if len([]rune(field.value)) > field.limit || strings.ContainsAny(field.value, "\x00") {
			return ErrInvalidDevice
		}
	}
	if (v.NetworkKey == "") != (v.PeerID == "") || (v.NetworkKey == "" && v.NodeHostname != "") || v.NodeHostname == "—" || strings.ContainsAny(v.NodeHostname, "\r\n") {
		return ErrInvalidDevice
	}
	if v.IPAddress != "" {
		ip, err := netip.ParseAddr(v.IPAddress)
		if err != nil || ip.Zone() != "" || !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return ErrInvalidDevice
		}
		v.IPAddress = ip.String()
	}
	if v.MACAddress != "" {
		mac, err := net.ParseMAC(v.MACAddress)
		if err != nil || len(mac) != 6 {
			return ErrInvalidDevice
		}
		v.MACAddress = strings.ToLower(mac.String())
	}
	if hasSemanticKind(v.SemanticKinds, "openwrt") {
		ip, err := netip.ParseAddr(v.IPAddress)
		if err != nil || !isPrivateIPv4(ip) {
			return ErrInvalidDevice
		}
	}
	return nil
}

func (s *DeviceStore) ListDeviceGroups(ctx context.Context) ([]DeviceGroup, error) {
	rows, err := s.store.db.QueryContext(ctx, `SELECT g.id,g.name,COUNT(d.id),g.created_at,g.updated_at
		FROM device_groups g LEFT JOIN devices d ON d.group_id=g.id
		GROUP BY g.id ORDER BY g.name COLLATE NOCASE,g.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]DeviceGroup, 0)
	for rows.Next() {
		var v DeviceGroup
		if err := rows.Scan(&v.ID, &v.Name, &v.DeviceCount, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *DeviceStore) CreateDeviceGroup(ctx context.Context, name string) (DeviceGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 || strings.ContainsAny(name, "\r\n\x00") {
		return DeviceGroup{}, ErrInvalidDeviceGroup
	}
	id, err := uuid()
	if err != nil {
		return DeviceGroup{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.store.db.ExecContext(ctx, "INSERT INTO device_groups(id,name,created_at,updated_at) VALUES(?,?,?,?)", id, name, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: device_groups.name") {
			return DeviceGroup{}, ErrDeviceGroupExists
		}
		return DeviceGroup{}, err
	}
	return DeviceGroup{ID: id, Name: name, CreatedAt: now, UpdatedAt: now}, nil
}
func (s *DeviceStore) UpdateDeviceGroup(ctx context.Context, id, name string) (DeviceGroup, error) {
	name = strings.TrimSpace(name)
	if id == "" || name == "" || len([]rune(name)) > 64 || strings.ContainsAny(name, "\r\n\x00") {
		return DeviceGroup{}, ErrInvalidDeviceGroup
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.store.db.ExecContext(ctx, "UPDATE device_groups SET name=?,updated_at=? WHERE id=?", name, now, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: device_groups.name") {
			return DeviceGroup{}, ErrDeviceGroupExists
		}
		return DeviceGroup{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return DeviceGroup{}, err
	}
	if n == 0 {
		return DeviceGroup{}, ErrDeviceGroupNotFound
	}
	var v DeviceGroup
	err = s.store.db.QueryRowContext(ctx, `SELECT g.id,g.name,COUNT(d.id),g.created_at,g.updated_at FROM device_groups g LEFT JOIN devices d ON d.group_id=g.id WHERE g.id=? GROUP BY g.id`, id).Scan(&v.ID, &v.Name, &v.DeviceCount, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
func (s *DeviceStore) DeleteDeviceGroup(ctx context.Context, id string) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_groups WHERE id=?", id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrDeviceGroupNotFound
	}
	if _, err = tx.ExecContext(ctx, "UPDATE devices SET group_id=NULL,updated_at=? WHERE group_id=?", time.Now().UTC().Format(time.RFC3339), id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM device_groups WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

const deviceSelect = `SELECT d.id,d.name,COALESCE(d.group_id,''),COALESCE(g.name,''),COALESCE(d.gateway_group_id,''),COALESCE(parent.name,gg.name,''),COALESCE(owned.id,''),d.device_type,d.semantic_kinds,d.is_node_device,d.is_gateway,d.manufacturer,d.model,d.ip_address,d.mac_address,d.note,d.is_openwrt,d.network_key,d.peer_id,d.node_hostname,
	CASE WHEN c.device_id IS NULL THEN 0 ELSE 1 END,COALESCE(c.rpcd_port,0),COALESCE(c.https,0),COALESCE(c.username,''),d.created_at,d.updated_at,
	CASE
		WHEN EXISTS(SELECT 1 FROM device_discovery_observations online_obs WHERE online_obs.device_id=d.id AND online_obs.status='online') THEN 'online'
		WHEN EXISTS(SELECT 1 FROM device_discovery_observations offline_obs WHERE offline_obs.device_id=d.id) THEN 'offline'
		ELSE 'unknown'
	END,
	COALESCE((SELECT MAX(obs.last_seen_at) FROM device_discovery_observations obs WHERE obs.device_id=d.id),'')
	FROM devices d
	LEFT JOIN device_groups g ON g.id=d.group_id
	LEFT JOIN device_gateway_groups gg ON gg.id=d.gateway_group_id
	LEFT JOIN devices parent ON parent.id=gg.gateway_device_id
	LEFT JOIN device_gateway_groups owned ON owned.gateway_device_id=d.id
	LEFT JOIN device_openwrt_configs c ON c.device_id=d.id`

func scanDevice(row interface{ Scan(...any) error }) (Device, error) {
	var v Device
	var configured, https bool
	var legacyIsOpenWrt bool
	var semanticKinds string
	err := row.Scan(&v.ID, &v.Name, &v.GroupID, &v.GroupName, &v.GatewayGroupID, &v.GatewayGroupName, &v.OwnedGatewayGroupID, &v.DeviceType, &semanticKinds, &v.IsNodeDevice, &v.IsGateway,
		&v.Manufacturer, &v.Model, &v.IPAddress, &v.MACAddress, &v.Note, &legacyIsOpenWrt, &v.NetworkKey, &v.PeerID, &v.NodeHostname,
		&configured, &v.OpenWrtRPCDPort, &https, &v.OpenWrtUsername, &v.CreatedAt, &v.UpdatedAt,
		&v.DiscoveryStatus, &v.DiscoveryLastSeenAt)
	if err != nil {
		return Device{}, err
	}
	if err := json.Unmarshal([]byte(semanticKinds), &v.SemanticKinds); err != nil {
		return Device{}, fmt.Errorf("设备语义标签数据损坏: %w", err)
	}
	if v.SemanticKinds == nil {
		v.SemanticKinds = []string{}
	}
	// 兼容历史记录：不改写数据库，只在读取结果中把旧标记映射为 OpenWrt 语义。
	if legacyIsOpenWrt && !hasSemanticKind(v.SemanticKinds, "openwrt") {
		v.SemanticKinds = append(v.SemanticKinds, "openwrt")
	}
	v.OpenWrtConfigured = configured
	v.OpenWrtHTTPS = https
	return v, nil
}

func (s *DeviceStore) listDevices(ctx context.Context, where string, args ...any) ([]Device, error) {
	query := deviceSelect + " " + where + " ORDER BY d.is_node_device DESC,COALESCE(gg.name,'') COLLATE NOCASE,d.name COLLATE NOCASE,d.id"
	rows, err := s.store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Device, 0)
	for rows.Next() {
		v, e := scanDevice(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (s *DeviceStore) ListDevices(ctx context.Context) ([]Device, error) {
	return s.listDevices(ctx, "")
}

func (s *DeviceStore) ListDevicesForNode(ctx context.Context, networkKey, peerID string) ([]Device, error) {
	if strings.TrimSpace(networkKey) == "" || strings.TrimSpace(peerID) == "" {
		return nil, ErrInvalidDevice
	}
	return s.listDevices(ctx, "WHERE d.network_key=? AND d.peer_id=?", networkKey, peerID)
}

func (s *DeviceStore) ListNodeDevices(ctx context.Context) ([]Device, error) {
	return s.listDevices(ctx, "WHERE d.is_node_device=1")
}

func (s *DeviceStore) ListUnassignedDevices(ctx context.Context) ([]Device, error) {
	return s.listDevices(ctx, "WHERE d.network_key='' AND d.peer_id=''")
}
func (s *DeviceStore) Device(ctx context.Context, id string) (Device, error) {
	v, err := scanDevice(s.store.db.QueryRowContext(ctx, deviceSelect+" WHERE d.id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrDeviceNotFound
	}
	return v, err
}
func validateGatewayGroupScopeTx(ctx context.Context, tx *sql.Tx, groupID, networkKey, peerID, deviceID string, allowUnowned bool) (string, error) {
	if groupID == "" {
		return "", nil
	}
	var groupNetworkKey, groupPeerID, gatewayDeviceID string
	err := tx.QueryRowContext(ctx, "SELECT network_key,peer_id,gateway_device_id FROM device_gateway_groups WHERE id=?", groupID).
		Scan(&groupNetworkKey, &groupPeerID, &gatewayDeviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrGatewayGroupNotFound
	}
	if err != nil {
		return "", err
	}
	if networkKey == "" || peerID == "" || groupNetworkKey != networkKey || groupPeerID != peerID {
		return "", ErrInvalidDevice
	}
	if gatewayDeviceID == "" {
		if allowUnowned {
			return "", nil
		}
		return "", ErrInvalidDevice
	}
	// 防止将设备挂到自身或自己的后代下面形成循环。
	seen := make(map[string]bool)
	for current := gatewayDeviceID; current != ""; {
		if current == deviceID || seen[current] {
			return "", ErrInvalidDevice
		}
		seen[current] = true
		var parentGroupID string
		err := tx.QueryRowContext(ctx, "SELECT COALESCE(gateway_group_id,'') FROM devices WHERE id=?", current).Scan(&parentGroupID)
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInvalidDevice
		}
		if err != nil {
			return "", err
		}
		if parentGroupID == "" {
			break
		}
		var ancestorID string
		err = tx.QueryRowContext(ctx, "SELECT gateway_device_id FROM device_gateway_groups WHERE id=?", parentGroupID).Scan(&ancestorID)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return "", err
		}
		if ancestorID == "" {
			break
		}
		current = ancestorID
	}
	return gatewayDeviceID, nil
}

func createOwnedGatewayGroupTx(ctx context.Context, tx *sql.Tx, deviceID, deviceName, requestedName, networkKey, peerID string) (string, error) {
	if deviceID == "" || networkKey == "" || peerID == "" {
		return "", ErrInvalidGatewayGroup
	}
	baseName := strings.TrimSpace(requestedName)
	if baseName == "" {
		baseName = strings.TrimSpace(deviceName) + " 网关组"
	}
	if strings.ContainsAny(baseName, "\r\n\x00") {
		return "", ErrInvalidGatewayGroup
	}
	runes := []rune(baseName)
	if len(runes) > 100 {
		baseName = string(runes[:100])
	}
	name := baseName
	for suffix := 2; ; suffix++ {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_gateway_groups WHERE network_key=? AND peer_id=? AND name=? COLLATE NOCASE",
			networkKey, peerID, name).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			break
		}
		suffixText := fmt.Sprintf(" (%d)", suffix)
		runes = []rune(baseName)
		limit := 100 - len([]rune(suffixText))
		if len(runes) > limit {
			runes = runes[:limit]
		}
		name = string(runes) + suffixText
	}
	id, err := uuid()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx, "INSERT INTO device_gateway_groups(id,name,network_key,peer_id,gateway_device_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?)",
		id, name, networkKey, peerID, deviceID, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return "", ErrGatewayDeviceConflict
		}
		return "", err
	}
	return id, nil
}

func moveGatewayDescendantsTx(ctx context.Context, tx *sql.Tx, groupID, networkKey, peerID, nodeHostname string, visited map[string]bool) error {
	if groupID == "" {
		return nil
	}
	if visited[groupID] {
		return ErrInvalidDevice
	}
	visited[groupID] = true
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, "UPDATE device_gateway_groups SET network_key=?,peer_id=?,updated_at=? WHERE id=?",
		networkKey, peerID, now, groupID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM devices WHERE gateway_group_id=? ORDER BY id", groupID)
	if err != nil {
		return err
	}
	var childIDs []string
	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			_ = rows.Close()
			return err
		}
		childIDs = append(childIDs, childID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, childID := range childIDs {
		if _, err := tx.ExecContext(ctx, "UPDATE devices SET network_key=?,peer_id=?,node_hostname=?,updated_at=? WHERE id=?",
			networkKey, peerID, nodeHostname, now, childID); err != nil {
			return err
		}
		var childGroupID string
		err := tx.QueryRowContext(ctx, "SELECT id FROM device_gateway_groups WHERE gateway_device_id=?", childID).Scan(&childGroupID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if err := moveGatewayDescendantsTx(ctx, tx, childGroupID, networkKey, peerID, nodeHostname, visited); err != nil {
			return err
		}
	}
	return nil
}

// 所属节点的原始主机名优先取已登记的基础节点/手工属性；新节点可由当前实时选择明确传入。
// 设备的可编辑名称不参与原主机名推断。
func resolveDeviceNodeHostnameTx(ctx context.Context, tx *sql.Tx, networkKey, peerID, supplied string) (string, error) {
	if networkKey == "" || peerID == "" {
		if supplied != "" {
			return "", ErrInvalidDevice
		}
		return "", nil
	}
	known := ""
	var stored string
	err := tx.QueryRowContext(ctx, `SELECT node_hostname FROM devices
		WHERE is_node_device=1 AND network_key=? AND peer_id=? LIMIT 1`, networkKey, peerID).Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil && stored != "" { known = stored }
	err = tx.QueryRowContext(ctx, `SELECT hostname FROM network_node_attributes
		WHERE network_key=? AND peer_id=? LIMIT 1`, networkKey, peerID).Scan(&stored)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil && stored != "" {
		if known != "" && !strings.EqualFold(known, stored) {
			return "", ErrInvalidDevice
		}
		known = stored
	}
	if supplied != "" && known != "" && !strings.EqualFold(known, supplied) {
		return "", ErrInvalidDevice
	}
	if known != "" {
		return known, nil
	}
	return supplied, nil
}

func (s *DeviceStore) CreateDevice(ctx context.Context, input DeviceInput) (Device, error) {
	if err := normalizeDevice(&input); err != nil {
		return Device{}, err
	}
	if input.CreateGatewayGroup {
		input.IsGateway = true
	}
	if input.IsGateway && (input.NetworkKey == "" || input.PeerID == "") {
		return Device{}, ErrInvalidDevice
	}
	id, err := uuid()
	if err != nil {
		return Device{}, err
	}
	semanticKinds, err := json.Marshal(input.SemanticKinds)
	if err != nil {
		return Device{}, err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, err
	}
	defer tx.Rollback()
	input.NodeHostname, err = resolveDeviceNodeHostnameTx(ctx, tx, input.NetworkKey, input.PeerID, input.NodeHostname)
	if err != nil {
		return Device{}, err
	}

	adoptGroupID := ""
	if input.GatewayGroupID != "" {
		ownerID, err := validateGatewayGroupScopeTx(ctx, tx, input.GatewayGroupID, input.NetworkKey, input.PeerID, "", input.IsGateway)
		if err != nil {
			return Device{}, err
		}
		if ownerID == "" {
			// 兼容旧流程：先建空网关组，再将首台网关设备绑定为组所有者。
			adoptGroupID = input.GatewayGroupID
			input.GatewayGroupID = ""
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx, `INSERT INTO devices(id,name,group_id,gateway_group_id,device_type,semantic_kinds,is_node_device,is_gateway,manufacturer,model,ip_address,mac_address,note,is_openwrt,network_key,peer_id,node_hostname,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, input.Name, nullIfEmpty(input.GroupID), nullIfEmpty(input.GatewayGroupID), input.DeviceType, string(semanticKinds),
		0, input.IsGateway, input.Manufacturer, input.Model, input.IPAddress, input.MACAddress,
		input.Note, hasSemanticKind(input.SemanticKinds, "openwrt"), input.NetworkKey, input.PeerID, input.NodeHostname, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "foreign key") {
			return Device{}, ErrDeviceGroupNotFound
		}
		return Device{}, err
	}
	if adoptGroupID != "" {
		result, err := tx.ExecContext(ctx, "UPDATE device_gateway_groups SET gateway_device_id=?,updated_at=? WHERE id=? AND gateway_device_id=''",
			id, now, adoptGroupID)
		if err != nil {
			return Device{}, err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return Device{}, ErrGatewayDeviceConflict
		}
	} else if input.IsGateway {
		if _, err := createOwnedGatewayGroupTx(ctx, tx, id, input.Name, input.GatewayGroupName, input.NetworkKey, input.PeerID); err != nil {
			return Device{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Device{}, err
	}
	return s.Device(ctx, id)
}

func (s *DeviceStore) UpdateDevice(ctx context.Context, id string, input DeviceInput) (Device, error) {
	if err := normalizeDevice(&input); err != nil {
		return Device{}, err
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Device{}, err
	}
	defer tx.Rollback()

	var isNodeDevice bool
	var currentNetworkKey, currentPeerID, currentOwnedGroupID string
	err = tx.QueryRowContext(ctx, `SELECT d.is_node_device,d.network_key,d.peer_id,COALESCE(owned.id,'')
		FROM devices d LEFT JOIN device_gateway_groups owned ON owned.gateway_device_id=d.id WHERE d.id=?`, id).
		Scan(&isNodeDevice, &currentNetworkKey, &currentPeerID, &currentOwnedGroupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Device{}, ErrDeviceNotFound
		}
		return Device{}, err
	}
	if isNodeDevice {
		// 允许基础节点设备转移到新的网络节点，仍须保持基础设备的直属层级。
		if input.NetworkKey == "" || input.PeerID == "" {
			return Device{}, ErrInvalidDevice
		}
		input.GatewayGroupID = ""
		if currentNetworkKey != input.NetworkKey || currentPeerID != input.PeerID {
			var conflicting int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices
				WHERE is_node_device=1 AND network_key=? AND peer_id=? AND id<>?`,
				input.NetworkKey, input.PeerID, id).Scan(&conflicting); err != nil {
					return Device{}, err
				}
			if conflicting > 0 {
				return Device{}, ErrDeviceNodeConflict
			}
		}
	}
	input.NodeHostname, err = resolveDeviceNodeHostnameTx(ctx, tx, input.NetworkKey, input.PeerID, input.NodeHostname)
	if err != nil {
		return Device{}, err
	}
	if isNodeDevice && input.NodeHostname != "" {
		var duplicate int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices
			WHERE is_node_device=1 AND network_key=? AND node_hostname=? COLLATE NOCASE AND id<>?`,
			input.NetworkKey, input.NodeHostname, id).Scan(&duplicate); err != nil {
				return Device{}, err
			}
		if duplicate > 0 {
			return Device{}, ErrDeviceNodeConflict
		}
	}
	if input.CreateGatewayGroup {
		input.IsGateway = true
	}
	if input.IsGateway && (input.NetworkKey == "" || input.PeerID == "") {
		return Device{}, ErrInvalidDevice
	}

	adoptGroupID := ""
	if input.GatewayGroupID != "" {
		ownerID, err := validateGatewayGroupScopeTx(ctx, tx, input.GatewayGroupID, input.NetworkKey, input.PeerID, id, input.IsGateway)
		if err != nil {
			return Device{}, err
		}
		if ownerID == "" {
			if currentOwnedGroupID != "" {
				return Device{}, ErrInvalidDevice
			}
			adoptGroupID = input.GatewayGroupID
			input.GatewayGroupID = ""
		}
	}
	semanticKinds, err := json.Marshal(input.SemanticKinds)
	if err != nil {
		return Device{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	if currentOwnedGroupID != "" && !input.IsGateway {
		if _, err := tx.ExecContext(ctx, "UPDATE devices SET gateway_group_id=?,network_key=?,peer_id=?,node_hostname=?,updated_at=? WHERE gateway_group_id=?",
			nullIfEmpty(input.GatewayGroupID), input.NetworkKey, input.PeerID, input.NodeHostname, now, currentOwnedGroupID); err != nil {
			return Device{}, err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM device_gateway_groups WHERE id=?", currentOwnedGroupID); err != nil {
			return Device{}, err
		}
		currentOwnedGroupID = ""
	}
	if input.IsGateway {
		switch {
		case adoptGroupID != "":
			result, err := tx.ExecContext(ctx, "UPDATE device_gateway_groups SET gateway_device_id=?,updated_at=? WHERE id=? AND gateway_device_id=''",
				id, now, adoptGroupID)
			if err != nil {
				return Device{}, err
			}
			if n, _ := result.RowsAffected(); n == 0 {
				return Device{}, ErrGatewayDeviceConflict
			}
			currentOwnedGroupID = adoptGroupID
		case currentOwnedGroupID == "":
			currentOwnedGroupID, err = createOwnedGatewayGroupTx(ctx, tx, id, input.Name, input.GatewayGroupName, input.NetworkKey, input.PeerID)
			if err != nil {
				return Device{}, err
			}
		case currentNetworkKey != input.NetworkKey || currentPeerID != input.PeerID:
			if err := moveGatewayDescendantsTx(ctx, tx, currentOwnedGroupID, input.NetworkKey, input.PeerID, input.NodeHostname, make(map[string]bool)); err != nil {
				return Device{}, err
			}
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE devices SET name=?,group_id=?,gateway_group_id=?,device_type=?,semantic_kinds=?,is_gateway=?,manufacturer=?,model=?,ip_address=?,mac_address=?,note=?,is_openwrt=?,network_key=?,peer_id=?,node_hostname=?,updated_at=? WHERE id=?`,
		input.Name, nullIfEmpty(input.GroupID), nullIfEmpty(input.GatewayGroupID), input.DeviceType, string(semanticKinds), input.IsGateway,
		input.Manufacturer, input.Model, input.IPAddress, input.MACAddress, input.Note, hasSemanticKind(input.SemanticKinds, "openwrt"), input.NetworkKey, input.PeerID, input.NodeHostname, now, id)
	if err != nil {
		return Device{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Device{}, err
	}
	if n == 0 {
		return Device{}, ErrDeviceNotFound
	}
	if !hasSemanticKind(input.SemanticKinds, "openwrt") {
		if _, err = tx.ExecContext(ctx, "DELETE FROM device_openwrt_configs WHERE device_id=?", id); err != nil {
			return Device{}, err
		}
	}
	if isNodeDevice {
		// 人工改绑到曾被删除的自动节点后，解除该位置的同步抑制。
		if _, err = tx.ExecContext(ctx, `DELETE FROM network_node_sync_deleted WHERE network_key=?
			AND (peer_id=? OR (hostname<>'' AND hostname=? COLLATE NOCASE))`,
			input.NetworkKey, input.PeerID, input.NodeHostname); err != nil {
				return Device{}, err
			}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO network_node_sync_seen(network_key,peer_id,first_seen_at)
			VALUES(?,?,?)`, input.NetworkKey, input.PeerID, now); err != nil {
				return Device{}, err
			}
	}
	if err = tx.Commit(); err != nil {
		return Device{}, err
	}
	return s.Device(ctx, id)
}

func (s *DeviceStore) ListGatewayGroups(ctx context.Context, networkKey, peerID string) ([]GatewayGroup, error) {
	if strings.TrimSpace(networkKey) == "" || strings.TrimSpace(peerID) == "" {
		return nil, ErrInvalidGatewayGroup
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT gg.id,gg.name,gg.network_key,gg.peer_id,COUNT(d.id),
		COALESCE(gg.gateway_device_id,''),COALESCE(owner.name,''),
		gg.created_at,gg.updated_at
		FROM device_gateway_groups gg
		LEFT JOIN devices d ON d.gateway_group_id=gg.id
		LEFT JOIN devices owner ON owner.id=gg.gateway_device_id
		WHERE gg.network_key=? AND gg.peer_id=? GROUP BY gg.id ORDER BY gg.name COLLATE NOCASE,gg.id`, networkKey, peerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]GatewayGroup, 0)
	for rows.Next() {
		var group GatewayGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.NetworkKey, &group.PeerID, &group.DeviceCount,
			&group.GatewayDeviceID, &group.GatewayDeviceName, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, group)
	}
	return result, rows.Err()
}

func normalizeGatewayGroup(name, networkKey, peerID string) (string, error) {
	name = strings.TrimSpace(name)
	networkKey = strings.TrimSpace(networkKey)
	peerID = strings.TrimSpace(peerID)
	if name == "" || len([]rune(name)) > 100 || strings.ContainsAny(name, "\r\n\x00") ||
		networkKey == "" || peerID == "" || len(networkKey) > 256 || len(peerID) > 128 {
		return "", ErrInvalidGatewayGroup
	}
	return name, nil
}

func createGatewayGroupTx(ctx context.Context, tx *sql.Tx, name, networkKey, peerID string) (string, error) {
	name, err := normalizeGatewayGroup(name, networkKey, peerID)
	if err != nil {
		return "", err
	}
	id, err := uuid()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx, "INSERT INTO device_gateway_groups(id,name,network_key,peer_id,created_at,updated_at) VALUES(?,?,?,?,?,?)",
		id, name, strings.TrimSpace(networkKey), strings.TrimSpace(peerID), now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return "", ErrInvalidGatewayGroup
		}
		return "", err
	}
	return id, nil
}

func (s *DeviceStore) CreateGatewayGroup(ctx context.Context, name, networkKey, peerID string) (GatewayGroup, error) {
	name, err := normalizeGatewayGroup(name, networkKey, peerID)
	if err != nil {
		return GatewayGroup{}, err
	}
	id, err := uuid()
	if err != nil {
		return GatewayGroup{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.store.db.ExecContext(ctx, "INSERT INTO device_gateway_groups(id,name,network_key,peer_id,created_at,updated_at) VALUES(?,?,?,?,?,?)",
		id, name, strings.TrimSpace(networkKey), strings.TrimSpace(peerID), now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return GatewayGroup{}, ErrInvalidGatewayGroup
		}
		return GatewayGroup{}, err
	}
	groups, err := s.ListGatewayGroups(ctx, networkKey, peerID)
	if err != nil {
		return GatewayGroup{}, err
	}
	for _, group := range groups {
		if group.ID == id {
			return group, nil
		}
	}
	return GatewayGroup{}, ErrGatewayGroupNotFound
}

func (s *DeviceStore) UpdateGatewayGroup(ctx context.Context, id, name string) (GatewayGroup, error) {
	name = strings.TrimSpace(name)
	if id == "" || name == "" || len([]rune(name)) > 100 || strings.ContainsAny(name, "\r\n\x00") {
		return GatewayGroup{}, ErrInvalidGatewayGroup
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.store.db.ExecContext(ctx, "UPDATE device_gateway_groups SET name=?,updated_at=? WHERE id=?", name, now, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return GatewayGroup{}, ErrInvalidGatewayGroup
		}
		return GatewayGroup{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return GatewayGroup{}, err
	}
	if n == 0 {
		return GatewayGroup{}, ErrGatewayGroupNotFound
	}
	var group GatewayGroup
	err = s.store.db.QueryRowContext(ctx, `SELECT gg.id,gg.name,gg.network_key,gg.peer_id,COUNT(d.id),
		COALESCE(gg.gateway_device_id,''),COALESCE(owner.name,''),
		gg.created_at,gg.updated_at FROM device_gateway_groups gg
		LEFT JOIN devices d ON d.gateway_group_id=gg.id
		LEFT JOIN devices owner ON owner.id=gg.gateway_device_id
		WHERE gg.id=? GROUP BY gg.id`, id).Scan(&group.ID, &group.Name, &group.NetworkKey, &group.PeerID, &group.DeviceCount,
		&group.GatewayDeviceID, &group.GatewayDeviceName, &group.CreatedAt, &group.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GatewayGroup{}, ErrGatewayGroupNotFound
	}
	return group, err
}

func (s *DeviceStore) DeleteGatewayGroup(ctx context.Context, id string) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var gatewayDeviceID, parentGroupID string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(gg.gateway_device_id,''),COALESCE(owner.gateway_group_id,'')
		FROM device_gateway_groups gg LEFT JOIN devices owner ON owner.id=gg.gateway_device_id WHERE gg.id=?`, id).
		Scan(&gatewayDeviceID, &parentGroupID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGatewayGroupNotFound
	}
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, "UPDATE devices SET gateway_group_id=?,updated_at=? WHERE gateway_group_id=?",
		nullIfEmpty(parentGroupID), now, id); err != nil {
		return err
	}
	if gatewayDeviceID != "" {
		if _, err := tx.ExecContext(ctx, "UPDATE devices SET is_gateway=0,updated_at=? WHERE id=?", now, gatewayDeviceID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM device_gateway_groups WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *DeviceStore) ClearDeviceData(ctx context.Context) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, statement := range []string{
		"DELETE FROM device_discovery_observations",
		"DELETE FROM device_openwrt_configs",
		"DELETE FROM devices",
		"DELETE FROM device_gateway_groups",
		"DELETE FROM device_groups",
		"DELETE FROM network_node_sync_seen",
		"DELETE FROM network_node_sync_deleted",
		"DELETE FROM settings WHERE key='device_network_node_auto_sync'",
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *DeviceStore) NetworkNodeSyncEnabled(ctx context.Context) (bool, error) {
	var value string
	err := s.store.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='device_network_node_auto_sync'").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return value == "1", err
}

func (s *DeviceStore) SyncNetworkNodeDevices(ctx context.Context, nodes []NetworkNodeDeviceInput) (NetworkNodeSyncResult, error) {
	return s.syncNetworkNodeDevices(ctx, nodes, true)
}

// SyncNetworkNodeDevicesIfEnabled 仅在自动补充仍启用时执行，避免清空数据后并发中的后台同步重新创建记录。
func (s *DeviceStore) SyncNetworkNodeDevicesIfEnabled(ctx context.Context, nodes []NetworkNodeDeviceInput) (NetworkNodeSyncResult, error) {
	return s.syncNetworkNodeDevices(ctx, nodes, false)
}

// 运行时 Peer ID 轮换时迁移节点作用域；主键 ID、网关所有者与 OpenWrt 凭据保持不变。
func rebindNetworkNodeTx(ctx context.Context, tx *sql.Tx, networkKey, oldPeerID, newPeerID, nodeDeviceID, hostname, now string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE devices SET peer_id=?,updated_at=?
		WHERE network_key=? AND peer_id=?`, newPeerID, now, networkKey, oldPeerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE devices SET node_hostname=?,updated_at=? WHERE id=?`,
		hostname, now, nodeDeviceID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE device_gateway_groups SET peer_id=?,updated_at=?
		WHERE network_key=? AND peer_id=?`, newPeerID, now, networkKey, oldPeerID); err != nil {
		return err
	}
	// 老版本的手工属性可能没有保存 hostname；换绑时补齐持久身份。
	// 若目标 Peer ID 已存在属性记录，保留两边原有数据，不冒险覆盖别名和坐标。
	if _, err := tx.ExecContext(ctx, `UPDATE network_node_attributes
		SET peer_id=?,hostname=CASE WHEN hostname='' THEN ? ELSE hostname END
		WHERE network_key=? AND peer_id=? AND NOT EXISTS (
			SELECT 1 FROM network_node_attributes WHERE network_key=? AND peer_id=?
		) AND NOT EXISTS (
			SELECT 1 FROM network_node_attributes WHERE network_key=? AND hostname=? COLLATE NOCASE AND peer_id<>?
		)`, newPeerID, hostname, networkKey, oldPeerID, networkKey, newPeerID,
		networkKey, hostname, oldPeerID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO network_node_sync_seen(network_key,peer_id,first_seen_at) VALUES(?,?,?)",
		networkKey, newPeerID, now)
	return err
}

// 历史版本未保存原始主机名，只允许唯一且名称完全一致的旧节点继承身份。
func uniqueLegacyNodeTx(ctx context.Context, tx *sql.Tx, networkKey, hostname, excludeID string) (string, string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,peer_id FROM devices
		WHERE is_node_device=1 AND network_key=? AND node_hostname='' AND name=? COLLATE NOCASE AND id<>?
		LIMIT 2`, networkKey, hostname, excludeID)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	var id, peerID string
	count := 0
	for rows.Next() {
		count++
		if err := rows.Scan(&id, &peerID); err != nil {
			return "", "", err
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	if count != 1 {
		return "", "", nil
	}
	return id, peerID, nil
}

// 仅清理系统自动生成且完全没有下挂设备、网关、人工属性和管理配置的重复节点壳。
func disposableDuplicateNodeTx(ctx context.Context, tx *sql.Tx, networkKey, peerID, nodeID, hostname string) (bool, error) {
	var name string
	var disposable bool
	err := tx.QueryRowContext(ctx, `SELECT name,
		is_node_device=1 AND node_hostname='' AND is_gateway=0 AND group_id IS NULL AND gateway_group_id IS NULL
		AND device_type='通用设备' AND semantic_kinds='[]'
		AND manufacturer='' AND model='' AND ip_address='' AND mac_address='' AND note='' AND is_openwrt=0
		AND NOT EXISTS(SELECT 1 FROM devices d WHERE d.network_key=? AND d.peer_id=? AND d.id<>?)
		AND NOT EXISTS(SELECT 1 FROM device_gateway_groups g WHERE g.network_key=? AND g.peer_id=?)
		AND NOT EXISTS(SELECT 1 FROM network_node_attributes a WHERE a.network_key=? AND a.peer_id=?)
		AND NOT EXISTS(SELECT 1 FROM device_openwrt_configs c WHERE c.device_id=?)
		AND NOT EXISTS(SELECT 1 FROM device_discovery_observations o WHERE o.device_id=? OR o.gateway_device_id=?)
		FROM devices WHERE id=?`, networkKey, peerID, nodeID, networkKey, peerID, networkKey, peerID,
		nodeID, nodeID, nodeID, nodeID).Scan(&name, &disposable)
	if err != nil {
		return false, err
	}
	isGeneratedName := strings.HasPrefix(strings.ToLower(name), strings.ToLower(hostname)+" · ") ||
		strings.HasPrefix(strings.ToLower(name), strings.ToLower(hostname)+" (")
	return disposable && isGeneratedName, nil
}

func (s *DeviceStore) syncNetworkNodeDevices(ctx context.Context, nodes []NetworkNodeDeviceInput, enableAutoSync bool) (NetworkNodeSyncResult, error) {
	if len(nodes) == 0 || len(nodes) > 500 {
		return NetworkNodeSyncResult{}, ErrInvalidDevice
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return NetworkNodeSyncResult{}, err
	}
	defer tx.Rollback()
	if enableAutoSync {
		if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES('device_network_node_auto_sync','1')
			ON CONFLICT(key) DO UPDATE SET value='1'`); err != nil {
			return NetworkNodeSyncResult{}, err
		}
	} else {
		var enabled string
		if err := tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='device_network_node_auto_sync'").Scan(&enabled); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return NetworkNodeSyncResult{}, ErrNetworkNodeAutoSyncDisabled
			}
			return NetworkNodeSyncResult{}, err
		}
		if enabled != "1" {
			return NetworkNodeSyncResult{}, ErrNetworkNodeAutoSyncDisabled
		}
	}
	result := NetworkNodeSyncResult{Enabled: true}
	now := time.Now().UTC().Format(time.RFC3339)
	// 同一网络中原主机名应唯一，若一次观测出现不同 Peer ID，不自动合并两个真实设备。
	incomingHosts := make(map[string]string, len(nodes))
	for _, node := range nodes {
		hostname := strings.TrimSpace(node.Hostname)
		if len([]rune(hostname)) > 100 || strings.ContainsAny(hostname, "\r\n\x00") || hostname == "—" {
			return NetworkNodeSyncResult{}, ErrInvalidDevice
		}
		if hostname == "" {
			continue // 兼容旧版客户端，但不会使用展示名称推断主机身份
		}
		key := strings.TrimSpace(node.NetworkKey) + "\x00" + strings.ToLower(hostname)
		if otherPeer, exists := incomingHosts[key]; exists && otherPeer != strings.TrimSpace(node.PeerID) {
			return NetworkNodeSyncResult{}, ErrInvalidDevice
		}
		incomingHosts[key] = strings.TrimSpace(node.PeerID)
	}
	for _, node := range nodes {
		input := DeviceInput{Name: node.Name, DeviceType: "通用设备", NetworkKey: node.NetworkKey, PeerID: node.PeerID}
		if err := normalizeDevice(&input); err != nil {
			return NetworkNodeSyncResult{}, err
		}
		if input.NetworkKey == "" || input.PeerID == "" {
			return NetworkNodeSyncResult{}, ErrInvalidDevice
		}
		hostname := strings.TrimSpace(node.Hostname)
		var deleted int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM network_node_sync_deleted
			WHERE network_key=? AND ((hostname<>'' AND hostname=? COLLATE NOCASE)
			OR (hostname='' AND peer_id=?))`, input.NetworkKey, hostname, input.PeerID).Scan(&deleted); err != nil {
			return NetworkNodeSyncResult{}, err
		}
		if deleted > 0 {
			result.Skipped++
			continue
		}
		if hostname != "" {
			// 历史节点可能只有别名/坐标，没有同步过基础设备；按本次实时 Peer ID 补全原始主机名。
			// 目标 hostname 已绑定其他 Peer ID 时，不覆盖或合并任何记录。
			if _, err = tx.ExecContext(ctx, `UPDATE network_node_attributes SET hostname=?
				WHERE network_key=? AND peer_id=? AND hostname='' AND NOT EXISTS (
					SELECT 1 FROM network_node_attributes WHERE network_key=? AND hostname=? COLLATE NOCASE AND peer_id<>?
				)`, hostname, input.NetworkKey, input.PeerID,
				input.NetworkKey, hostname, input.PeerID); err != nil {
				return NetworkNodeSyncResult{}, err
			}
		}
		var existingID, oldHostname string
		err := tx.QueryRowContext(ctx, "SELECT id,node_hostname FROM devices WHERE is_node_device=1 AND network_key=? AND peer_id=?", input.NetworkKey, input.PeerID).Scan(&existingID, &oldHostname)
		if err == nil {
			if hostname != "" && oldHostname == "" {
				// 老版本可能已经为变动后的 Peer ID 创建了新节点壳；若它完全未使用，保留原有历史节点。
				legacyID, legacyPeerID, lookupErr := uniqueLegacyNodeTx(ctx, tx, input.NetworkKey, hostname, existingID)
				if lookupErr != nil {
					return NetworkNodeSyncResult{}, lookupErr
				}
				if legacyID != "" {
					disposable, checkErr := disposableDuplicateNodeTx(ctx, tx, input.NetworkKey, input.PeerID, existingID, hostname)
					if checkErr != nil {
						return NetworkNodeSyncResult{}, checkErr
					}
					if disposable {
						if _, err := tx.ExecContext(ctx, "DELETE FROM devices WHERE id=?", existingID); err != nil {
							return NetworkNodeSyncResult{}, err
						}
						if err := rebindNetworkNodeTx(ctx, tx, input.NetworkKey, legacyPeerID, input.PeerID, legacyID, hostname, now); err != nil {
							return NetworkNodeSyncResult{}, err
						}
						result.Rebound++
						continue
					}
				}
			}
			// 以 Peer ID 确认同一节点时记录原始主机名，保留设备自定义名称与其他属性。
			if hostname == "" {
				hostname = oldHostname
			}
			if _, err = tx.ExecContext(ctx, "UPDATE devices SET node_hostname=?,updated_at=? WHERE id=?", hostname, now, existingID); err != nil {
				return NetworkNodeSyncResult{}, err
			}
			if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO network_node_sync_seen(network_key,peer_id,first_seen_at) VALUES(?,?,?)", input.NetworkKey, input.PeerID, now); err != nil {
				return NetworkNodeSyncResult{}, err
			}
			result.Existing++
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return NetworkNodeSyncResult{}, err
		}
		// 优先使用上次记录的原始 hostname；历史版本没有此字段时，仅允许名称完全匹配的唯一旧节点接管。
		var previousID, previousPeerID string
		if hostname != "" {
			err = tx.QueryRowContext(ctx, `SELECT id,peer_id FROM devices
				WHERE is_node_device=1 AND network_key=? AND node_hostname=? COLLATE NOCASE`,
				input.NetworkKey, hostname).Scan(&previousID, &previousPeerID)
			if errors.Is(err, sql.ErrNoRows) {
				rows, lookupErr := tx.QueryContext(ctx, `SELECT id,peer_id FROM devices
					WHERE is_node_device=1 AND network_key=? AND node_hostname='' AND name=? COLLATE NOCASE LIMIT 2`,
					input.NetworkKey, hostname)
				if lookupErr != nil {
					return NetworkNodeSyncResult{}, lookupErr
				}
				count := 0
				for rows.Next() {
					count++
					if scanErr := rows.Scan(&previousID, &previousPeerID); scanErr != nil {
						rows.Close()
						return NetworkNodeSyncResult{}, scanErr
					}
				}
				lookupErr = rows.Err()
				rows.Close()
				if lookupErr != nil {
					return NetworkNodeSyncResult{}, lookupErr
				}
				if count != 1 {
					previousID, previousPeerID = "", ""
				}
			} else if err != nil {
				return NetworkNodeSyncResult{}, err
			}
		}
		if previousID != "" {
			if err := rebindNetworkNodeTx(ctx, tx, input.NetworkKey, previousPeerID, input.PeerID, previousID, hostname, now); err != nil {
				return NetworkNodeSyncResult{}, err
			}
			result.Rebound++
			continue
		}
		var seen int
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM network_node_sync_seen WHERE network_key=? AND peer_id=?", input.NetworkKey, input.PeerID).Scan(&seen); err != nil {
			return NetworkNodeSyncResult{}, err
		}
		if seen > 0 {
			result.Skipped++
			continue
		}
		id, err := uuid()
		if err != nil {
			return NetworkNodeSyncResult{}, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO devices(id,name,group_id,gateway_group_id,device_type,semantic_kinds,is_node_device,is_gateway,manufacturer,model,ip_address,mac_address,note,is_openwrt,network_key,peer_id,node_hostname,created_at,updated_at)
			VALUES(?,?,NULL,NULL,'通用设备','[]',1,0,'','','','','',0,?,?,?,?,?)`, id, input.Name, input.NetworkKey, input.PeerID, hostname, now, now); err != nil {
			return NetworkNodeSyncResult{}, err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO network_node_sync_seen(network_key,peer_id,first_seen_at) VALUES(?,?,?)", input.NetworkKey, input.PeerID, now); err != nil {
			return NetworkNodeSyncResult{}, err
		}
		result.Created++
	}
	if err = tx.Commit(); err != nil {
		return NetworkNodeSyncResult{}, err
	}
	return result, nil
}

func (s *DeviceStore) DeleteDevice(ctx context.Context, id string) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var isNodeDevice bool
	var parentGroupID, ownedGroupID, networkKey, peerID, hostname string
	err = tx.QueryRowContext(ctx, `SELECT d.is_node_device,COALESCE(d.gateway_group_id,''),COALESCE(owned.id,''),
		d.network_key,d.peer_id,d.node_hostname
		FROM devices d LEFT JOIN device_gateway_groups owned ON owned.gateway_device_id=d.id WHERE d.id=?`, id).
		Scan(&isNodeDevice, &parentGroupID, &ownedGroupID, &networkKey, &peerID, &hostname)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if isNodeDevice {
		if hostname == "" {
			// 老节点记录如有同作用域属性，尽量保留实际原始主机名用于跨 Peer ID 的删除抑制。
			err = tx.QueryRowContext(ctx, `SELECT hostname FROM network_node_attributes
				WHERE network_key=? AND peer_id=?`, networkKey, peerID).Scan(&hostname)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM network_node_sync_deleted WHERE network_key=?
			AND (peer_id=? OR (hostname<>'' AND hostname=? COLLATE NOCASE))`,
			networkKey, peerID, hostname); err != nil {
				return err
			}
		if _, err = tx.ExecContext(ctx, `INSERT INTO network_node_sync_deleted(network_key,peer_id,hostname,deleted_at)
			VALUES(?,?,?,?)`, networkKey, peerID, hostname, now); err != nil {
				return err
			}
	}
	if ownedGroupID != "" {
		if _, err := tx.ExecContext(ctx, "UPDATE devices SET gateway_group_id=?,updated_at=? WHERE gateway_group_id=?",
			nullIfEmpty(parentGroupID), now, ownedGroupID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM device_gateway_groups WHERE id=?", ownedGroupID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM devices WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *DeviceStore) DeviceOpenWrtConfig(ctx context.Context, id string) (DeviceOpenWrtConfig, error) {
	device, err := s.Device(ctx, id)
	if err != nil {
		return DeviceOpenWrtConfig{}, err
	}
	if !hasSemanticKind(device.SemanticKinds, "openwrt") {
		return DeviceOpenWrtConfig{}, ErrDeviceNotOpenWrt
	}
	var c DeviceOpenWrtConfig
	err = s.store.db.QueryRowContext(ctx, "SELECT device_id,rpcd_port,https,username FROM device_openwrt_configs WHERE device_id=?", id).Scan(&c.DeviceID, &c.RPCDPort, &c.HTTPS, &c.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceOpenWrtConfig{DeviceID: id, Configured: false, RPCDPort: 80}, nil
	}
	c.Configured = true
	return c, err
}
func (s *DeviceStore) SaveDeviceOpenWrtConfig(ctx context.Context, id string, port int, useHTTPS bool, username, password string) (DeviceOpenWrtConfig, error) {
	device, err := s.Device(ctx, id)
	if err != nil {
		return DeviceOpenWrtConfig{}, err
	}
	if !hasSemanticKind(device.SemanticKinds, "openwrt") {
		return DeviceOpenWrtConfig{}, ErrDeviceNotOpenWrt
	}
	ip, parseErr := netip.ParseAddr(device.IPAddress)
	if parseErr != nil || !isPrivateIPv4(ip) {
		return DeviceOpenWrtConfig{}, ErrInvalidOpenWrtConfig
	}
	username = strings.TrimSpace(username)
	if port < 1 || port > 65535 || username == "" || len(username) > 64 || strings.ContainsAny(username, " \t\r\n\x00") || len(password) > 256 {
		return DeviceOpenWrtConfig{}, ErrInvalidOpenWrtConfig
	}
	var oldPort int
	var oldHTTPS bool
	var oldUsername string
	var oldSecret []byte
	oldErr := s.store.db.QueryRowContext(ctx, "SELECT rpcd_port,https,username,credential_sealed FROM device_openwrt_configs WHERE device_id=?", id).Scan(&oldPort, &oldHTTPS, &oldUsername, &oldSecret)
	if oldErr != nil && !errors.Is(oldErr, sql.ErrNoRows) {
		return DeviceOpenWrtConfig{}, oldErr
	}
	var sealed []byte
	if password == "" {
		if errors.Is(oldErr, sql.ErrNoRows) || oldPort != port || oldHTTPS != useHTTPS || oldUsername != username {
			return DeviceOpenWrtConfig{}, ErrInvalidOpenWrtConfig
		}
		sealed = oldSecret
	} else {
		sealed, err = s.seal(id, password)
		if err != nil {
			return DeviceOpenWrtConfig{}, err
		}
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO device_openwrt_configs(device_id,rpcd_port,https,username,credential_sealed,updated_at)
		VALUES(?,?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET rpcd_port=excluded.rpcd_port,https=excluded.https,username=excluded.username,credential_sealed=excluded.credential_sealed,updated_at=excluded.updated_at`,
		id, port, useHTTPS, username, sealed, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return DeviceOpenWrtConfig{}, err
	}
	return DeviceOpenWrtConfig{DeviceID: id, Configured: true, RPCDPort: port, HTTPS: useHTTPS, Username: username}, nil
}
func (s *DeviceStore) ClearDeviceOpenWrtConfig(ctx context.Context, id string) error {
	device, err := s.Device(ctx, id)
	if err != nil {
		return err
	}
	if !hasSemanticKind(device.SemanticKinds, "openwrt") {
		return ErrDeviceNotOpenWrt
	}
	_, err = s.store.db.ExecContext(ctx, "DELETE FROM device_openwrt_configs WHERE device_id=?", id)
	return err
}
func (s *DeviceStore) DeviceOpenWrtSecret(ctx context.Context, id string) (DeviceOpenWrtSecret, error) {
	device, err := s.Device(ctx, id)
	if err != nil {
		return DeviceOpenWrtSecret{}, err
	}
	if !hasSemanticKind(device.SemanticKinds, "openwrt") {
		return DeviceOpenWrtSecret{}, ErrDeviceNotOpenWrt
	}
	var port int
	var https bool
	var username string
	var sealed []byte
	err = s.store.db.QueryRowContext(ctx, "SELECT rpcd_port,https,username,credential_sealed FROM device_openwrt_configs WHERE device_id=?", id).Scan(&port, &https, &username, &sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceOpenWrtSecret{}, ErrOpenWrtConfigNotFound
	}
	if err != nil {
		return DeviceOpenWrtSecret{}, err
	}
	password, err := s.unseal(id, sealed)
	if err != nil {
		return DeviceOpenWrtSecret{}, err
	}
	scheme := "http"
	if https {
		scheme = "https"
	}
	return DeviceOpenWrtSecret{URL: fmt.Sprintf("%s://%s:%d/ubus", scheme, device.IPAddress, port), Username: username, Password: password}, nil
}

func isPrivateIPv4(ip netip.Addr) bool {
	if !ip.IsValid() || !ip.Is4() {
		return false
	}
	return netip.MustParsePrefix("10.0.0.0/8").Contains(ip) || netip.MustParsePrefix("172.16.0.0/12").Contains(ip) || netip.MustParsePrefix("192.168.0.0/16").Contains(ip)
}
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
