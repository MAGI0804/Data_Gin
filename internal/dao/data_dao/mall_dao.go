package data_dao

import (
	"context"
	"errors"
	"fmt"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMallNotFound        = errors.New("mall: not found")
	ErrMallVersionConflict = errors.New("mall: version conflict")
)

const maxMallPageSize = 200

type MallListFilter struct {
	AfterID        uint
	Limit          int
	City           string
	Status         string
	GeocodeStatus  string
	WeatherEnabled *bool
}

type MallDAO struct {
	db *gorm.DB
}

func NewMallDAO(databases ...*gorm.DB) *MallDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallDAO{db: db}
}

func (dao *MallDAO) WithDB(db *gorm.DB) *MallDAO {
	return &MallDAO{db: db}
}

func (dao *MallDAO) Create(ctx context.Context, mall *model.Mall) error {
	if mall == nil {
		return fmt.Errorf("mall: create nil model")
	}
	return dao.db.WithContext(ctx).Create(mall).Error
}

func (dao *MallDAO) FindByID(ctx context.Context, id uint) (*model.Mall, error) {
	return dao.findByID(ctx, id, false)
}

func (dao *MallDAO) FindByIDForUpdate(ctx context.Context, id uint) (*model.Mall, error) {
	return dao.findByID(ctx, id, true)
}

func (dao *MallDAO) findByID(ctx context.Context, id uint, forUpdate bool) (*model.Mall, error) {
	var mall model.Mall
	query := dao.db.WithContext(ctx)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&mall, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMallNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mall: find by id: %w", err)
	}
	return &mall, nil
}

func (dao *MallDAO) FindByCode(ctx context.Context, code string) (*model.Mall, error) {
	var mall model.Mall
	err := dao.db.WithContext(ctx).Where("mall_code = ?", code).First(&mall).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMallNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mall: find by code: %w", err)
	}
	return &mall, nil
}

func (dao *MallDAO) List(ctx context.Context, filter MallListFilter) ([]model.Mall, error) {
	query := dao.db.WithContext(ctx).Model(&model.Mall{})
	if filter.AfterID > 0 {
		query = query.Where("id > ?", filter.AfterID)
	}
	if filter.City != "" {
		query = query.Where("city = ?", filter.City)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.GeocodeStatus != "" {
		query = query.Where("geocode_status = ?", filter.GeocodeStatus)
	}
	if filter.WeatherEnabled != nil {
		query = query.Where("weather_enabled = ?", *filter.WeatherEnabled)
	}

	var malls []model.Mall
	if err := query.Order("id ASC").Limit(normalizePageSize(filter.Limit)).Find(&malls).Error; err != nil {
		return nil, fmt.Errorf("mall: list: %w", err)
	}
	return malls, nil
}

func (dao *MallDAO) ListEnabledWeatherAfterID(ctx context.Context, afterID uint, limit int) ([]model.Mall, error) {
	confirmed := "confirmed"
	enabled := true
	return dao.List(ctx, MallListFilter{
		AfterID:        afterID,
		Limit:          limit,
		Status:         "active",
		GeocodeStatus:  confirmed,
		WeatherEnabled: &enabled,
	})
}

func (dao *MallDAO) UpdateWithVersion(ctx context.Context, id uint, expectedVersion uint64, updates map[string]interface{}) error {
	safeUpdates, err := sanitizeMallUpdates(updates)
	if err != nil {
		return err
	}
	safeUpdates["version"] = gorm.Expr("version + 1")
	result := dao.db.WithContext(ctx).
		Model(&model.Mall{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(safeUpdates)
	if result.Error != nil {
		return fmt.Errorf("mall: update: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrMallVersionConflict
	}
	return nil
}

func (dao *MallDAO) DeleteWithVersion(ctx context.Context, id uint, expectedVersion uint64) error {
	result := dao.db.WithContext(ctx).
		Where("id = ? AND version = ?", id, expectedVersion).
		Delete(&model.Mall{})
	if result.Error != nil {
		return fmt.Errorf("mall: delete: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrMallVersionConflict
	}
	return nil
}

func normalizePageSize(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > maxMallPageSize {
		return maxMallPageSize
	}
	return limit
}

func sanitizeMallUpdates(updates map[string]interface{}) (map[string]interface{}, error) {
	allowed := map[string]struct{}{
		"name_cn": {}, "name_en": {}, "aliases_json": {}, "brand_name": {}, "group_name": {},
		"management_company": {}, "business_status": {}, "opening_date": {}, "renovation_date": {},
		"mall_type": {}, "positioning": {}, "tags_json": {}, "country": {}, "province": {},
		"city": {}, "district": {}, "township": {}, "street": {}, "street_number": {},
		"postal_code": {}, "address_raw": {}, "address_standardized": {}, "adcode": {}, "citycode": {},
		"longitude": {}, "latitude": {}, "coordinate_system": {}, "weather_longitude": {},
		"weather_latitude": {}, "weather_coordinate_system": {}, "geocode_level": {},
		"geocode_confidence": {}, "geocode_status": {}, "geocoded_at": {}, "geocode_confirmed_by": {},
		"timezone": {}, "gross_floor_area_sqm": {}, "retail_area_sqm": {}, "floor_count_above": {},
		"floor_count_below": {}, "store_count": {}, "anchor_store_count": {}, "parking_spaces": {},
		"ev_charging_spaces": {}, "business_hours_json": {}, "service_phone": {}, "website_url": {},
		"metro_lines_json": {}, "metro_stations_json": {}, "bus_stops_json": {}, "indoor_outdoor_type": {},
		"contact_name": {}, "contact_phone": {}, "contact_email": {}, "operator_department": {},
		"data_owner_user_id": {}, "source_type": {}, "source_reference": {}, "remark": {},
		"custom_fields_json": {}, "weather_enabled": {}, "weather_provider": {}, "coverage_radius_m": {},
		"sampling_mode": {}, "detail_profile": {}, "fast_refresh_minutes": {}, "retention_policy_code": {},
		"last_weather_success_at": {}, "last_weather_error_at": {}, "status": {}, "updated_by": {},
	}

	result := make(map[string]interface{}, len(updates)+1)
	for field, value := range updates {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("mall: update field %q is not allowed", field)
		}
		result[field] = value
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("mall: no fields to update")
	}
	return result, nil
}
