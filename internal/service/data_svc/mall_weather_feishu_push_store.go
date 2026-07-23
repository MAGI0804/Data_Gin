package data_svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const mallWeatherFeishuPushOperationScope = "weather.feishu.push"

var ErrMallWeatherFeishuDestinationConflict = errors.New("mall weather feishu push: destination changed")

type MallWeatherFeishuPushCreateResult struct {
	RunID          uint      `json:"runId"`
	TraceID        string    `json:"traceId"`
	Status         string    `json:"status"`
	DestinationID  uint      `json:"destinationId"`
	ProfileID      uint      `json:"profileId"`
	ProfileVersion uint64    `json:"profileVersion"`
	EstimatedRows  int64     `json:"estimatedRows"`
	CreatedBy      uint      `json:"createdBy"`
	CreatedAt      time.Time `json:"createdAt"`
}

type mallWeatherFeishuPushCreateCommand struct {
	ActorUserID             uint
	DestinationID           uint
	DestinationCode         string
	DestinationConfigJSON   string
	ProfileID               uint
	ProfileVersion          uint64
	ProfileCode             string
	ProfileName             string
	ProfileJSON             model.JSONText
	ProfileSnapshotJSON     model.JSONText
	FiltersJSON             model.JSONText
	DestinationSnapshotJSON model.JSONText
	KeyHash                 string
	RequestHash             string
	TraceID                 string
	EstimatedRows           int64
	RequestedAt             time.Time
}

type mallWeatherFeishuPushStore interface {
	Create(context.Context, mallWeatherFeishuPushCreateCommand) (*MallWeatherFeishuPushCreateResult, bool, error)
}

type gormMallWeatherFeishuPushStore struct {
	db *gorm.DB
}

func (store gormMallWeatherFeishuPushStore) Create(
	ctx context.Context,
	command mallWeatherFeishuPushCreateCommand,
) (*MallWeatherFeishuPushCreateResult, bool, error) {
	if store.db == nil || ctx == nil || !validMallWeatherFeishuPushCreateCommand(command) {
		return nil, false, fmt.Errorf("mall weather feishu push: invalid store command")
	}
	var result *MallWeatherFeishuPushCreateResult
	var replayed bool
	err := store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		idempotencyDAO := data_dao.NewAPIIdempotencyDAO(tx)
		reservation := &model.APIIdempotencyRecord{
			OperationScope: mallWeatherFeishuPushOperationScope,
			ActorUserID:    command.ActorUserID,
			KeyHash:        command.KeyHash,
			RequestHash:    command.RequestHash,
			ResourceType:   "weather_feishu_pipeline_run",
			ResponseJSON:   model.JSONText(`{}`),
		}
		reserved, err := idempotencyDAO.Reserve(ctx, reservation)
		if err != nil {
			return err
		}
		if !reserved {
			existing, err := idempotencyDAO.FindForUpdate(
				ctx,
				mallWeatherFeishuPushOperationScope,
				command.ActorUserID,
				command.KeyHash,
			)
			if err != nil {
				return err
			}
			if existing.RequestHash != command.RequestHash {
				return ErrMallIdempotencyConflict
			}
			if existing.ResourceID == 0 || existing.HTTPStatus == 0 ||
				existing.ResponseJSON == "" || existing.ResponseJSON == model.JSONText(`{}`) {
				return ErrMallIdempotencyPending
			}
			replayedResult, err := decodeMallWeatherFeishuPushCreateResult(existing.ResponseJSON)
			if err != nil {
				return err
			}
			if replayedResult.RunID != existing.ResourceID {
				return fmt.Errorf("mall weather feishu push: idempotency resource mismatch")
			}
			result, replayed = replayedResult, true
			return nil
		}

		if err := lockMallWeatherFeishuDestination(ctx, tx, command); err != nil {
			return err
		}
		if err := lockMallWeatherFeishuProfile(ctx, tx, command); err != nil {
			return err
		}

		record := &data_dao.MallWeatherFeishuRunRecord{
			Pipeline: model.PipelineRun{
				TraceID:       command.TraceID,
				RunType:       "delivery",
				TriggerType:   "api",
				DestinationID: command.DestinationID,
				Status:        "running",
			},
			Detail: model.MallWeatherFeishuRun{
				ProfileID:               command.ProfileID,
				ProfileVersion:          command.ProfileVersion,
				ProfileSnapshotJSON:     command.ProfileSnapshotJSON,
				FiltersJSON:             command.FiltersJSON,
				DestinationSnapshotJSON: command.DestinationSnapshotJSON,
				CreatedBy:               command.ActorUserID,
			},
		}
		if err := data_dao.NewMallWeatherFeishuRunDAO(tx).Create(ctx, record); err != nil {
			return fmt.Errorf("mall weather feishu push: create run: %w", err)
		}
		outbox, err := newMallWeatherFeishuOutbox(record.Pipeline.ID, command.TraceID, command.RequestedAt)
		if err != nil {
			return err
		}
		if err := data_dao.NewAsyncJobOutboxDAO(tx).Create(ctx, &outbox); err != nil {
			return fmt.Errorf("mall weather feishu push: create outbox: %w", err)
		}
		created := &MallWeatherFeishuPushCreateResult{
			RunID:          record.Pipeline.ID,
			TraceID:        record.Pipeline.TraceID,
			Status:         "PENDING",
			DestinationID:  record.Pipeline.DestinationID,
			ProfileID:      record.Detail.ProfileID,
			ProfileVersion: record.Detail.ProfileVersion,
			EstimatedRows:  command.EstimatedRows,
			CreatedBy:      record.Detail.CreatedBy,
			CreatedAt:      record.Detail.CreatedAt.UTC(),
		}
		responseJSON, err := json.Marshal(created)
		if err != nil {
			return fmt.Errorf("mall weather feishu push: encode response: %w", err)
		}
		if err := idempotencyDAO.Complete(
			ctx,
			reservation.ID,
			record.Pipeline.ID,
			http.StatusAccepted,
			model.JSONText(responseJSON),
		); err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return result, replayed, nil
}

