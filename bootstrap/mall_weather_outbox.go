package bootstrap

import (
	"context"
	"errors"
	"sync"
	"time"

	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/service/data_svc"
	"gin-biz-web-api/job"
	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/console"
	"gin-biz-web-api/pkg/database"
	"gin-biz-web-api/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var mallWeatherOutboxLifecycle struct {
	sync.Mutex
	cancel context.CancelFunc
	done   <-chan struct{}
}

func startMallWeatherOutboxDispatcher() {
	if !config.GetBool("cfg.mall_weather.enabled") {
		return
	}
	if database.DB == nil {
		console.Warning("Mall weather outbox dispatcher was not started: database is unavailable")
		return
	}
	if global.QueueJobClient == nil {
		console.Warning("Mall weather outbox dispatcher was not started: queue client is unavailable")
		return
	}

	maxAttempts := config.GetInt("cfg.mall_weather.max_attempts")
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	dispatcher, err := job.NewOutboxDispatcher(
		data_dao.NewAsyncJobOutboxDAO(database.DB),
		job.NewAsynqMallWeatherTaskPublisher(global.QueueJobClient),
		job.OutboxDispatcherConfig{
			WorkerID:     "mall-weather-outbox-" + uuid.NewString(),
			PollInterval: time.Duration(config.GetInt("cfg.mall_weather.outbox_poll_interval_ms")) * time.Millisecond,
			LockTimeout:  time.Duration(config.GetInt("cfg.mall_weather.outbox_lock_timeout_seconds")) * time.Second,
			BatchSize:    config.GetInt("cfg.mall_weather.outbox_batch_size"),
			RetryBase:    time.Duration(config.GetInt("cfg.mall_weather.outbox_retry_base_seconds")) * time.Second,
			RetryMax:     time.Duration(config.GetInt("cfg.mall_weather.outbox_retry_max_seconds")) * time.Second,
			MaxRetry:     maxAttempts - 1,
			TaskTimeout:  time.Duration(config.GetInt("cfg.mall_weather.task_timeout_seconds")) * time.Second,
			OnPublished:  data_svc.RecordMallWeatherOutboxQueueLag,
			OnCycleError: func(err error) {
				logger.Error("Mall weather outbox dispatch cycle failed", zap.Error(err))
			},
		},
	)
	if err != nil {
		console.Warning("Mall weather outbox dispatcher was not started: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	mallWeatherOutboxLifecycle.Lock()
	if mallWeatherOutboxLifecycle.cancel != nil {
		mallWeatherOutboxLifecycle.Unlock()
		cancel()
		console.Warning("Mall weather outbox dispatcher is already running")
		return
	}
	mallWeatherOutboxLifecycle.cancel = cancel
	mallWeatherOutboxLifecycle.done = done
	mallWeatherOutboxLifecycle.Unlock()

	go func() {
		defer close(done)
		if err := dispatcher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("Mall weather outbox dispatcher stopped", zap.Error(err))
		}
	}()
	console.Success("Mall weather outbox dispatcher started successfully")
}

func stopMallWeatherOutboxDispatcher() {
	mallWeatherOutboxLifecycle.Lock()
	cancel := mallWeatherOutboxLifecycle.cancel
	done := mallWeatherOutboxLifecycle.done
	mallWeatherOutboxLifecycle.cancel = nil
	mallWeatherOutboxLifecycle.done = nil
	mallWeatherOutboxLifecycle.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
		console.Info("Mall weather outbox dispatcher stopped")
	case <-time.After(5 * time.Second):
		console.Warning("Timed out waiting for mall weather outbox dispatcher to stop")
	}
}
