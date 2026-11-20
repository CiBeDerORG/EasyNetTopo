package storage

import (
	"context"
	"encoding/json"
	"slices"
	"time"
)

type DeviceServiceImport struct {
	DeviceServiceInput
	Source DeviceServiceSource
}

type DeviceServiceImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// MatchesExisting 保留同来源入口及占用当前端口协议的已有服务。
func (item DeviceServiceImport) MatchesExisting(service DeviceService) bool {
	source := service.Source
	if source != nil && source.HostID == item.Source.HostID && source.SourceID == item.Source.SourceID &&
		source.AppID == item.Source.AppID && source.OriginalPort == item.Source.OriginalPort {
		return true
	}
	return service.Port == item.Port && slices.ContainsFunc(item.Protocols, func(protocol string) bool {
		return slices.Contains(service.Protocols, protocol)
	})
}

func (s *DeviceStore) ImportDeviceServices(ctx context.Context, deviceID string, items []DeviceServiceImport) (DeviceServiceImportResult, error) {
	var result DeviceServiceImportResult
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()

	existing, err := listDeviceServices(ctx, tx, deviceID)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range items {
		input := item.DeviceServiceInput
		protocol, err := normalizeDeviceService(&input)
		if err != nil {
			return DeviceServiceImportResult{}, err
		}
		if slices.ContainsFunc(existing, item.MatchesExisting) {
			result.Skipped++
			continue
		}
		id, err := uuid()
		if err != nil {
			return DeviceServiceImportResult{}, err
		}
		source, err := json.Marshal(item.Source)
		if err != nil {
			return DeviceServiceImportResult{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO device_services
			(id,device_id,name,protocol,port,access_url,icon,note,created_at,import_source)
			VALUES(?,?,?,?,?,?,?,?,?,?)`, id, deviceID, input.Name, protocol, input.Port,
			input.AccessURL, input.Icon, input.Note, now, string(source)); err != nil {
			return DeviceServiceImportResult{}, err
		}
		existing = append(existing, DeviceService{DeviceServiceInput: input, Source: &item.Source})
		result.Imported++
	}
	if err := tx.Commit(); err != nil {
		return DeviceServiceImportResult{}, err
	}
	return result, nil
}
