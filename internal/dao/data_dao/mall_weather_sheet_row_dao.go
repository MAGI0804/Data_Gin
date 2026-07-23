package data_dao

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/credential"
	"gin-biz-web-api/pkg/database"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	mallWeatherSheetMappingStateKey = "state:initialized:v1"
	maxMallWeatherSheetMappingBatch = 500
	maxMallWeatherSheetBusinessKey  = 512
	maxMallWeatherSheetRowNumber    = int64(10_000_000)
)

var mallWeatherSheetDatasets = map[string]struct{}{
	"realtime": {}, "minutely": {}, "hourly": {}, "daily": {}, "alerts": {}, "life_indices": {},
}

var ErrMallWeatherSheetDuplicateRemoteMapping = errors.New(
	"mall weather sheet row: duplicate remote mapping",
)

var mallWeatherSheetEnvNames = map[string]struct{}{
	credential.EnvFeishuWeatherRealtimeSheetID:  {},
	credential.EnvFeishuWeatherMinutelySheetID:  {},
	credential.EnvFeishuWeatherHourlySheetID:    {},
	credential.EnvFeishuWeatherDailySheetID:     {},
	credential.EnvFeishuWeatherAlertSheetID:     {},
	credential.EnvFeishuWeatherLifeIndexSheetID: {},
}

type MallWeatherSheetRowMapping struct {
	BusinessKey string
	RowNumber   int64
	Checksum    string
}

type MallWeatherSheetRowDAO struct {
	db *gorm.DB
}

func NewMallWeatherSheetRowDAO(databases ...*gorm.DB) *MallWeatherSheetRowDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallWeatherSheetRowDAO{db: db}
}

func (dao *MallWeatherSheetRowDAO) FindByBusinessKeys(
	ctx context.Context,
	destinationID uint,
	datasetKind string,
	businessKeys []string,
) (map[string]model.MallWeatherSheetRow, error) {
	datasetKind = strings.TrimSpace(datasetKind)
	if dao == nil || dao.db == nil || ctx == nil || destinationID == 0 ||
		!validMallWeatherSheetDataset(datasetKind) || len(businessKeys) == 0 ||
		len(businessKeys) > maxMallWeatherSheetMappingBatch {
		return nil, errors.New("mall weather sheet row: invalid mapping query")
	}
	keys := make([]string, len(businessKeys))
	seen := make(map[string]struct{}, len(businessKeys))
	for index, key := range businessKeys {
		if !validMallWeatherSheetBusinessKey(key) {
			return nil, errors.New("mall weather sheet row: invalid business key")
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, errors.New("mall weather sheet row: duplicate business key")
		}
		seen[key] = struct{}{}
		keys[index] = key
	}
	var rows []model.MallWeatherSheetRow
	if err := dao.db.WithContext(ctx).
		Where("destination_id = ? AND dataset_kind = ? AND business_key IN ?", destinationID, datasetKind, keys).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather sheet row: query mappings: %w", err)
	}
	result := make(map[string]model.MallWeatherSheetRow, len(rows))
	for _, row := range rows {
		if !validStoredMallWeatherSheetRow(row, destinationID, datasetKind) {
			return nil, errors.New("mall weather sheet row: invalid stored mapping")
		}
		if _, duplicate := result[row.BusinessKey]; duplicate {
			return nil, errors.New("mall weather sheet row: duplicate stored mapping")
		}
		result[row.BusinessKey] = row
	}
	return result, nil
}

func (dao *MallWeatherSheetRowDAO) IsInitialized(
	ctx context.Context,
	destinationID uint,
	datasetKind string,
	sheetIDEnv string,
	schemaChecksum string,
) (bool, error) {
	if dao == nil || dao.db == nil || ctx == nil || destinationID == 0 ||
		!validMallWeatherSheetDataset(datasetKind) || !validMallWeatherSheetEnv(sheetIDEnv) ||
		!validMallWeatherSheetChecksum(schemaChecksum) {
		return false, errors.New("mall weather sheet row: invalid initialization query")
	}
	var count int64
	err := dao.db.WithContext(ctx).Model(&model.MallWeatherSheetRow{}).
		Where(
			"destination_id = ? AND dataset_kind = ? AND business_key = ? AND sheet_id_env = ? AND row_number = ? AND checksum = ?",
			destinationID,
			datasetKind,
			mallWeatherSheetMappingStateKey,
			sheetIDEnv,
			1,
			schemaChecksum,
		).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("mall weather sheet row: query initialization state: %w", err)
	}
	if count < 0 || count > 1 {
		return false, errors.New("mall weather sheet row: invalid initialization state")
	}
	return count == 1, nil
}

