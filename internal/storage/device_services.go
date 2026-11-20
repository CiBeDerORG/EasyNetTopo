package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidDeviceService        = errors.New("设备服务名称、协议、端口、访问地址、图标或备注不合法")
	ErrDeviceServiceNotFound       = errors.New("设备服务不存在")
	ErrDeviceServiceForwardEnabled = errors.New("请先关闭转发，再修改服务协议或端口")
)

const deviceServiceSchema = `CREATE TABLE IF NOT EXISTS device_services (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	protocol TEXT NOT NULL CHECK(protocol IN ('tcp','udp','tcp_udp')),
	port INTEGER NOT NULL CHECK(port BETWEEN 1 AND 65535),
	access_url TEXT NOT NULL DEFAULT '',
	icon TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	forward_config TEXT NOT NULL DEFAULT '',
	navigation_enabled INTEGER NOT NULL DEFAULT 0
)`

func ensureDeviceServiceColumn(ctx context.Context, db *Store, name, definition string) error {
	var exists bool
	if err := db.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pragma_table_info('device_services') WHERE name=?)`, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := db.db.ExecContext(ctx, "ALTER TABLE device_services ADD COLUMN "+definition)
	return err
}

type DeviceServiceForward struct {
	Enabled     bool     `json:"enabled"`
	HostID      string   `json:"host_id"`
	InstanceID  string   `json:"instance_id"`
	Listen      string   `json:"listen"`
	Destination string   `json:"destination"`
	Protocols   []string `json:"protocols"`
}

type DeviceServiceInput struct {
	Name      string   `json:"name"`
	Protocols []string `json:"protocols"`
	Port      int      `json:"port"`
	AccessURL string   `json:"access_url"`
	Icon      string   `json:"icon"`
	Note      string   `json:"note"`
}

type DeviceService struct {
	ID       string `json:"id"`
	DeviceID string `json:"device_id"`
	DeviceServiceInput
	CreatedAt         string                `json:"created_at"`
	NavigationEnabled bool                  `json:"navigation_enabled"`
	Forward           *DeviceServiceForward `json:"forward,omitempty"`
}

func normalizeDeviceService(input *DeviceServiceInput) (string, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.AccessURL = strings.TrimSpace(input.AccessURL)
	input.Note = strings.TrimSpace(input.Note)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 128 || strings.ContainsFunc(input.Name, unicode.IsControl) ||
		input.Port < 1 || input.Port > 65535 || utf8.RuneCountInString(input.Note) > 2000 ||
		strings.ContainsRune(input.Note, 0) || len(input.AccessURL) > 2048 {
		return "", ErrInvalidDeviceService
	}
	if input.AccessURL != "" {
		parsed, err := url.Parse(input.AccessURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Hostname() == "" || parsed.User != nil || strings.ContainsFunc(input.AccessURL, unicode.IsSpace) ||
			strings.Contains(input.AccessURL, "\\") {
			return "", ErrInvalidDeviceService
		}
	}
	switch input.Icon {
	case "", "web", "storage", "terminal", "media":
	default:
		return "", ErrInvalidDeviceService
	}
	if len(input.Protocols) == 0 || len(input.Protocols) > 2 {
		return "", ErrInvalidDeviceService
	}
	for _, name := range input.Protocols {
		if name != "tcp" && name != "udp" {
			return "", ErrInvalidDeviceService
		}
	}
	tcp, udp := slices.Contains(input.Protocols, "tcp"), slices.Contains(input.Protocols, "udp")
	switch {
	case tcp && udp:
		input.Protocols = []string{"tcp", "udp"}
		return "tcp_udp", nil
	case tcp:
		input.Protocols = []string{"tcp"}
		return "tcp", nil
	default:
		input.Protocols = []string{"udp"}
		return "udp", nil
	}
}

const deviceServiceColumns = `COALESCE(ds.id,''),COALESCE(ds.device_id,''),COALESCE(ds.name,''),COALESCE(ds.protocol,''),COALESCE(ds.port,0),
	COALESCE(ds.access_url,''),COALESCE(ds.icon,''),COALESCE(ds.note,''),COALESCE(ds.created_at,''),COALESCE(ds.forward_config,''),COALESCE(ds.navigation_enabled,0)`

func scanDeviceService(row interface{ Scan(...any) error }) (DeviceService, error) {
	var service DeviceService
	var protocol, forward string
	if err := row.Scan(&service.ID, &service.DeviceID, &service.Name, &protocol, &service.Port, &service.AccessURL, &service.Icon,
		&service.Note, &service.CreatedAt, &forward, &service.NavigationEnabled); err != nil {
		return DeviceService{}, err
	}
	if service.ID != "" {
		service.Protocols = strings.Split(protocol, "_")
	}
	if forward != "" {
		if err := json.Unmarshal([]byte(forward), &service.Forward); err != nil {
			return DeviceService{}, fmt.Errorf("设备服务转发配置无效: %w", err)
		}
	}
	return service, nil
}

func (s *DeviceStore) DeviceService(ctx context.Context, deviceID, serviceID string) (DeviceService, error) {
	row := s.store.db.QueryRowContext(ctx, "SELECT "+deviceServiceColumns+" FROM device_services ds WHERE ds.device_id=? AND ds.id=?", deviceID, serviceID)
	service, err := scanDeviceService(row)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceService{}, ErrDeviceServiceNotFound
	}
	return service, err
}

func (s *DeviceStore) UpdateDeviceService(ctx context.Context, deviceID, serviceID string, input DeviceServiceInput) (DeviceService, error) {
	protocol, err := normalizeDeviceService(&input)
	if err != nil {
		return DeviceService{}, err
	}
	current, err := s.DeviceService(ctx, deviceID, serviceID)
	if err != nil {
		return DeviceService{}, err
	}
	if current.Forward != nil && current.Forward.Enabled && (current.Port != input.Port || !slices.Equal(current.Protocols, input.Protocols)) {
		return DeviceService{}, ErrDeviceServiceForwardEnabled
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE device_services SET name=?,protocol=?,port=?,access_url=?,icon=?,note=?
		WHERE device_id=? AND id=?`, input.Name, protocol, input.Port, input.AccessURL, input.Icon, input.Note, deviceID, serviceID); err != nil {
		return DeviceService{}, err
	}
	return s.DeviceService(ctx, deviceID, serviceID)
}

