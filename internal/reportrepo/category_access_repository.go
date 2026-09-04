package reportrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCategoryAccessNotFound = errors.New("report category access: not found")
	ErrCategoryAccessConflict = errors.New("report category access: version conflict")
)

type CategoryAccess struct {
	Policy      model.ReportCategoryAccess
	ReportCount int64
	Grants      []model.ReportCategoryGrant
}

type categoryAccessRecord struct {
	Category         string `gorm:"column:category"`
	ReportCount      int64  `gorm:"column:report_count"`
	CategoryAccessID uint   `gorm:"column:category_access_id"`
	LockVersion      uint64 `gorm:"column:lock_version"`
}

func (repository *Repository) ListCategoryAccess(ctx context.Context, actor uint) ([]CategoryAccess, error) {
	if err := repository.validate(ctx, actor); err != nil {
		return nil, err
	}
	var records []categoryAccessRecord
	err := repository.db.WithContext(ctx).Table("report_definitions AS definitions").
		Select(`definitions.category, COUNT(definitions.id) AS report_count,
			COALESCE(category_access.id, 0) AS category_access_id,
			COALESCE(category_access.lock_version, 0) AS lock_version`).
		Joins("LEFT JOIN report_category_access AS category_access ON category_access.category = definitions.category").
		Where("definitions.category <> ''").
		Group("definitions.category, category_access.id, category_access.lock_version").
		Order("definitions.category ASC").
		Scan(&records).Error
	if err != nil {
		return nil, fmt.Errorf("report category access: list categories: %w", err)
	}

	accessIDs := make([]uint, 0, len(records))
	for _, record := range records {
		if record.CategoryAccessID > 0 {
			accessIDs = append(accessIDs, record.CategoryAccessID)
		}
	}
	grantsByAccessID := make(map[uint][]model.ReportCategoryGrant, len(accessIDs))
	if len(accessIDs) > 0 {
		var grants []model.ReportCategoryGrant
		if err := repository.db.WithContext(ctx).Where("category_access_id IN ?", accessIDs).
			Order("category_access_id ASC, subject_type ASC, subject_id ASC").Find(&grants).Error; err != nil {
			return nil, fmt.Errorf("report category access: list grants: %w", err)
		}
		for _, grant := range grants {
			grantsByAccessID[grant.CategoryAccessID] = append(grantsByAccessID[grant.CategoryAccessID], grant)
		}
	}

	result := make([]CategoryAccess, 0, len(records))
	for _, record := range records {
		result = append(result, CategoryAccess{
			Policy: model.ReportCategoryAccess{
				BaseModel:   model.BaseModel{ID: record.CategoryAccessID},
				Category:    record.Category,
				LockVersion: record.LockVersion,
			},
			ReportCount: record.ReportCount,
			Grants:      grantsByAccessID[record.CategoryAccessID],
		})
	}
	return result, nil
}