func (dao *MallWeatherSheetRowDAO) UpsertMappings(
	ctx context.Context,
	destinationID uint,
	datasetKind string,
	sheetIDEnv string,
	mappings []MallWeatherSheetRowMapping,
	syncedAt time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil {
		return errors.New("mall weather sheet row: invalid mapping upsert")
	}
	rows, err := buildMallWeatherSheetRows(destinationID, datasetKind, sheetIDEnv, mappings, syncedAt)
	if err != nil {
		return fmt.Errorf("mall weather sheet row: invalid mapping upsert: %w", err)
	}
	result := dao.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "destination_id"}, {Name: "dataset_kind"}, {Name: "business_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sheet_id_env", "row_number", "checksum", "last_synced_at", "updated_at",
		}),
	}).Create(&rows)
	if result.Error != nil {
		return fmt.Errorf("mall weather sheet row: upsert mappings: %w", result.Error)
	}
	return nil
}

func (dao *MallWeatherSheetRowDAO) CreateScannedMappings(
	ctx context.Context,
	destinationID uint,
	datasetKind string,
	sheetIDEnv string,
	mappings []MallWeatherSheetRowMapping,
	syncedAt time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil {
		return errors.New("mall weather sheet row: invalid scanned mapping create")
	}
	rows, err := buildMallWeatherSheetRows(destinationID, datasetKind, sheetIDEnv, mappings, syncedAt)
	if err != nil {
		return fmt.Errorf("mall weather sheet row: invalid scanned mappings: %w", err)
	}
	result := dao.db.WithContext(ctx).Create(&rows)
	if result.Error != nil {
		var mysqlError *mysqlDriver.MySQLError
		if errors.As(result.Error, &mysqlError) && mysqlError != nil && mysqlError.Number == 1062 {
			return fmt.Errorf("%w: %v", ErrMallWeatherSheetDuplicateRemoteMapping, result.Error)
		}
		return fmt.Errorf("mall weather sheet row: create scanned mappings: %w", result.Error)
	}
	if result.RowsAffected != int64(len(rows)) {
		return errors.New("mall weather sheet row: scanned mapping count changed")
	}
	return nil
}

func (dao *MallWeatherSheetRowDAO) ResetMappings(
	ctx context.Context,
	destinationID uint,
	datasetKind string,
) error {
	if dao == nil || dao.db == nil || ctx == nil || destinationID == 0 ||
		!validMallWeatherSheetDataset(datasetKind) {
		return errors.New("mall weather sheet row: invalid mapping reset")
	}
	result := dao.db.WithContext(ctx).
		Where("destination_id = ? AND dataset_kind = ?", destinationID, datasetKind).
		Delete(&model.MallWeatherSheetRow{})
	if result.Error != nil {
		return fmt.Errorf("mall weather sheet row: reset mappings: %w", result.Error)
	}
	return nil
}

