package data_dao

import (
	"context"
	"fmt"

	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"

	"gorm.io/gorm"
)

type MallGeocodeDAO struct {
	db *gorm.DB
}

func NewMallGeocodeDAO(databases ...*gorm.DB) *MallGeocodeDAO {
	db := database.DB
	if len(databases) > 0 && databases[0] != nil {
		db = databases[0]
	}
	return &MallGeocodeDAO{db: db}
}

func (dao *MallGeocodeDAO) WithDB(db *gorm.DB) *MallGeocodeDAO {
	return &MallGeocodeDAO{db: db}
}

func (dao *MallGeocodeDAO) CreateRun(ctx context.Context, run *model.MallGeocodeRun) error {
	if run == nil {
		return fmt.Errorf("mall geocode: create nil run")
	}
	return dao.db.WithContext(ctx).Create(run).Error
}

func (dao *MallGeocodeDAO) CreateCandidates(ctx context.Context, candidates []model.MallGeocodeCandidate) error {
	if len(candidates) == 0 {
		return nil
	}
	if err := dao.db.WithContext(ctx).CreateInBatches(&candidates, 100).Error; err != nil {
		return fmt.Errorf("mall geocode: create candidates: %w", err)
	}
	return nil
}

func (dao *MallGeocodeDAO) ListCandidates(ctx context.Context, mallID, runID uint) ([]model.MallGeocodeCandidate, error) {
	var candidates []model.MallGeocodeCandidate
	if err := dao.db.WithContext(ctx).
		Where("mall_id = ? AND run_id = ?", mallID, runID).
		Order("candidate_no ASC").
		Find(&candidates).Error; err != nil {
		return nil, fmt.Errorf("mall geocode: list candidates: %w", err)
	}
	return candidates, nil
}

func (dao *MallGeocodeDAO) MarkSelected(ctx context.Context, runID, candidateID uint) error {
	return dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.MallGeocodeCandidate{}).
			Where("run_id = ?", runID).
			Update("is_selected", false).Error; err != nil {
			return fmt.Errorf("mall geocode: clear selected candidate: %w", err)
		}
		result := tx.Model(&model.MallGeocodeCandidate{}).
			Where("id = ? AND run_id = ?", candidateID, runID).
			Update("is_selected", true)
		if result.Error != nil {
			return fmt.Errorf("mall geocode: select candidate: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("mall geocode: candidate not found")
		}
		return tx.Model(&model.MallGeocodeRun{}).
			Where("id = ?", runID).
			Updates(map[string]interface{}{"selected_candidate_id": candidateID, "status": "success"}).Error
	})
}