func (repository *Repository) ReplaceCategoryAccess(
	ctx context.Context,
	actor uint,
	category string,
	expectedLockVersion uint64,
	grants []model.ReportCategoryGrant,
) (*CategoryAccess, error) {
	if err := repository.validate(ctx, actor); err != nil {
		return nil, err
	}
	category = strings.TrimSpace(category)
	if category == "" {
		return nil, invalidDraft("report category is required")
	}

	var saved CategoryAccess
	err := repository.transact(ctx, repository.db, func(tx *gorm.DB) error {
		var reportCount int64
		if err := tx.WithContext(ctx).Model(&model.ReportDefinition{}).Where("category = ?", category).Count(&reportCount).Error; err != nil {
			return fmt.Errorf("report category access: count reports: %w", err)
		}
		if reportCount == 0 {
			return ErrCategoryAccessNotFound
		}
		referenceGrants := make([]model.ReportGrant, 0, len(grants))
		for _, grant := range grants {
			referenceGrants = append(referenceGrants, model.ReportGrant{
				SubjectType: grant.SubjectType,
				SubjectID:   grant.SubjectID,
				ActionsJSON: grant.ActionsJSON,
			})
		}
		if err := validateGrantReferences(ctx, tx, referenceGrants); err != nil {
			return err
		}

		policy, err := lockCategoryAccess(ctx, tx, category)
		if err != nil {
			return err
		}
		if policy.ID == 0 {
			if expectedLockVersion != 0 {
				return ErrCategoryAccessConflict
			}
			policy = model.ReportCategoryAccess{Category: category, LockVersion: 1, UpdatedBy: actor}
			if err := tx.WithContext(ctx).Create(&policy).Error; err != nil {
				return fmt.Errorf("report category access: create policy: %w", err)
			}
		} else {
			if expectedLockVersion == 0 || policy.LockVersion != expectedLockVersion {
				return ErrCategoryAccessConflict
			}
			policy.LockVersion++
			policy.UpdatedBy = actor
			if err := tx.WithContext(ctx).Model(&model.ReportCategoryAccess{}).Where("id = ?", policy.ID).
				Updates(map[string]interface{}{"lock_version": policy.LockVersion, "updated_by": actor}).Error; err != nil {
				return fmt.Errorf("report category access: update policy: %w", err)
			}
		}

		if err := tx.WithContext(ctx).Where("category_access_id = ?", policy.ID).Delete(&model.ReportCategoryGrant{}).Error; err != nil {
			return fmt.Errorf("report category access: delete grants: %w", err)
		}
		rows := make([]model.ReportCategoryGrant, 0, len(grants))
		for _, grant := range grants {
			grant.ID = 0
			grant.CategoryAccessID = policy.ID
			grant.CreatedBy = actor
			grant.UpdatedBy = actor
			rows = append(rows, grant)
		}
		if len(rows) > 0 {
			if err := tx.WithContext(ctx).Create(&rows).Error; err != nil {
				return fmt.Errorf("report category access: create grants: %w", err)
			}
		}
		if err := writeCategoryAccessAudit(ctx, tx, actor, policy.ID, category, policy.LockVersion, len(rows)); err != nil {
			return err
		}
		saved = CategoryAccess{Policy: policy, ReportCount: reportCount, Grants: rows}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func lockCategoryAccess(ctx context.Context, tx *gorm.DB, category string) (model.ReportCategoryAccess, error) {
	var policy model.ReportCategoryAccess
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("category = ?", category).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ReportCategoryAccess{}, nil
	}
	if err != nil {
		return model.ReportCategoryAccess{}, fmt.Errorf("report category access: lock policy: %w", err)
	}
	return policy, nil
}

func writeCategoryAccessAudit(ctx context.Context, tx *gorm.DB, actor, targetID uint, category string, lockVersion uint64, grantCount int) error {
	detail, err := json.Marshal(map[string]interface{}{
		"category":    category,
		"lockVersion": lockVersion,
		"grantCount":  grantCount,
	})
	if err != nil {
		return fmt.Errorf("report category access: encode audit: %w", err)
	}
	return createReportAudit(ctx, tx, model.ReportAudit{
		ActorUserID: actor,
		Action:      "REPORT_CATEGORY_ACCESS_REPLACE",
		TargetType:  "REPORT_CATEGORY",
		TargetID:    targetID,
		RequestID:   uuid.NewString(),
		DetailJSON:  model.JSONText(detail),
	})
}

func loadCategoryReportGrants(ctx context.Context, db *gorm.DB, category string) (bool, []model.ReportGrant, error) {
	var policy model.ReportCategoryAccess
	err := db.WithContext(ctx).Where("category = ?", category).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, []model.ReportGrant{}, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("report run: load category access: %w", err)
	}
	var rows []model.ReportCategoryGrant
	if err := db.WithContext(ctx).Where("category_access_id = ?", policy.ID).
		Order("subject_type ASC, subject_id ASC").Find(&rows).Error; err != nil {
		return false, nil, fmt.Errorf("report run: load category grants: %w", err)
	}
	grants := make([]model.ReportGrant, 0, len(rows))
	for _, row := range rows {
		grants = append(grants, model.ReportGrant{
			SubjectType: row.SubjectType,
			SubjectID:   row.SubjectID,
			ActionsJSON: row.ActionsJSON,
		})
	}
	return true, grants, nil
}
