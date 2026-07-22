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
	ErrMallGeocodeRunNotFound       = errors.New("mall geocode: run not found")
	ErrMallGeocodeCandidateNotFound = errors.New("mall geocode: candidate not found")
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

func (dao *MallGeocodeDAO) FindLatestRun(ctx context.Context, mallID uint) (*model.MallGeocodeRun, error) {
	return dao.findLatestRun(ctx, mallID, false)
}

func (dao *MallGeocodeDAO) FindLatestRunForUpdate(ctx context.Context, mallID uint) (*model.MallGeocodeRun, error) {
	return dao.findLatestRun(ctx, mallID, true)
}

func (dao *MallGeocodeDAO) findLatestRun(ctx context.Context, mallID uint, forUpdate bool) (*model.MallGeocodeRun, error) {
	var run model.MallGeocodeRun
	query := dao.db.WithContext(ctx).Where("mall_id = ?", mallID).Order("id DESC")
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMallGeocodeRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mall geocode: find latest run: %w", err)
	}
	return &run, nil
}

func (dao *MallGeocodeDAO) FindCandidateForUpdate(ctx context.Context, mallID, candidateID uint) (*model.MallGeocodeCandidate, error) {
	var candidate model.MallGeocodeCandidate
	err := dao.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND mall_id = ?", candidateID, mallID).
		First(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMallGeocodeCandidateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mall geocode: find candidate: %w", err)
	}
	return &candidate, nil
}

func (dao *MallGeocodeDAO) MarkSelected(ctx context.Context, runID, candidateID uint) error {
	return dao.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return NewMallGeocodeDAO(tx).markSelectedForMall(ctx, 0, runID, candidateID, "success")
	})
}

// MarkSelectedForMall performs the selection updates on the DAO's current DB.
// Callers that require atomicity must supply an existing transaction.
func (dao *MallGeocodeDAO) MarkSelectedForMall(ctx context.Context, mallID, runID, candidateID uint) error {
	return dao.markSelectedForMall(ctx, mallID, runID, candidateID, "manually_confirmed")
}

func (dao *MallGeocodeDAO) markSelectedForMall(ctx context.Context, mallID, runID, candidateID uint, runStatus string) error {
	candidateScope := dao.db.WithContext(ctx).Model(&model.MallGeocodeCandidate{}).Where("run_id = ?", runID)
	if mallID != 0 {
		candidateScope = candidateScope.Where("mall_id = ?", mallID)
	}
	if err := candidateScope.Update("is_selected", false).Error; err != nil {
		return fmt.Errorf("mall geocode: clear selected candidate: %w", err)
	}
	selectedScope := dao.db.WithContext(ctx).Model(&model.MallGeocodeCandidate{}).
		Where("id = ? AND run_id = ?", candidateID, runID)
	if mallID != 0 {
		selectedScope = selectedScope.Where("mall_id = ?", mallID)
	}
	result := selectedScope.Update("is_selected", true)
	if result.Error != nil {
		return fmt.Errorf("mall geocode: select candidate: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrMallGeocodeCandidateNotFound
	}
	runScope := dao.db.WithContext(ctx).Model(&model.MallGeocodeRun{}).Where("id = ?", runID)
	if mallID != 0 {
		runScope = runScope.Where("mall_id = ?", mallID)
	}
	result = runScope.Updates(map[string]interface{}{
		"selected_candidate_id": candidateID,
		"status":                runStatus,
	})
	if result.Error != nil {
		return fmt.Errorf("mall geocode: update selected run: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrMallGeocodeRunNotFound
	}
	return nil
}

func (dao *MallGeocodeDAO) CreateCoordinateAudit(ctx context.Context, audit *model.MallCoordinateAudit) error {
	if audit == nil || audit.MallID == 0 || audit.ConfirmedBy == 0 || audit.MallVersionBefore == 0 || audit.MallVersionAfter == 0 {
		return fmt.Errorf("mall geocode: invalid coordinate audit")
	}
	if err := dao.db.WithContext(ctx).Create(audit).Error; err != nil {
		return fmt.Errorf("mall geocode: create coordinate audit: %w", err)
	}
	return nil
}
