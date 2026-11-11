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
	ErrNodeDeviceProtected         = errors.New("网络节点基础设备不能删除")
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
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	GroupID           string   `json:"group_id,omitempty"`
	GroupName         string   `json:"group_name,omitempty"`
	GatewayGroupID    string   `json:"gateway_group_id,omitempty"`
	GatewayGroupName  string   `json:"gateway_group_name,omitempty"`
	DeviceType        string   `json:"device_type"`
	SemanticKinds     []string `json:"semantic_kinds"`
	IsNodeDevice      bool     `json:"is_node_device"`
	IsGateway         bool     `json:"is_gateway"`
	Manufacturer      string   `json:"manufacturer,omitempty"`
	Model             string   `json:"model,omitempty"`
	IPAddress         string   `json:"ip_address,omitempty"`
	MACAddress        string   `json:"mac_address,omitempty"`
	Note              string   `json:"note,omitempty"`
	IsOpenWrt         bool     `json:"is_openwrt"`
	NetworkKey        string   `json:"network_key,omitempty"`
	PeerID            string   `json:"peer_id,omitempty"`
	OpenWrtConfigured bool     `json:"openwrt_configured"`
	OpenWrtRPCDPort   int      `json:"openwrt_rpcd_port,omitempty"`
	OpenWrtHTTPS      bool     `json:"openwrt_https,omitempty"`
	OpenWrtUsername   string   `json:"openwrt_username,omitempty"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

type DeviceInput struct {
	Name           string   `json:"name"`
	GroupID        string   `json:"group_id"`
	GatewayGroupID string   `json:"gateway_group_id"`
	DeviceType     string   `json:"device_type"`
	SemanticKinds  []string `json:"semantic_kinds"`
	IsGateway      bool     `json:"is_gateway"`
	Manufacturer   string   `json:"manufacturer"`
	Model          string   `json:"model"`
	IPAddress      string   `json:"ip_address"`
	MACAddress     string   `json:"mac_address"`
	Note           string   `json:"note"`
	IsOpenWrt      bool     `json:"is_openwrt"`
	NetworkKey     string   `json:"network_key"`
	PeerID         string   `json:"peer_id"`
}

type NetworkNodeDeviceInput struct {
	Name       string `json:"name"`
	NetworkKey string `json:"network_key"`
	PeerID     string `json:"peer_id"`
}

type NetworkNodeSyncResult struct {
	Created  int  `json:"created"`
	Existing int  `json:"existing"`
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
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_network_node
			ON devices(network_key,peer_id) WHERE network_key<>'' AND peer_id<>''`,
		`CREATE INDEX IF NOT EXISTS idx_devices_group ON devices(group_id,name)`,
		`CREATE TABLE IF NOT EXISTS device_gateway_groups (
			id TEXT PRIMARY KEY, name TEXT NOT NULL,
			network_key TEXT NOT NULL, peer_id TEXT NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			CHECK (network_key<>'' AND peer_id<>'')
		)`,
		`CREATE TABLE IF NOT EXISTS network_node_sync_seen (
			network_key TEXT NOT NULL, peer_id TEXT NOT NULL, first_seen_at TEXT NOT NULL,
			PRIMARY KEY(network_key,peer_id)
		)`,
		`CREATE TABLE IF NOT EXISTS device_openwrt_configs (
			device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
			rpcd_port INTEGER NOT NULL DEFAULT 80, https INTEGER NOT NULL DEFAULT 0,
			username TEXT NOT NULL, credential_sealed BLOB NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	} {
		if _, err := db.db.ExecContext(ctx, query); err != nil {
			return nil, fmt.Errorf("初始化设备管理数据失败: %w", err)
		}
	}
	for _, column := range []struct{ name, definition string }{
		{"semantic_kinds", `semantic_kinds TEXT NOT NULL DEFAULT '[]'`},
		{"is_node_device", "is_node_device INTEGER NOT NULL DEFAULT 0"},
		{"is_gateway", "is_gateway INTEGER NOT NULL DEFAULT 0"},
		{"gateway_group_id", "gateway_group_id TEXT REFERENCES device_gateway_groups(id) ON DELETE SET NULL"},
	} {
		if err := ensureDeviceColumn(ctx, db, column.name, column.definition); err != nil {
			return nil, fmt.Errorf("迁移设备字段 %s 失败: %w", column.name, err)
		}
	}
	// 启动时只补齐数据库结构，不自动转换或重写历史设备记录。
	if _, err := db.db.ExecContext(ctx, "DROP INDEX IF EXISTS idx_devices_network_node"); err != nil {
		return nil, fmt.Errorf("迁移节点设备唯一索引失败: %w", err)
	}
	for _, stmt := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_node_device_unique
			ON devices(network_key,peer_id) WHERE is_node_device=1 AND network_key<>'' AND peer_id<>''`,
		`CREATE INDEX IF NOT EXISTS idx_devices_node_scope
			ON devices(network_key,peer_id,is_node_device,gateway_group_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_gateway_group_gateway
			ON devices(gateway_group_id) WHERE gateway_group_id IS NOT NULL AND gateway_group_id<>'' AND is_gateway=1`,
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

