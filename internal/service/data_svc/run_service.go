package data_svc

import (
	"context"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
)

type RunService struct {
	pipelineRunDAO *data_dao.PipelineRunDAO
}

func NewRunService() *RunService {
	return &RunService{
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
	}
}

func (s *RunService) ListPipelineRuns(ctx context.Context, limit int) ([]model.PipelineRun, error) {
	return s.pipelineRunDAO.FindRecent(ctx, limit)
}

func (s *RunService) ListPipelineRunsPage(ctx context.Context, query data_dao.PipelineRunListQuery) (*data_dao.PipelineRunListPage, error) {
	return s.pipelineRunDAO.FindPage(ctx, query)
}
