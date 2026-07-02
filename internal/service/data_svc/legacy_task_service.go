package data_svc

import (
	"context"
	"fmt"

	"gin-biz-web-api/global"
	"gin-biz-web-api/job"
	jobClient "gin-biz-web-api/pkg/job"

	"github.com/hibiken/asynq"
)

type LegacyTaskRunResult struct {
	ID    string `json:"id"`
	Queue string `json:"queue"`
	Type  string `json:"type"`
}

type LegacyTaskService struct{}

func NewLegacyTaskService() *LegacyTaskService {
	return &LegacyTaskService{}
}

func (s *LegacyTaskService) ListDefinitions(ctx context.Context) []job.LegacyTaskDefinition {
	_ = ctx
	return job.LegacyTaskDefinitions()
}

func (s *LegacyTaskService) Enqueue(ctx context.Context, code string, payload map[string]interface{}) (*LegacyTaskRunResult, error) {
	_ = ctx

	task, err := job.NewLegacyTask(code, payload)
	if err != nil {
		return nil, err
	}

	client := global.QueueJobClient
	if client == nil {
		client = jobClient.Client
	}
	if client == nil {
		return nil, fmt.Errorf("queue job client is not initialized")
	}

	info, err := client.Enqueue(task, asynq.MaxRetry(3))
	if err != nil {
		return nil, err
	}

	return &LegacyTaskRunResult{
		ID:    info.ID,
		Queue: info.Queue,
		Type:  task.Type(),
	}, nil
}
