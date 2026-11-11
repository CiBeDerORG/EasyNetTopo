package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
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
	ErrDeviceNotFound        = errors.New("设备不存在")
	ErrDeviceGroupNotFound   = errors.New("设备分组不存在")
	ErrDeviceGroupExists     = errors.New("设备分组名称已存在")
	ErrDeviceNodeConflict    = errors.New("该网络节点已关联其他设备")
	ErrInvalidDevice         = errors.New("设备信息不合法")
	ErrInvalidDeviceGroup    = errors.New("设备分组名称不合法")
	ErrDeviceNotOpenWrt      = errors.New("该设备未标记为 OpenWrt")
	ErrOpenWrtConfigNotFound = errors.New("OpenWrt 发现服务尚未配置")
	ErrInvalidOpenWrtConfig  = errors.New("OpenWrt 发现服务配置不合法")
)

type DeviceGroup struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DeviceCount int    `json:"device_count"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Device struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	GroupID           string `json:"group_id,omitempty"`
	GroupName         string `json:"group_name,omitempty"`
	DeviceType        string `json:"device_type"`
	Manufacturer      string `json:"manufacturer,omitempty"`
	Model             string `json:"model,omitempty"`
	IPAddress         string `json:"ip_address,omitempty"`
	MACAddress        string `json:"mac_address,omitempty"`
	Note              string `json:"note,omitempty"`
	IsOpenWrt         bool   `json:"is_openwrt"`
	NetworkKey        string `json:"network_key,omitempty"`
	PeerID            string `json:"peer_id,omitempty"`
	OpenWrtConfigured bool   `json:"openwrt_configured"`
	OpenWrtRPCDPort   int    `json:"openwrt_rpcd_port,omitempty"`
	OpenWrtHTTPS      bool   `json:"openwrt_https,omitempty"`
	OpenWrtUsername   string `json:"openwrt_username,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type DeviceInput struct {
	Name         string `json:"name"`
	GroupID      string `json:"group_id"`
	DeviceType   string `json:"device_type"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	IPAddress    string `json:"ip_address"`
	MACAddress   string `json:"mac_address"`
	Note         string `json:"note"`
	IsOpenWrt    bool   `json:"is_openwrt"`
	NetworkKey   string `json:"network_key"`
	PeerID       string `json:"peer_id"`
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
			device_type TEXT NOT NULL DEFAULT 'other', manufacturer TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '', ip_address TEXT NOT NULL DEFAULT '', mac_address TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '', is_openwrt INTEGER NOT NULL DEFAULT 0,
			network_key TEXT NOT NULL DEFAULT '', peer_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			CHECK ((network_key='' AND peer_id='') OR (network_key<>'' AND peer_id<>''))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_network_node
			ON devices(network_key,peer_id) WHERE network_key<>'' AND peer_id<>''`,
		`CREATE INDEX IF NOT EXISTS idx_devices_group ON devices(group_id,name)`,
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
		v.DeviceType = "other"
	}
	switch v.DeviceType {
	case "router", "computer", "phone", "tablet", "tv", "nas", "other":
	default:
		return ErrInvalidDevice
	}
	if v.IsOpenWrt && v.DeviceType != "router" {
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

const deviceSelect = `SELECT d.id,d.name,COALESCE(d.group_id,''),COALESCE(g.name,''),d.device_type,d.manufacturer,d.model,d.ip_address,d.mac_address,d.note,d.is_openwrt,d.network_key,d.peer_id,
	CASE WHEN c.device_id IS NULL THEN 0 ELSE 1 END,COALESCE(c.rpcd_port,0),COALESCE(c.https,0),COALESCE(c.username,''),d.created_at,d.updated_at
	FROM devices d LEFT JOIN device_groups g ON g.id=d.group_id LEFT JOIN device_openwrt_configs c ON c.device_id=d.id`

func scanDevice(row interface{ Scan(...any) error }) (Device, error) {
	var v Device
	var configured, https bool
	err := row.Scan(&v.ID, &v.Name, &v.GroupID, &v.GroupName, &v.DeviceType, &v.Manufacturer, &v.Model, &v.IPAddress, &v.MACAddress, &v.Note, &v.IsOpenWrt, &v.NetworkKey, &v.PeerID, &configured, &v.OpenWrtRPCDPort, &https, &v.OpenWrtUsername, &v.CreatedAt, &v.UpdatedAt)
	v.OpenWrtConfigured = configured
	v.OpenWrtHTTPS = https
	return v, err
}
func (s *DeviceStore) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.store.db.QueryContext(ctx, deviceSelect+" ORDER BY COALESCE(g.name,'') COLLATE NOCASE,d.name COLLATE NOCASE,d.id")
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
func (s *DeviceStore) Device(ctx context.Context, id string) (Device, error) {
	v, err := scanDevice(s.store.db.QueryRowContext(ctx, deviceSelect+" WHERE d.id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, ErrDeviceNotFound
	}
	return v, err
}
func (s *DeviceStore) CreateDevice(ctx context.Context, input DeviceInput) (Device, error) {
	if err := normalizeDevice(&input); err != nil {
		return Device{}, err
	}
	id, err := uuid()
	if err != nil {
		return Device{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO devices(id,name,group_id,device_type,manufacturer,model,ip_address,mac_address,note,is_openwrt,network_key,peer_id,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, input.Name, nullIfEmpty(input.GroupID), input.DeviceType, input.Manufacturer, input.Model, input.IPAddress, input.MACAddress, input.Note, input.IsOpenWrt, input.NetworkKey, input.PeerID, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "foreign key") {
			return Device{}, ErrDeviceGroupNotFound
		}
		if strings.Contains(strings.ToLower(err.Error()), "idx_devices_network_node") || strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: devices.network_key") {
			return Device{}, ErrDeviceNodeConflict
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
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.ExecContext(ctx, `UPDATE devices SET name=?,group_id=?,device_type=?,manufacturer=?,model=?,ip_address=?,mac_address=?,note=?,is_openwrt=?,network_key=?,peer_id=?,updated_at=? WHERE id=?`,
		input.Name, nullIfEmpty(input.GroupID), input.DeviceType, input.Manufacturer, input.Model, input.IPAddress, input.MACAddress, input.Note, input.IsOpenWrt, input.NetworkKey, input.PeerID, now, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "foreign key") {
			return Device{}, ErrDeviceGroupNotFound
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: devices.network_key") {
			return Device{}, ErrDeviceNodeConflict
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
func (s *DeviceStore) DeleteDevice(ctx context.Context, id string) error {
	result, err := s.store.db.ExecContext(ctx, "DELETE FROM devices WHERE id=?", id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrDeviceNotFound
	}
	return nil
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
