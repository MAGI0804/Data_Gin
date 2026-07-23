package data_svc

import (
	"context"
	"fmt"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
)

type MallWeatherFeishuPushRunDTO struct {
	RunID          uint       `json:"runId"`
	TraceID        string     `json:"traceId"`
	Status         string     `json:"status"`
	DestinationID  uint       `json:"destinationId"`
	ProfileID      uint       `json:"profileId"`
	ProfileVersion uint64     `json:"profileVersion"`
	TotalCount     int        `json:"totalCount"`
	SuccessCount   int        `json:"successCount"`
	FailedCount    int        `json:"failedCount"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
	CreatedBy      uint       `json:"createdBy"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (service *MallWeatherFeishuPushService) Get(
	ctx context.Context,
	actorUserID uint,
	pipelineRunID uint,
) (*MallWeatherFeishuPushRunDTO, error) {
	if service == nil || ctx == nil || actorUserID == 0 || pipelineRunID == 0 {
		return nil, ErrMallWeatherFeishuInvalid
	}
	allowed, err := service.permissions.HasPermission(
		ctx,
		actorUserID,
		PermissionWeatherFeishuPush,
		service.now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu push: authorize query: %w", err)
	}
	if !allowed {
		return nil, ErrMallForbidden
	}
	record, err := service.runs.FindByPipelineRunID(ctx, pipelineRunID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Detail.CreatedBy != actorUserID {
		return nil, data_dao.ErrMallWeatherFeishuRunNotFound
	}
	return mallWeatherFeishuPushRunDTO(record)
}

func mallWeatherFeishuPushRunDTO(
	record *data_dao.MallWeatherFeishuRunRecord,
) (*MallWeatherFeishuPushRunDTO, error) {
	if record == nil || record.Pipeline.ID == 0 || record.Detail.PipelineRunID != record.Pipeline.ID ||
		record.Detail.ProfileID == 0 || record.Detail.ProfileVersion == 0 || record.Detail.CreatedBy == 0 ||
		record.Detail.CreatedAt.IsZero() || record.Detail.UpdatedAt.IsZero() {
		return nil, fmt.Errorf("mall weather feishu push: invalid stored run")
	}
	status, err := mallWeatherFeishuPushPublicStatus(record.Pipeline)
	if err != nil {
		return nil, err
	}
	result := &MallWeatherFeishuPushRunDTO{
		RunID: record.Pipeline.ID, TraceID: record.Pipeline.TraceID, Status: status,
		DestinationID: record.Pipeline.DestinationID, ProfileID: record.Detail.ProfileID,
		ProfileVersion: record.Detail.ProfileVersion, TotalCount: record.Pipeline.TotalCount,
		SuccessCount: record.Pipeline.SuccessCount, FailedCount: record.Pipeline.FailedCount,
		ErrorMessage: record.Pipeline.ErrorMessage, CreatedBy: record.Detail.CreatedBy,
		CreatedAt: record.Detail.CreatedAt.UTC(), UpdatedAt: record.Detail.UpdatedAt.UTC(),
	}
	result.StartedAt = mallWeatherFeishuPipelineTime(record.Pipeline.StartedAt)
	result.FinishedAt = mallWeatherFeishuPipelineTime(record.Pipeline.FinishedAt)
	return result, nil
}

func mallWeatherFeishuPushPublicStatus(run model.PipelineRun) (string, error) {
	switch run.Status {
	case "running":
		if run.StartedAt == nil {
			return "PENDING", nil
		}
		if run.StartedAt.IsZero() || run.FinishedAt != nil {
			return "", fmt.Errorf("mall weather feishu push: invalid running state")
		}
		return "RUNNING", nil
	case "success":
		return "SUCCESS", nil
	case "partial_success":
		return "PARTIAL_SUCCESS", nil
	case "failed":
		return "FAILED", nil
	default:
		return "", fmt.Errorf("mall weather feishu push: invalid run status")
	}
}

func mallWeatherFeishuPipelineTime(value *model.TimeNormal) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
