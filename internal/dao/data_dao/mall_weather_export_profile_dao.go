package data_dao

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrMallWeatherExportProfileNotFound = errors.New("mall weather export profile: not found")
	ErrMallWeatherExportProfileConflict = errors.New("mall weather export profile: version conflict")
)

const maxMallWeatherExportProfiles = 1_000

type MallWeatherExportProfileDAO struct {
	db *gorm.DB
}

func NewMallWeatherExportProfileDAO(databases ...*gorm.DB) *MallWeatherExportProfileDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallWeatherExportProfileDAO{db: db}
}

func (dao *MallWeatherExportProfileDAO) Save(ctx context.Context, row *model.MallWeatherExportProfile, expectedVersion *uint64) (bool, error) {
	if dao == nil || dao.db == nil || ctx == nil || row == nil || strings.TrimSpace(row.Code) == "" ||
		strings.TrimSpace(row.Name) == "" || row.ProfileJSON == "" || row.UpdatedBy == 0 {
		return false, fmt.Errorf("mall weather export profile: invalid save")
	}
	created := false
	err := dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.MallWeatherExportProfile
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", row.Code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if expectedVersion != nil && *expectedVersion != 0 {
				return ErrMallWeatherExportProfileConflict
			}
			row.Version = 1
			row.CreatedBy = row.UpdatedBy
			if err := tx.Create(row).Error; err != nil {
				if isMallWeatherExportProfileDuplicate(err) {
					return ErrMallWeatherExportProfileConflict
				}
				return fmt.Errorf("mall weather export profile: create: %w", err)
			}
			created = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("mall weather export profile: find for update: %w", err)
		}
		if expectedVersion == nil || *expectedVersion != existing.Version {
			return ErrMallWeatherExportProfileConflict
		}
		if existing.Version == ^uint64(0) {
			return fmt.Errorf("mall weather export profile: version exhausted")
		}
		nextVersion := existing.Version + 1
		result := tx.Model(&model.MallWeatherExportProfile{}).
			Where("id = ? AND version = ?", existing.ID, existing.Version).
			Updates(map[string]interface{}{
				"name": row.Name, "profile_json": row.ProfileJSON, "enabled": row.Enabled,
				"updated_by": row.UpdatedBy, "version": nextVersion,
			})
		if result.Error != nil {
			return fmt.Errorf("mall weather export profile: update: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrMallWeatherExportProfileConflict
		}
		if err := tx.First(row, existing.ID).Error; err != nil {
			return fmt.Errorf("mall weather export profile: reload updated profile: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

func (dao *MallWeatherExportProfileDAO) List(ctx context.Context, enabled *bool) ([]model.MallWeatherExportProfile, error) {
	if dao == nil || dao.db == nil || ctx == nil {
		return nil, fmt.Errorf("mall weather export profile: invalid list")
	}
	query := dao.db.WithContext(ctx).Model(&model.MallWeatherExportProfile{})
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var rows []model.MallWeatherExportProfile
	if err := query.Order("code ASC").Limit(maxMallWeatherExportProfiles + 1).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("mall weather export profile: list: %w", err)
	}
	if len(rows) > maxMallWeatherExportProfiles {
		return nil, fmt.Errorf("mall weather export profile: list limit exceeded")
	}
	return rows, nil
}

func isMallWeatherExportProfileDuplicate(err error) bool {
	var mysqlError *mysqlDriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
