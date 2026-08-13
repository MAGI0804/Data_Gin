package data_svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gin-biz-web-api/internal/reportcontract"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/internal/reportquery"
	"gin-biz-web-api/internal/reportrepo"
)

type reportExportOracleSession interface {
	Read(context.Context, []string, *reportoracle.ResultCursor, int) (reportoracle.ResultPage, error)
	Purge(context.Context, int) (int64, error)
	Close() error
}

type reportExportOracleSessionFactory interface {
	Open(context.Context, reportrepo.ExportRuntime, string) (reportExportOracleSession, error)
}

type oracleReportExportSession struct {
	adapter      *reportoracle.Adapter
	pagePlan     reportoracle.ResultPagePlan
	purgePlan    reportoracle.PurgePlan
	runUUID      string
	queryTimeout time.Duration
}

type oracleReportExportSessionFactory struct{}

func (oracleReportExportSessionFactory) Open(ctx context.Context, runtime reportrepo.ExportRuntime, password string) (reportExportOracleSession, error) {
	queryCtx, cancel := reportOracleQueryContext(ctx, runtime.Datasource)
	defer cancel()
	adapter, err := reportoracle.Open(queryCtx, oracleConfigFromDatasource(runtime.Datasource, password))
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
	queryColumns, err := reportrepo.FrozenExportQueryColumns(runtime.Export.FrozenColumnsJSON)
	if err != nil {
		return closeOnError(err)
	}
	ref := reportoracle.ResultSnapshotRef{
		Table:       reportoracle.ResultTableRef{Owner: runtime.Version.ResultTableOwner, Name: runtime.Version.ResultTableName},
		RunIDColumn: runtime.Version.ResultRunIDColumn, RowIDColumn: runtime.Version.ResultRowIDColumn, Columns: queryDatabaseColumns(queryColumns, databaseColumns),
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
	query, err := reportquery.Decode([]byte(runtime.Export.FrozenFiltersJSON), []byte(runtime.Export.FrozenSortJSON))
	if err != nil {
		return closeOnError(err)
	}
	if err := reportquery.ValidateCompiled(query, queryColumns); err != nil {
		return closeOnError(err)
	}
	pagePlan, err := reportoracle.BuildResultQueryPlan(contract, databaseColumns, query)
	if err != nil {
		return closeOnError(err)
	}
	purgePlan, err := reportoracle.BuildPurgePlan(contract)
	if err != nil {
		return closeOnError(err)
	}
	queryTimeout := time.Duration(runtime.Datasource.QueryTimeoutSeconds) * time.Second
	if queryTimeout <= 0 {
		queryTimeout = defaultReportPublicationInspectionTimeout
	}
	return &oracleReportExportSession{adapter: adapter, pagePlan: pagePlan, purgePlan: purgePlan, runUUID: runtime.Run.RunUUID, queryTimeout: queryTimeout}, nil
}

func queryDatabaseColumns(columns []reportquery.Column, output []string) []string {
	result := append([]string(nil), output...)
	seen := make(map[string]struct{}, len(result))
	for _, column := range result {
		seen[strings.ToUpper(column)] = struct{}{}
	}
	for _, column := range columns {
		key := strings.ToUpper(column.DatabaseColumn)
		if _, ok := seen[key]; !ok && (column.Filterable || column.Sortable) {
			result = append(result, column.DatabaseColumn)
			seen[key] = struct{}{}
		}
	}
	return result
}

func (session *oracleReportExportSession) Read(ctx context.Context, columns []string, after *reportoracle.ResultCursor, limit int) (reportoracle.ResultPage, error) {
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
	queryCtx, cancel := context.WithTimeout(ctx, session.queryTimeout)
	defer cancel()
	return session.adapter.ReadResultPage(queryCtx, session.pagePlan, session.runUUID, after, limit)
}

func (session *oracleReportExportSession) Purge(ctx context.Context, batchSize int) (deleted int64, resultErr error) {
	if session == nil || session.adapter == nil || ctx == nil {
		return 0, fmt.Errorf("report export oracle: invalid purge session")
	}
	queryCtx, cancel := context.WithTimeout(ctx, session.queryTimeout)
	defer cancel()
	tx, err := session.adapter.BeginTx(queryCtx, &sql.TxOptions{})
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
	deleted, err = session.adapter.PurgeResultBatch(queryCtx, tx, session.purgePlan, session.runUUID, batchSize)
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