func (s *DeviceStore) DeleteDeviceService(ctx context.Context, deviceID, serviceID string) error {
	result, err := s.store.db.ExecContext(ctx, "DELETE FROM device_services WHERE device_id=? AND id=?", deviceID, serviceID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrDeviceServiceNotFound
	}
	return nil
}

func (s *DeviceStore) SaveDeviceServiceForward(ctx context.Context, deviceID, serviceID string, forward *DeviceServiceForward) (DeviceService, error) {
	value := ""
	if forward != nil {
		data, err := json.Marshal(forward)
		if err != nil {
			return DeviceService{}, err
		}
		value = string(data)
	}
	if _, err := s.store.db.ExecContext(ctx, "UPDATE device_services SET forward_config=? WHERE device_id=? AND id=?", value, deviceID, serviceID); err != nil {
		return DeviceService{}, err
	}
	return s.DeviceService(ctx, deviceID, serviceID)
}

func (s *DeviceStore) ListDeviceServices(ctx context.Context, deviceID string) ([]DeviceService, error) {
	rows, err := s.store.db.QueryContext(ctx, "SELECT "+deviceServiceColumns+` FROM devices d LEFT JOIN device_services ds ON ds.device_id=d.id WHERE d.id=?
		ORDER BY ds.name COLLATE NOCASE,ds.id`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DeviceService, 0)
	exists := false
	for rows.Next() {
		exists = true
		service, err := scanDeviceService(rows)
		if err != nil {
			return nil, err
		}
		if service.ID != "" {
			items = append(items, service)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return items, nil
}

func (s *DeviceStore) SetDeviceServiceNavigation(ctx context.Context, deviceID, serviceID string, enabled bool) (DeviceService, error) {
	if _, err := s.store.db.ExecContext(ctx, "UPDATE device_services SET navigation_enabled=? WHERE device_id=? AND id=?", enabled, deviceID, serviceID); err != nil {
		return DeviceService{}, err
	}
	return s.DeviceService(ctx, deviceID, serviceID)
}

func (s *DeviceStore) ListNavigationServices(ctx context.Context) ([]DeviceService, error) {
	rows, err := s.store.db.QueryContext(ctx, "SELECT "+deviceServiceColumns+` FROM device_services ds
		WHERE ds.navigation_enabled=1 ORDER BY ds.name COLLATE NOCASE,ds.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DeviceService, 0)
	for rows.Next() {
		service, err := scanDeviceService(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, service)
	}
	return items, rows.Err()
}

func (s *DeviceStore) CreateDeviceService(ctx context.Context, deviceID string, input DeviceServiceInput) (DeviceService, error) {
	protocol, err := normalizeDeviceService(&input)
	if err != nil {
		return DeviceService{}, err
	}
	id, err := uuid()
	if err != nil {
		return DeviceService{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.store.db.ExecContext(ctx, `INSERT INTO device_services
		(id,device_id,name,protocol,port,access_url,icon,note,created_at)
		SELECT ?,id,?,?,?,?,?,?,? FROM devices WHERE id=?`,
		id, input.Name, protocol, input.Port, input.AccessURL, input.Icon, input.Note, now, deviceID)
	if err != nil {
		return DeviceService{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return DeviceService{}, err
	}
	if count == 0 {
		return DeviceService{}, ErrDeviceNotFound
	}
	return DeviceService{ID: id, DeviceID: deviceID, DeviceServiceInput: input, CreatedAt: now}, nil
}
