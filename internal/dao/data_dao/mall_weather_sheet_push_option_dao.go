package data_dao

import (
	"context"
	"fmt"
	"strings"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

const maxMallWeatherSheetPushOptionRows = 201

type MallWeatherSheetPushOptionDAO struct {
	db *gorm.DB
}

func NewMallWeatherSheetPushOptionDAO(databases ...*gorm.DB) *MallWeatherSheetPushOptionDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallWeatherSheetPushOptionDAO{db: db}
}

func (dao *MallWeatherSheetPushOptionDAO) ListEnabledDestinations(
	ctx context.Context,
	destinationType string,
	limit int,
) ([]model.DestinationDefinition, error) {
	destinationType = strings.TrimSpace(destinationType)
	if dao == nil || dao.db == nil || ctx == nil || destinationType == "" ||
		limit < 1 || limit > maxMallWeatherSheetPushOptionRows {
		return nil, fmt.Errorf("mall weather sheet push options: invalid destination list")
	}
	rows := make([]model.DestinationDefinition, 0)
	err := dao.db.WithContext(ctx).
		Select("id", "name", "code", "destination_type", "config_json", "enabled").
		Where("enabled = ? AND destination_type = ?", true, destinationType).
		Order("id ASC").
		Limit(limit).
		Find(&rows).
		Error
	if err != nil {
		return nil, fmt.Errorf("mall weather sheet push options: list destinations: %w", err)
	}
	return rows, nil
}

func (dao *MallWeatherSheetPushOptionDAO) ListEnabledProfilesByCodes(
	ctx context.Context,
	codes []string,
) ([]model.MallWeatherExportProfile, error) {
	if dao == nil || dao.db == nil || ctx == nil || len(codes) == 0 ||
		len(codes) > maxMallWeatherSheetPushOptionRows {
		return nil, fmt.Errorf("mall weather sheet push options: invalid profile list")
	}
	rows := make([]model.MallWeatherExportProfile, 0, len(codes))
	err := dao.db.WithContext(ctx).
		Select("id", "code", "version", "enabled").
		Where("enabled = ? AND code IN ?", true, codes).
		Order("code ASC").
		Find(&rows).
		Error
	if err != nil {
		return nil, fmt.Errorf("mall weather sheet push options: list profiles: %w", err)
	}
	return rows, nil
}
