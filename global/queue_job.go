package global

import (
	"github.com/hibiken/asynq"
)

// QueueJobClient 异步队列客户端实例
var QueueJobClient *asynq.Client

// QueueJobServer 异步队列服务端实例
var QueueJobServer *asynq.Server

// QueueJobScheduler 定时任务调度器实例
var QueueJobScheduler *asynq.Scheduler
