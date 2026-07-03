package crontab

import (
	"context"
	"sync/atomic"

	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/pkg/logger"

	"go.uber.org/zap"
)

type BojunOrderCrontab struct{}

var bojunOrderCronRunning atomic.Bool

func (b BojunOrderCrontab) Run() {
	if !bojunOrderCronRunning.CompareAndSwap(false, true) {
		logger.Warn("伯俊订单拉取仍在运行，跳过本次调度")
		return
	}
	defer bojunOrderCronRunning.Store(false)

	result, err := data_svc.NewBojunOrderService().SyncRecentOrders(context.Background())
	if err != nil {
		logger.Error("伯俊订单拉取失败", zap.Error(err))
		return
	}

	logger.Info(
		"伯俊订单拉取完成",
		zap.String("start_time", result.StartTime),
		zap.String("end_time", result.EndTime),
		zap.Int("fetch_pages", result.FetchPages),
		zap.Int("saved_count", result.SavedCount),
	)
}
