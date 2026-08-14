package data_svc

import (
	"context"
	"errors"
	"time"

	"gin-biz-web-api/internal/reportcontract"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
	"gin-biz-web-api/model"
)

type reportResultCleanupOracleFactory interface {
	Open(context.Context, reportrepo.ResultCleanupRuntime, string) (reportExportOracleSession, error)
}

type oracleReportResultCleanupFactory struct{}

func (oracleReportResultCleanupFactory) Open(ctx context.Context, runtime reportrepo.ResultCleanupRuntime, password string) (reportExportOracleSession, error) {
	queryCtx, cancel := reportOracleQueryContext(ctx, runtime.Datasource)
	defer cancel()
	adapter, err := reportoracle.Open(queryCtx, oracleConfigFromDatasource(runtime.Datasource, password))
	if err != nil {
		return nil, err
	}
	closeOnError := func(openErr error) (reportExportOracleSession, error) {
		return nil, errors.Join(openErr, adapter.Close())
	}
	configuredColumns := make([]string, 0, len(runtime.Columns))
	for _, column := range runtime.Columns {
		configuredColumns = append(configuredColumns, column.DatabaseColumn)
	}
	queryTimeout := time.Duration(runtime.Datasource.QueryTimeoutSeconds) * time.Second
	if queryTimeout <= 0 {
		queryTimeout = defaultReportPublicationInspectionTimeout
	}
	if runtime.Version.ExecutionMode == model.ReportExecutionModeRefCursor {
		if err := adapter.ValidateJSONSnapshotStore(queryCtx); err != nil {
			return closeOnError(err)
		}
		return &oracleReportExportSession{
			adapter: adapter, jsonSnapshot: true, columns: configuredColumns,
			runUUID: runtime.Run.RunUUID, queryTimeout: queryTimeout,
		}, nil
	}
	ref := reportoracle.ResultSnapshotRef{
		Table:       reportoracle.ResultTableRef{Owner: runtime.Version.ResultTableOwner, Name: runtime.Version.ResultTableName},
		RunIDColumn: runtime.Version.ResultRunIDColumn,
		RowIDColumn: runtime.Version.ResultRowIDColumn,
		Columns:     configuredColumns,
	}
	resultColumns, err := adapter.InspectResultTable(queryCtx, ref.Table)
	if err != nil {
		return closeOnError(err)
	}
	if err := reportcontract.VerifyRuntimeResultMetadata(
		[]byte(runtime.Version.CompiledSpecJSON), runtime.Run.ContractHash, runtime.Run.ResultSchemaHash, resultColumns,
	); err != nil {
		return closeOnError(err)
	}
	contract, err := adapter.InspectResultSnapshotContract(queryCtx, ref)
	if err != nil {
		return closeOnError(err)
	}
	purgePlan, err := reportoracle.BuildPurgePlan(contract)
	if err != nil {
		return closeOnError(err)
	}
	return &oracleReportExportSession{
		adapter: adapter, purgePlan: purgePlan, runUUID: runtime.Run.RunUUID, queryTimeout: queryTimeout,
	}, nil
}