func (dao *MallWeatherSheetRowDAO) MarkInitialized(
	ctx context.Context,
	destinationID uint,
	datasetKind string,
	sheetIDEnv string,
	schemaChecksum string,
	initializedAt time.Time,
) error {
	if dao == nil || dao.db == nil || ctx == nil || destinationID == 0 ||
		!validMallWeatherSheetDataset(datasetKind) || !validMallWeatherSheetEnv(sheetIDEnv) ||
		!validMallWeatherSheetChecksum(schemaChecksum) || initializedAt.IsZero() {
		return errors.New("mall weather sheet row: invalid initialization marker")
	}
	initializedAt = initializedAt.UTC().Truncate(time.Millisecond)
	row := model.MallWeatherSheetRow{
		DestinationID: destinationID,
		DatasetKind:   datasetKind,
		BusinessKey:   mallWeatherSheetMappingStateKey,
		SheetIDEnv:    sheetIDEnv,
		RowNumber:     1,
		Checksum:      schemaChecksum,
		LastSyncedAt:  initializedAt,
		WeatherTimestamps: model.WeatherTimestamps{
			CreatedAt: initializedAt,
			UpdatedAt: initializedAt,
		},
	}
	result := dao.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "destination_id"}, {Name: "dataset_kind"}, {Name: "business_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sheet_id_env", "row_number", "checksum", "last_synced_at", "updated_at",
		}),
	}).Create(&row)
	if result.Error != nil {
		return fmt.Errorf("mall weather sheet row: mark initialized: %w", result.Error)
	}
	return nil
}

func validMallWeatherSheetDataset(value string) bool {
	_, exists := mallWeatherSheetDatasets[value]
	return exists
}

func buildMallWeatherSheetRows(
	destinationID uint,
	datasetKind string,
	sheetIDEnv string,
	mappings []MallWeatherSheetRowMapping,
	syncedAt time.Time,
) ([]model.MallWeatherSheetRow, error) {
	if destinationID == 0 || !validMallWeatherSheetDataset(datasetKind) || !validMallWeatherSheetEnv(sheetIDEnv) ||
		len(mappings) == 0 || len(mappings) > maxMallWeatherSheetMappingBatch || syncedAt.IsZero() {
		return nil, errors.New("invalid mappings")
	}
	syncedAt = syncedAt.UTC().Truncate(time.Millisecond)
	rows := make([]model.MallWeatherSheetRow, len(mappings))
	seenKeys := make(map[string]struct{}, len(mappings))
	seenRows := make(map[int64]struct{}, len(mappings))
	for index, mapping := range mappings {
		if !validMallWeatherSheetBusinessKey(mapping.BusinessKey) || mapping.RowNumber < 2 ||
			mapping.RowNumber > maxMallWeatherSheetRowNumber || !validMallWeatherSheetChecksum(mapping.Checksum) {
			return nil, errors.New("invalid mapping")
		}
		if _, duplicate := seenKeys[mapping.BusinessKey]; duplicate {
			return nil, errors.New("duplicate mapping key")
		}
		if _, duplicate := seenRows[mapping.RowNumber]; duplicate {
			return nil, errors.New("duplicate mapping row")
		}
		seenKeys[mapping.BusinessKey] = struct{}{}
		seenRows[mapping.RowNumber] = struct{}{}
		rows[index] = model.MallWeatherSheetRow{
			DestinationID: destinationID,
			DatasetKind:   datasetKind,
			BusinessKey:   mapping.BusinessKey,
			SheetIDEnv:    sheetIDEnv,
			RowNumber:     mapping.RowNumber,
			Checksum:      mapping.Checksum,
			LastSyncedAt:  syncedAt,
			WeatherTimestamps: model.WeatherTimestamps{
				CreatedAt: syncedAt,
				UpdatedAt: syncedAt,
			},
		}
	}
	return rows, nil
}

func validMallWeatherSheetEnv(value string) bool {
	_, exists := mallWeatherSheetEnvNames[value]
	return exists
}

func validMallWeatherSheetBusinessKey(value string) bool {
	return value != "" && value != mallWeatherSheetMappingStateKey && len(value) <= maxMallWeatherSheetBusinessKey &&
		value == strings.TrimSpace(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validMallWeatherSheetChecksum(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}

func validStoredMallWeatherSheetRow(
	row model.MallWeatherSheetRow,
	destinationID uint,
	datasetKind string,
) bool {
	return row.ID != 0 && row.DestinationID == destinationID && row.DatasetKind == datasetKind &&
		validMallWeatherSheetBusinessKey(row.BusinessKey) && validMallWeatherSheetEnv(row.SheetIDEnv) &&
		row.RowNumber >= 2 && row.RowNumber <= maxMallWeatherSheetRowNumber &&
		validMallWeatherSheetChecksum(row.Checksum) && !row.LastSyncedAt.IsZero()
}