func normalizeDevice(v *DeviceInput) error {
	v.Name = strings.TrimSpace(v.Name)
	v.GroupID = strings.TrimSpace(v.GroupID)
	v.GatewayGroupID = strings.TrimSpace(v.GatewayGroupID)
	v.DeviceType = strings.TrimSpace(v.DeviceType)
	v.Manufacturer = strings.TrimSpace(v.Manufacturer)
	v.Model = strings.TrimSpace(v.Model)
	v.IPAddress = strings.TrimSpace(v.IPAddress)
	v.MACAddress = strings.TrimSpace(v.MACAddress)
	v.Note = strings.TrimSpace(v.Note)
	v.NetworkKey = strings.TrimSpace(v.NetworkKey)
	v.PeerID = strings.TrimSpace(v.PeerID)
	if v.Name == "" || len([]rune(v.Name)) > 100 || strings.ContainsAny(v.Name, "\r\n\x00") {
		return ErrInvalidDevice
	}
	if v.DeviceType == "" {
		v.DeviceType = "通用设备"
	}
	if len([]rune(v.DeviceType)) > 100 || strings.ContainsAny(v.DeviceType, "\r\n\x00") {
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
	if v.IsOpenWrt {
		hasRouterKind := false
		for _, kind := range v.SemanticKinds {
			if strings.TrimSpace(kind) == "router" {
				hasRouterKind = true
				break
			}
		}
		if !hasRouterKind {
			return ErrInvalidDevice
		}
		v.SemanticKinds = append(v.SemanticKinds, "openwrt")
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
	if v.IsOpenWrt && !seenKinds["router"] {
		return ErrInvalidDevice
	}
	for _, field := range []struct {
		value string
		limit int
	}{
		{v.Manufacturer, 100}, {v.Model, 120}, {v.Note, 1000}, {v.NetworkKey, 256}, {v.PeerID, 128},
	} {
		if len([]rune(field.value)) > field.limit || strings.ContainsAny(field.value, "\x00") {
			return ErrInvalidDevice
		}
	}
	if (v.NetworkKey == "") != (v.PeerID == "") {
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
	if v.IsOpenWrt {
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

const deviceSelect = `SELECT d.id,d.name,COALESCE(d.group_id,''),COALESCE(g.name,''),COALESCE(d.gateway_group_id,''),COALESCE(gg.name,''),d.device_type,d.semantic_kinds,d.is_node_device,d.is_gateway,d.manufacturer,d.model,d.ip_address,d.mac_address,d.note,d.is_openwrt,d.network_key,d.peer_id,
	CASE WHEN c.device_id IS NULL THEN 0 ELSE 1 END,COALESCE(c.rpcd_port,0),COALESCE(c.https,0),COALESCE(c.username,''),d.created_at,d.updated_at
	FROM devices d LEFT JOIN device_groups g ON g.id=d.group_id LEFT JOIN device_gateway_groups gg ON gg.id=d.gateway_group_id LEFT JOIN device_openwrt_configs c ON c.device_id=d.id`

func scanDevice(row interface{ Scan(...any) error }) (Device, error) {
	var v Device
	var configured, https bool
	var semanticKinds string
	err := row.Scan(&v.ID, &v.Name, &v.GroupID, &v.GroupName, &v.GatewayGroupID, &v.GatewayGroupName, &v.DeviceType, &semanticKinds, &v.IsNodeDevice, &v.IsGateway,
		&v.Manufacturer, &v.Model, &v.IPAddress, &v.MACAddress, &v.Note, &v.IsOpenWrt, &v.NetworkKey, &v.PeerID,
		&configured, &v.OpenWrtRPCDPort, &https, &v.OpenWrtUsername, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return Device{}, err
	}
	if err := json.Unmarshal([]byte(semanticKinds), &v.SemanticKinds); err != nil {
		return Device{}, fmt.Errorf("设备语义标签数据损坏: %w", err)
	}
	if v.SemanticKinds == nil {
		v.SemanticKinds = []string{}
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
func (s *DeviceStore) validateGatewayGroupScope(ctx context.Context, groupID, networkKey, peerID string) error {
	if groupID == "" {
		return nil
	}
	var groupNetworkKey, groupPeerID string
	err := s.store.db.QueryRowContext(ctx, "SELECT network_key,peer_id FROM device_gateway_groups WHERE id=?", groupID).Scan(&groupNetworkKey, &groupPeerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrGatewayGroupNotFound
	}
	if err != nil {
		return err
	}
	if networkKey == "" || peerID == "" || groupNetworkKey != networkKey || groupPeerID != peerID {
		return ErrInvalidDevice
	}
	return nil
}

func (s *DeviceStore) CreateDevice(ctx context.Context, input DeviceInput) (Device, error) {
	if err := normalizeDevice(&input); err != nil {
		return Device{}, err
	}
	if input.IsGateway && input.GatewayGroupID == "" {
		return Device{}, ErrInvalidDevice
	}
	if err := s.validateGatewayGroupScope(ctx, input.GatewayGroupID, input.NetworkKey, input.PeerID); err != nil {
		return Device{}, err
	}
	id, err := uuid()
	if err != nil {
		return Device{}, err
	}
	semanticKinds, err := json.Marshal(input.SemanticKinds)
	if err != nil {
		return Device{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO devices(id,name,group_id,gateway_group_id,device_type,semantic_kinds,is_node_device,is_gateway,manufacturer,model,ip_address,mac_address,note,is_openwrt,network_key,peer_id,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, input.Name, nullIfEmpty(input.GroupID), nullIfEmpty(input.GatewayGroupID), input.DeviceType, string(semanticKinds),
		0, input.IsGateway, input.Manufacturer, input.Model, input.IPAddress, input.MACAddress,
		input.Note, input.IsOpenWrt, input.NetworkKey, input.PeerID, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "foreign key") {
			return Device{}, ErrDeviceGroupNotFound
		}
		if strings.Contains(strings.ToLower(err.Error()), "idx_devices_gateway_group_gateway") ||
			strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: devices.gateway_group_id") {
			return Device{}, ErrGatewayDeviceConflict
		}
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
	var currentNetworkKey, currentPeerID string
	if err = tx.QueryRowContext(ctx, "SELECT is_node_device,network_key,peer_id FROM devices WHERE id=?", id).Scan(&isNodeDevice, &currentNetworkKey, &currentPeerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Device{}, ErrDeviceNotFound
		}
		return Device{}, err
	}
	if isNodeDevice {
		input.NetworkKey, input.PeerID = currentNetworkKey, currentPeerID
		input.GatewayGroupID = ""
		input.IsGateway = false
	}
	if input.IsGateway && input.GatewayGroupID == "" {
		return Device{}, ErrInvalidDevice
	}
	if err = s.validateGatewayGroupScope(ctx, input.GatewayGroupID, input.NetworkKey, input.PeerID); err != nil {
		return Device{}, err
	}
	semanticKinds, err := json.Marshal(input.SemanticKinds)
	if err != nil {
		return Device{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.ExecContext(ctx, `UPDATE devices SET name=?,group_id=?,gateway_group_id=?,device_type=?,semantic_kinds=?,is_gateway=?,manufacturer=?,model=?,ip_address=?,mac_address=?,note=?,is_openwrt=?,network_key=?,peer_id=?,updated_at=? WHERE id=?`,
		input.Name, nullIfEmpty(input.GroupID), nullIfEmpty(input.GatewayGroupID), input.DeviceType, string(semanticKinds), input.IsGateway,
		input.Manufacturer, input.Model, input.IPAddress, input.MACAddress, input.Note, input.IsOpenWrt, input.NetworkKey, input.PeerID, now, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "idx_devices_gateway_group_gateway") ||
			strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: devices.gateway_group_id") {
			return Device{}, ErrGatewayDeviceConflict
		}
		return Device{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Device{}, err
	}
	if n == 0 {
		return Device{}, ErrDeviceNotFound
	}
	if !input.IsOpenWrt {
		if _, err = tx.ExecContext(ctx, "DELETE FROM device_openwrt_configs WHERE device_id=?", id); err != nil {
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
		COALESCE(MAX(CASE WHEN d.is_gateway=1 THEN d.id END),''),COALESCE(MAX(CASE WHEN d.is_gateway=1 THEN d.name END),''),
		gg.created_at,gg.updated_at
		FROM device_gateway_groups gg LEFT JOIN devices d ON d.gateway_group_id=gg.id
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
		COALESCE(MAX(CASE WHEN d.is_gateway=1 THEN d.id END),''),COALESCE(MAX(CASE WHEN d.is_gateway=1 THEN d.name END),''),
		gg.created_at,gg.updated_at FROM device_gateway_groups gg LEFT JOIN devices d ON d.gateway_group_id=gg.id
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
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_gateway_groups WHERE id=?", id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrGatewayGroupNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, "UPDATE devices SET gateway_group_id=NULL,is_gateway=0,updated_at=? WHERE gateway_group_id=?", now, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM device_gateway_groups WHERE id=?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearDeviceData 原子清理 EasyNetTopo 设备管理模块的数据，不影响节点属性、主机和控制端设置。
func (s *DeviceStore) ClearDeviceData(ctx context.Context) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, statement := range []string{
		"DELETE FROM device_openwrt_configs",
		"DELETE FROM devices",
		"DELETE FROM device_gateway_groups",
		"DELETE FROM device_groups",
		"DELETE FROM network_node_sync_seen",
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
	for _, node := range nodes {
		input := DeviceInput{Name: node.Name, DeviceType: "通用设备", NetworkKey: node.NetworkKey, PeerID: node.PeerID}
		if err := normalizeDevice(&input); err != nil {
			return NetworkNodeSyncResult{}, err
		}
		if input.NetworkKey == "" || input.PeerID == "" {
			return NetworkNodeSyncResult{}, ErrInvalidDevice
		}
		var existingID string
		err := tx.QueryRowContext(ctx, "SELECT id FROM devices WHERE is_node_device=1 AND network_key=? AND peer_id=?", input.NetworkKey, input.PeerID).Scan(&existingID)
		if err == nil {
			if _, err = tx.ExecContext(ctx, "UPDATE devices SET is_node_device=1,updated_at=? WHERE id=?", now, existingID); err != nil {
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
		if _, err = tx.ExecContext(ctx, `INSERT INTO devices(id,name,group_id,gateway_group_id,device_type,semantic_kinds,is_node_device,is_gateway,manufacturer,model,ip_address,mac_address,note,is_openwrt,network_key,peer_id,created_at,updated_at)
			VALUES(?,?,NULL,NULL,'通用设备','[]',1,0,'','','','','',0,?,?,?,?)`, id, input.Name, input.NetworkKey, input.PeerID, now, now); err != nil {
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
	if err := tx.QueryRowContext(ctx, "SELECT is_node_device FROM devices WHERE id=?", id).Scan(&isNodeDevice); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}
	if isNodeDevice {
		return ErrNodeDeviceProtected
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
	if !device.IsOpenWrt {
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
	if !device.IsOpenWrt {
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
	if !device.IsOpenWrt {
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
	if !device.IsOpenWrt {
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
