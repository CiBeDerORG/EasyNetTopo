package storage

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net/netip"
	"strings"
	"time"
)

var ErrInvalidNodeAttributes = errors.New("节点属性不合法")

// 节点属性保存在控制端 SQLite；network_key 为组网名称，peer_id 为 EasyTier 节点标识。
// 未报告组网名称的实例由前端使用入口主机与实例组成的独立作用域。
type NodeLocation struct {
	Country   string   `json:"country"`
	Province  string   `json:"province"`
	City      string   `json:"city"`
	County    string   `json:"county"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type NodeAttributes struct {
	NetworkKey  string       `json:"network_key"`
	PeerID      string       `json:"peer_id"`
	Hostname    string       `json:"hostname,omitempty"` // 原始主机名，显示别名不参与身份匹配
	Alias       string       `json:"alias"`
	HasPublicIP bool         `json:"has_public_ip"`
	PublicIP    string       `json:"public_ip"`
	Location    NodeLocation `json:"location"`
}

// 旧数据库保留原属性主键，同时为稳定的原始 hostname 建立唯一索引。
func ensureNodeAttributeHostname(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(network_node_attributes)")
	if err != nil {
		return err
	}
	exists := false
	for rows.Next() {
		var cid, notNull, primary int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &primary); err != nil {
			rows.Close()
			return err
		}
		if name == "hostname" {
			exists = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !exists {
		if _, err := db.Exec("ALTER TABLE network_node_attributes ADD COLUMN hostname TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_node_attributes_hostname
		ON network_node_attributes(network_key,hostname COLLATE NOCASE) WHERE hostname<>''`)
	return err
}

func (s *Store) ListNodeAttributes(ctx context.Context) ([]NodeAttributes, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT network_key,peer_id,hostname,alias,has_public_ip,public_ip,country,province,city,county,latitude,longitude FROM network_node_attributes ORDER BY network_key,peer_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]NodeAttributes, 0)
	for rows.Next() {
		var attr NodeAttributes
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&attr.NetworkKey, &attr.PeerID, &attr.Hostname, &attr.Alias, &attr.HasPublicIP, &attr.PublicIP,
			&attr.Location.Country, &attr.Location.Province, &attr.Location.City, &attr.Location.County,
			&lat, &lng); err != nil {
			return nil, err
		}
		if lat.Valid {
			value := lat.Float64
			attr.Location.Latitude = &value
		}
		if lng.Valid {
			value := lng.Float64
			attr.Location.Longitude = &value
		}
		result = append(result, attr)
	}
	return result, rows.Err()
}

func normalizeNodeAttributes(attr *NodeAttributes) error {
	attr.NetworkKey = strings.TrimSpace(attr.NetworkKey)
	attr.PeerID = strings.TrimSpace(attr.PeerID)
	attr.Hostname = strings.TrimSpace(attr.Hostname)
	attr.Alias = strings.TrimSpace(attr.Alias)
	attr.PublicIP = strings.TrimSpace(attr.PublicIP)
	for _, value := range []string{attr.NetworkKey, attr.PeerID} {
		if value == "" || value == "—" {
			return ErrInvalidNodeAttributes
		}
	}
	if len(attr.NetworkKey) > 256 || len(attr.PeerID) > 128 || len([]rune(attr.Hostname)) > 100 ||
		attr.Hostname == "—" || strings.ContainsAny(attr.Hostname, "\r\n\x00") || len([]rune(attr.Alias)) > 100 {
		return ErrInvalidNodeAttributes
	}
	for _, field := range []*string{&attr.Location.Country, &attr.Location.Province, &attr.Location.City, &attr.Location.County} {
		*field = strings.TrimSpace(*field)
		if len([]rune(*field)) > 120 {
			return ErrInvalidNodeAttributes
		}
	}
	if (attr.Location.Latitude == nil) != (attr.Location.Longitude == nil) {
		return ErrInvalidNodeAttributes
	}
	if attr.Location.Latitude != nil && (math.IsNaN(*attr.Location.Latitude) || math.IsInf(*attr.Location.Latitude, 0) ||
		math.IsNaN(*attr.Location.Longitude) || math.IsInf(*attr.Location.Longitude, 0) ||
		*attr.Location.Latitude < -85.05112878 || *attr.Location.Latitude > 85.05112878 ||
		*attr.Location.Longitude < -180 || *attr.Location.Longitude > 180) {
		return ErrInvalidNodeAttributes
	}
	if !attr.HasPublicIP {
		attr.PublicIP = ""
	}
	if attr.PublicIP != "" {
		ip, err := netip.ParseAddr(attr.PublicIP)
		if err != nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return ErrInvalidNodeAttributes
		}
		attr.PublicIP = ip.String()
	}
	return nil
}

func (s *Store) DeleteNodeAttributes(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM network_node_attributes")
	return err
}

func (s *Store) SaveNodeAttributes(ctx context.Context, attr NodeAttributes) (NodeAttributes, error) {
	if err := normalizeNodeAttributes(&attr); err != nil {
		return NodeAttributes{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return NodeAttributes{}, err
	}
	defer tx.Rollback()
	if attr.Hostname != "" {
		// 以原主机名找到旧 Peer ID 的人工属性，并在无目标冲突时原地换绑。
		var previousPeerID string
		err = tx.QueryRowContext(ctx, `SELECT peer_id FROM network_node_attributes
			WHERE network_key=? AND hostname=? COLLATE NOCASE`, attr.NetworkKey, attr.Hostname).Scan(&previousPeerID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return NodeAttributes{}, err
		}
		if err == nil && previousPeerID != attr.PeerID {
			var target int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM network_node_attributes
				WHERE network_key=? AND peer_id=?`, attr.NetworkKey, attr.PeerID).Scan(&target); err != nil {
				return NodeAttributes{}, err
			}
			if target != 0 {
				return NodeAttributes{}, ErrInvalidNodeAttributes
			}
			if _, err := tx.ExecContext(ctx, `UPDATE network_node_attributes SET peer_id=?
				WHERE network_key=? AND peer_id=?`, attr.PeerID, attr.NetworkKey, previousPeerID); err != nil {
				return NodeAttributes{}, err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO network_node_attributes
		(network_key,peer_id,hostname,alias,has_public_ip,public_ip,country,province,city,county,latitude,longitude,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(network_key,peer_id) DO UPDATE SET
		hostname=excluded.hostname,alias=excluded.alias,has_public_ip=excluded.has_public_ip,public_ip=excluded.public_ip,
		country=excluded.country,province=excluded.province,city=excluded.city,county=excluded.county,
		latitude=excluded.latitude,longitude=excluded.longitude,updated_at=excluded.updated_at`,
		attr.NetworkKey, attr.PeerID, attr.Hostname, attr.Alias, attr.HasPublicIP, attr.PublicIP,
		attr.Location.Country, attr.Location.Province, attr.Location.City, attr.Location.County,
		attr.Location.Latitude, attr.Location.Longitude, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return NodeAttributes{}, err
	}
	return attr, tx.Commit()
}
