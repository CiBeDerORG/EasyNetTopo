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
	Alias       string       `json:"alias"`
	HasPublicIP bool         `json:"has_public_ip"`
	PublicIP    string       `json:"public_ip"`
	Location    NodeLocation `json:"location"`
}

func (s *Store) ListNodeAttributes(ctx context.Context) ([]NodeAttributes, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT network_key,peer_id,alias,has_public_ip,public_ip,country,province,city,county,latitude,longitude FROM network_node_attributes ORDER BY network_key,peer_id`)
	if err != nil { return nil, err }
	defer rows.Close()
	result := make([]NodeAttributes, 0)
	for rows.Next() {
		var attr NodeAttributes
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&attr.NetworkKey, &attr.PeerID, &attr.Alias, &attr.HasPublicIP, &attr.PublicIP,
			&attr.Location.Country, &attr.Location.Province, &attr.Location.City, &attr.Location.County,
			&lat, &lng); err != nil { return nil, err }
		if lat.Valid { value := lat.Float64; attr.Location.Latitude = &value }
		if lng.Valid { value := lng.Float64; attr.Location.Longitude = &value }
		result = append(result, attr)
	}
	return result, rows.Err()
}

func normalizeNodeAttributes(attr *NodeAttributes) error {
	attr.NetworkKey = strings.TrimSpace(attr.NetworkKey)
	attr.PeerID = strings.TrimSpace(attr.PeerID)
	attr.Alias = strings.TrimSpace(attr.Alias)
	attr.PublicIP = strings.TrimSpace(attr.PublicIP)
	for _, value := range []string{attr.NetworkKey, attr.PeerID} {
		if value == "" || value == "—" { return ErrInvalidNodeAttributes }
	}
	if len(attr.NetworkKey) > 256 || len(attr.PeerID) > 128 || len([]rune(attr.Alias)) > 100 {
		return ErrInvalidNodeAttributes
	}
	for _, field := range []*string{&attr.Location.Country, &attr.Location.Province, &attr.Location.City, &attr.Location.County} {
		*field = strings.TrimSpace(*field)
		if len([]rune(*field)) > 120 { return ErrInvalidNodeAttributes }
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
	if !attr.HasPublicIP { attr.PublicIP = "" }
	if attr.PublicIP != "" {
		ip, err := netip.ParseAddr(attr.PublicIP)
		if err != nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return ErrInvalidNodeAttributes
		}
		attr.PublicIP = ip.String()
	}
	return nil
}

func (s *Store) SaveNodeAttributes(ctx context.Context, attr NodeAttributes) (NodeAttributes, error) {
	if err := normalizeNodeAttributes(&attr); err != nil { return NodeAttributes{}, err }
	_, err := s.db.ExecContext(ctx, `INSERT INTO network_node_attributes
		(network_key,peer_id,alias,has_public_ip,public_ip,country,province,city,county,latitude,longitude,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(network_key,peer_id) DO UPDATE SET
		alias=excluded.alias,has_public_ip=excluded.has_public_ip,public_ip=excluded.public_ip,
		country=excluded.country,province=excluded.province,city=excluded.city,county=excluded.county,
		latitude=excluded.latitude,longitude=excluded.longitude,updated_at=excluded.updated_at`,
		attr.NetworkKey, attr.PeerID, attr.Alias, attr.HasPublicIP, attr.PublicIP,
		attr.Location.Country, attr.Location.Province, attr.Location.City, attr.Location.County,
		attr.Location.Latitude, attr.Location.Longitude, time.Now().UTC().Format(time.RFC3339))
	return attr, err
}
