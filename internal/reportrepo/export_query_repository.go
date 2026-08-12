package reportrepo

import (
	"context"
	"errors"
	"fmt"

	"gin-biz-web-api/model"

	"gorm.io/gorm"
)

func (repository *Repository) FindExportForActor(ctx context.Context, actor, exportID uint) (*model.ReportExport, error) {
	if repository == nil || repository.db == nil || ctx == nil || actor == 0 || exportID == 0 {
		return nil, fmt.Errorf("report export query: invalid request")
	}
	var export model.ReportExport
	err := repository.db.WithContext(ctx).Where("id = ? AND created_by = ?", exportID, actor).First(&export).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrReportExportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("report export query: find export: %w", err)
	}
	return &export, nil
}
