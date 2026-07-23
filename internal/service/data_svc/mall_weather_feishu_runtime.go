package data_svc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/dao/data_dao"
	weatherdomain "gin-biz-web-api/internal/weather"
	"gin-biz-web-api/pkg/database"
	projectredis "gin-biz-web-api/pkg/redis"

	"github.com/google/uuid"
)

type mallWeatherFeishuRuntimeSheets interface {
	mallWeatherFeishuExecutionSheets
	AppendValues(context.Context, string, feishu.SheetWriteRange) (*feishu.SheetWriteResult, error)
}

func NewMallWeatherFeishuProcessor() (*MallWeatherFeishuProcessor, error) {
	sheets, err := newRuntimeMallWeatherFeishuSheets()
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu runtime: sheets client: %w", err)
	}
	redisInstance := projectredis.Instance()
	if redisInstance == nil || redisInstance.Client == nil {
		return nil, errors.New("mall weather feishu runtime: redis is unavailable")
	}
	locker, err := weatherdomain.NewRedisTaskLocker(
		redisInstance.Client,
		projectredis.GenNamespace("lock:mall_weather_feishu:"),
		defaultMallWeatherFeishuRunStaleAfter,
	)
	if err != nil {
		return nil, fmt.Errorf("mall weather feishu runtime: destination locker: %w", err)
	}
	now := time.Now
	pager := data_dao.NewMallWeatherExportDataDAO(database.DB)
	deliveryLogs := data_dao.NewDeliveryLogDAO(database.DB)
	mappings := data_dao.NewMallWeatherSheetRowDAO(database.DB)
	appendBatches, err := newMallWeatherFeishuAppendBatchExecutor(sheets, deliveryLogs, now)
	if err != nil {
		return nil, err
	}
	overwriteBatches, err := newMallWeatherFeishuOverwriteBatchExecutor(sheets, deliveryLogs, now)
	if err != nil {
		return nil, err
	}
	upsertBatches, err := newMallWeatherFeishuUpsertBatchExecutor(sheets, deliveryLogs, now)
	if err != nil {
		return nil, err
	}
	appendDatasets, err := newMallWeatherFeishuAppendDatasetRunner(
		pager,
		sheets,
		deliveryLogs,
		appendBatches,
		now,
	)
	if err != nil {
		return nil, err
	}
	overwriteDatasets, err := newMallWeatherFeishuOverwriteDatasetRunner(
		pager,
		sheets,
		deliveryLogs,
		overwriteBatches,
		now,
	)
	if err != nil {
		return nil, err
	}
	overwriteCleanup, err := newMallWeatherFeishuOverwriteCleanupRunner(
		sheets,
		deliveryLogs,
		overwriteBatches,
		now,
	)
	if err != nil {
		return nil, err
	}
	upsertScanner, err := newMallWeatherFeishuUpsertScanner(sheets, mappings, now)
	if err != nil {
		return nil, err
	}
	upsertDatasets, err := newMallWeatherFeishuUpsertDatasetRunner(
		pager,
		upsertScanner,
		mappings,
		upsertBatches,
		now,
	)
	if err != nil {
		return nil, err
	}
	executor, err := newMallWeatherFeishuExecutor(
		global.Credentials,
		sheets,
		locker,
		appendDatasets,
		overwriteDatasets,
		upsertDatasets,
		overwriteCleanup,
		defaultMallWeatherFeishuStateUpdateTimeout,
	)
	if err != nil {
		return nil, err
	}
	return newMallWeatherFeishuProcessor(
		data_dao.NewMallWeatherFeishuRunDAO(database.DB),
		executor,
		now,
		uuid.NewString,
		noopMallWeatherMetricRecorder{},
		defaultMallWeatherFeishuRunStaleAfter,
		defaultMallWeatherFeishuHeartbeatInterval,
		defaultMallWeatherFeishuStateUpdateTimeout,
	)
}
