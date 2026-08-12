package data_svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportrepo"
)

type reportExportOracleSession interface {
	Read(context.Context, []string, *int64, int) (reportoracle.ResultPage, error)
	Purge(context.Context, int) (int64, error)
	Close() error
}

type reportExportOracleSessionFactory interface {
	Open(context.Context, reportrepo.ExportRuntime, string) (reportExportOracleSession, error)
}

type oracleReportExportSession struct {
	adapter   *reportoracle.Adapter
	pagePlan  reportoracle.ResultPagePlan
	purgePlan reportoracle.PurgePlan
	runUUID   string
}

type oracleReportExportSessionFactory struct{}

func (oracleReportExportSessionFactory) Open(ctx context.Context, runtime reportrepo.ExportRuntime, password string) (reportExportOracleSession, error) {
	adapter, err := reportoracle.Open(ctx, oracleConfigFromDatasource(runtime.Datasource, password))
	if err != nil {
		return nil, err
	}
	closeOnError := func(openErr error) (reportExportOracleSession, error) {
		return nil, errors.Join(openErr, adapter.Close())
	}
	columns, err := frozenExportColumns(runtime.Export.FrozenColumnsJSON)
	if err != nil {
		return closeOnError(err)
	}
	databaseColumns := make([]string, len(columns))
	for index := range columns {
		databaseColumns[index] = columns[index].DatabaseColumn
	}
	ref := reportoracle.ResultSnapshotRef{
		Table:       reportoracle.ResultTableRef{Owner: runtime.Version.ResultTableOwner, Name: runtime.Version.ResultTableName},
		RunIDColumn: runtime.Version.ResultRunIDColumn, RowIDColumn: runtime.Version.ResultRowIDColumn, Columns: databaseColumns,
	}
	contract, err := adapter.InspectResultSnapshotContract(ctx, ref)
	if err != nil {
		return closeOnError(err)
	}
	pagePlan, err := reportoracle.BuildResultPagePlan(contract, databaseColumns)
	if err != nil {
		return closeOnError(err)
	}
	purgePlan, err := reportoracle.BuildPurgePlan(contract)
	if err != nil {
		return closeOnError(err)
	}
	return &oracleReportExportSession{adapter: adapter, pagePlan: pagePlan, purgePlan: purgePlan, runUUID: runtime.Run.RunUUID}, nil
}

func (session *oracleReportExportSession) Read(ctx context.Context, columns []string, after *int64, limit int) (reportoracle.ResultPage, error) {
	if session == nil || session.adapter == nil || ctx == nil {
		return reportoracle.ResultPage{}, fmt.Errorf("report export oracle: invalid read session")
	}
	planned := session.pagePlan.Columns()
	if len(columns) != len(planned) {
		return reportoracle.ResultPage{}, fmt.Errorf("report export oracle: column contract changed")
	}
	for index := range columns {
		if columns[index] != planned[index] {
			return reportoracle.ResultPage{}, fmt.Errorf("report export oracle: column contract changed")
		}
	}
	return session.adapter.ReadResultPage(ctx, session.pagePlan, session.runUUID, after, limit)
}

func (session *oracleReportExportSession) Purge(ctx context.Context, batchSize int) (deleted int64, resultErr error) {
	if session == nil || session.adapter == nil || ctx == nil {
		return 0, fmt.Errorf("report export oracle: invalid purge session")
	}
	tx, err := session.adapter.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() {
		if resultErr != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, fmt.Errorf("report export oracle: rollback purge: %w", rollbackErr))
			}
		}
	}()
	deleted, err = session.adapter.PurgeResultBatch(ctx, tx, session.purgePlan, session.runUUID, batchSize)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("report export oracle: commit purge batch: %w", err)
	}
	return deleted, nil
}

func (session *oracleReportExportSession) Close() error {
	if session == nil || session.adapter == nil {
		return nil
	}
	return session.adapter.Close()
}
