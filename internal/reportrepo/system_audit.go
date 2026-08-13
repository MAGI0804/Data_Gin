package reportrepo

import (
	"context"
	"encoding/json"
	"fmt"

	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func writeSystemReportAudit(ctx context.Context, tx *gorm.DB, action, targetType string, targetID uint, detail map[string]interface{}) error {
	if detail == nil {
		detail = map[string]interface{}{}
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("report system audit: encode detail: %w", err)
	}
	return createReportAudit(ctx, tx, model.ReportAudit{
		ActorType: model.ReportAuditActorSystem, ActorUserID: 0, Action: action,
		TargetType: targetType, TargetID: targetID, RequestID: uuid.NewString(), DetailJSON: model.JSONText(encoded),
	})
}