func lockMallWeatherFeishuDestination(
	ctx context.Context,
	tx *gorm.DB,
	command mallWeatherFeishuPushCreateCommand,
) error {
	var current model.DestinationDefinition
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ?", command.DestinationID).
		First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrMallWeatherFeishuDestinationNotFound
	}
	if err != nil {
		return fmt.Errorf("mall weather feishu push: lock destination: %w", err)
	}
	if !current.Enabled || current.DestinationType != mallWeatherFeishuDestinationType ||
		current.Code != command.DestinationCode || current.ConfigJSON != command.DestinationConfigJSON {
		return ErrMallWeatherFeishuDestinationConflict
	}
	return nil
}

func lockMallWeatherFeishuProfile(
	ctx context.Context,
	tx *gorm.DB,
	command mallWeatherFeishuPushCreateCommand,
) error {
	var current model.MallWeatherExportProfile
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ?", command.ProfileID).
		First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return data_dao.ErrMallWeatherExportProfileNotFound
	}
	if err != nil {
		return fmt.Errorf("mall weather feishu push: lock profile: %w", err)
	}
	if !current.Enabled || current.Version != command.ProfileVersion || current.Code != command.ProfileCode ||
		current.Name != command.ProfileName || current.ProfileJSON != command.ProfileJSON {
		return ErrMallWeatherExportProfileConflict
	}
	return nil
}

func validMallWeatherFeishuPushCreateCommand(command mallWeatherFeishuPushCreateCommand) bool {
	return command.ActorUserID > 0 && command.DestinationID > 0 && command.DestinationCode != "" &&
		validMallWeatherFeishuPushJSON(command.DestinationConfigJSON, 64*1024) &&
		command.ProfileID > 0 && command.ProfileVersion > 0 && command.ProfileCode != "" && command.ProfileName != "" &&
		validMallWeatherFeishuPushJSON(string(command.ProfileJSON), 256*1024) &&
		validMallWeatherFeishuPushJSON(string(command.ProfileSnapshotJSON), 256*1024) &&
		validMallWeatherFeishuPushJSON(string(command.FiltersJSON), 64*1024) &&
		validMallWeatherFeishuPushJSON(string(command.DestinationSnapshotJSON), 64*1024) &&
		len(command.KeyHash) == 64 && len(command.RequestHash) == 64 &&
		len(command.TraceID) == 36 && uuid.Validate(command.TraceID) == nil &&
		command.EstimatedRows >= 0 && !command.RequestedAt.IsZero()
}

func validMallWeatherFeishuPushJSON(value string, maxBytes int) bool {
	trimmed := strings.TrimSpace(value)
	return len(trimmed) >= 2 && len(trimmed) <= maxBytes && strings.HasPrefix(trimmed, "{") &&
		strings.HasSuffix(trimmed, "}") && json.Valid([]byte(trimmed))
}

func decodeMallWeatherFeishuPushCreateResult(
	value model.JSONText,
) (*MallWeatherFeishuPushCreateResult, error) {
	var result MallWeatherFeishuPushCreateResult
	decoder := json.NewDecoder(strings.NewReader(string(value)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("mall weather feishu push: decode idempotency response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("mall weather feishu push: decode idempotency response: trailing data")
	}
	if result.RunID == 0 || len(result.TraceID) != 36 || uuid.Validate(result.TraceID) != nil ||
		result.Status != "PENDING" || result.DestinationID == 0 || result.ProfileID == 0 ||
		result.ProfileVersion == 0 || result.EstimatedRows < 0 || result.CreatedBy == 0 || result.CreatedAt.IsZero() {
		return nil, fmt.Errorf("mall weather feishu push: invalid idempotency response")
	}
	result.CreatedAt = result.CreatedAt.UTC()
	return &result, nil
}
