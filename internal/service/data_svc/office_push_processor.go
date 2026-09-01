package data_svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	appconfig "gin-biz-web-api/config"
	"gin-biz-web-api/connector/feishu"
	"gin-biz-web-api/global"
	"gin-biz-web-api/internal/reportoracle"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
	projectredis "gin-biz-web-api/pkg/redis"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const officePushMaximumRows = int64(100_000)

var ErrOfficePushProcessNonRetryable = errors.New("office push processor: non-retryable")

type officeBot interface {
	UploadFile(context.Context, string, string) (string, error)
	SendText(context.Context, string, string, string, string) (string, error)
	SendFile(context.Context, string, string, string, string) (string, error)
}

type officeOracleConnection interface {
	InspectProcedure(context.Context, reportoracle.ProcedureRef) ([]reportoracle.ProcedureArgument, error)
	InspectResultSnapshotContract(context.Context, reportoracle.ResultSnapshotRef) (reportoracle.ResultSnapshotContract, error)
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	Execute(context.Context, *sql.Tx, reportoracle.CallPlan, map[string]interface{}) error
	CountResultRowsTx(context.Context, *sql.Tx, reportoracle.ResultCountPlan) (int64, error)
	ReadResultPage(context.Context, reportoracle.ResultPagePlan, *reportoracle.ResultCursor, int) (reportoracle.ResultPage, error)
	QuerySelect(context.Context, string, ...interface{}) (*sql.Rows, error)
	Close() error
}

type officeBotFactory func() (officeBot, error)
type officeOracleOpener func(context.Context, reportoracle.Config) (officeOracleConnection, error)

type OfficePushProcessor struct {
	db         *gorm.DB
	newBot     officeBotFactory
	openOracle officeOracleOpener
	now        func() time.Time
}

func NewOfficePushProcessor() *OfficePushProcessor {
	var once sync.Once
	var bot officeBot
	var botErr error
	newBot := func() (officeBot, error) {
		once.Do(func() {
			redisInstance := projectredis.Instance()
			if redisInstance == nil || redisInstance.Client == nil {
				botErr = fmt.Errorf("office push processor: redis is unavailable")
				return
			}
			tokens, err := feishu.NewTenantTokenProvider(global.Credentials.FeishuAppID(), global.Credentials.FeishuAppSecret(), nil, redisInstance.Client)
			if err != nil {
				botErr = err
				return
			}
			bot, botErr = feishu.NewBotClient(tokens, nil)
		})
		return bot, botErr
	}
	return newOfficePushProcessor(database.DB, newBot, func(ctx context.Context, config reportoracle.Config) (officeOracleConnection, error) {
		return reportoracle.Open(ctx, config)
	})
}

func newOfficePushProcessor(db *gorm.DB, newBot officeBotFactory, openOracle officeOracleOpener) *OfficePushProcessor {
	if db == nil || newBot == nil || openOracle == nil {
		panic("office push processor: dependencies are required")
	}
	return &OfficePushProcessor{db: db, newBot: newBot, openOracle: openOracle, now: func() time.Time { return time.Now().UTC() }}
}

func (processor *OfficePushProcessor) Process(ctx context.Context, runID uint, retryAllowed bool) error {
	if processor == nil || processor.db == nil || processor.newBot == nil || processor.openOracle == nil || ctx == nil || runID == 0 {
		return fmt.Errorf("%w: invalid processor request", ErrOfficePushProcessNonRetryable)
	}
	run, target, message, err := processor.claim(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status == model.OfficePushRunStatusSucceeded || run.Status == model.OfficePushRunStatusUnknown {
		return nil
	}
	bot, err := processor.newBot()
	if err != nil {
		return processor.fail(ctx, run.ID, retryAllowed, "FEISHU_CONFIGURATION_INVALID", "飞书机器人配置不可用", err)
	}

	var messageID string
	var rowCount int64
	if message.SourceType == model.OfficeMessageSourceEdited {
		messageID, err = bot.SendText(ctx, target.ReceiveIDType, target.ReceiveID, message.Content, run.RunUUID)
	} else {
		path, fileName, exportedRows, exportErr := processor.exportWorkbook(ctx, message, run.ParametersJSON)
		if exportErr != nil {
			return processor.fail(ctx, run.ID, retryAllowed, "ORACLE_EXPORT_FAILED", "Oracle 数据导出失败", exportErr)
		}
		defer os.Remove(path)
		rowCount = exportedRows
		fileKey, uploadErr := bot.UploadFile(ctx, path, fileName)
		if uploadErr != nil {
			return processor.fail(ctx, run.ID, retryAllowed, "FEISHU_FILE_UPLOAD_FAILED", "飞书文件上传失败", uploadErr)
		}
		messageID, err = bot.SendFile(ctx, target.ReceiveIDType, target.ReceiveID, fileKey, run.RunUUID)
	}
	if err != nil {
		return processor.fail(ctx, run.ID, retryAllowed, "FEISHU_MESSAGE_FAILED", "飞书消息发送失败", err)
	}
	finishedAt := processor.now().UTC()
	result := processor.db.WithContext(ctx).Model(&model.OfficePushRun{}).
		Where("id = ? AND status = ?", run.ID, model.OfficePushRunStatusRunning).
		Updates(map[string]interface{}{
			"status": model.OfficePushRunStatusSucceeded, "row_count": rowCount,
			"feishu_message_id": messageID, "error_code": "", "error_message_safe": "",
			"finished_at": finishedAt, "updated_at": finishedAt,
		})
	if result.Error != nil || result.RowsAffected != 1 {
		return fmt.Errorf("office push processor: persist success: %w", result.Error)
	}
	return nil
}

func (processor *OfficePushProcessor) claim(ctx context.Context, runID uint) (model.OfficePushRun, model.OfficePushTarget, model.OfficeMessage, error) {
	var run model.OfficePushRun
	var target model.OfficePushTarget
	var message model.OfficeMessage
	err := processor.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runID).First(&run).Error; err != nil {
			return fmt.Errorf("office push processor: load run: %w", err)
		}
		if run.Status == model.OfficePushRunStatusSucceeded || run.Status == model.OfficePushRunStatusUnknown {
			return nil
		}
		if err := tx.Where("id = ? AND enabled = ?", run.TargetID, true).First(&target).Error; err != nil {
			return fmt.Errorf("%w: push target is unavailable", ErrOfficePushProcessNonRetryable)
		}
		if err := tx.Where("id = ? AND enabled = ?", run.MessageID, true).First(&message).Error; err != nil {
			return fmt.Errorf("%w: message is unavailable", ErrOfficePushProcessNonRetryable)
		}
		now := processor.now().UTC()
		if err := tx.Model(&run).Updates(map[string]interface{}{
			"status": model.OfficePushRunStatusRunning, "attempt_count": gorm.Expr("attempt_count + 1"),
			"started_at": now, "finished_at": nil, "error_code": "", "error_message_safe": "", "updated_at": now,
		}).Error; err != nil {
			return fmt.Errorf("office push processor: claim run: %w", err)
		}
		run.Status = model.OfficePushRunStatusRunning
		return nil
	})
	return run, target, message, err
}

func (processor *OfficePushProcessor) fail(ctx context.Context, runID uint, retryAllowed bool, code, safeMessage string, cause error) error {
	now := processor.now().UTC()
	status := model.OfficePushRunStatusFailed
	if retryAllowed && officePushRetryable(cause) {
		status = model.OfficePushRunStatusQueued
	}
	updateErr := processor.db.WithContext(ctx).Model(&model.OfficePushRun{}).Where("id = ?", runID).Updates(map[string]interface{}{
		"status": status, "error_code": code, "error_message_safe": safeMessage, "finished_at": now, "updated_at": now,
	}).Error
	if updateErr != nil {
		return errors.Join(cause, fmt.Errorf("office push processor: persist failure: %w", updateErr))
	}
	if status == model.OfficePushRunStatusQueued {
		return cause
	}
	return fmt.Errorf("%w: %s", ErrOfficePushProcessNonRetryable, safeMessage)
}

func officePushRetryable(err error) bool {
	var botError *feishu.BotError
	if errors.As(err, &botError) {
		return botError.Retryable
	}
	return true
}

func (processor *OfficePushProcessor) exportWorkbook(ctx context.Context, message model.OfficeMessage, rawParameters model.JSONText) (string, string, int64, error) {
	mappings, _, err := normalizeOfficeColumnMappings(message.ColumnMappingJSON)
	if err != nil {
		return "", "", 0, err
	}
	oracleConfig, queryTimeout, err := officeOracleConfigFromEnvironment()
	if err != nil {
		return "", "", 0, err
	}
	openCtx, cancel := context.WithTimeout(ctx, oracleConfig.ConnectTimeout)
	connection, err := processor.openOracle(openCtx, oracleConfig)
	cancel()
	if err != nil {
		return "", "", 0, err
	}
	defer connection.Close()

	queryCtx, queryCancel := context.WithTimeout(ctx, queryTimeout)
	defer queryCancel()
	var pager interface {
		reportExportPageReader
		Close() error
	}
	switch message.SourceType {
	case model.OfficeMessageSourceOracleProcedure:
		pager, err = newOfficeProcedurePager(queryCtx, connection, message, mappings)
	case model.OfficeMessageSourceOracleQuery:
		pager, err = newOfficeSelectPager(queryCtx, connection, message, mappings, rawParameters)
	default:
		err = fmt.Errorf("office push processor: unsupported message source")
	}
	if err != nil {
		return "", "", 0, err
	}
	defer pager.Close()

	tempDir, err := os.MkdirTemp("", "office-push-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("office push processor: create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	fileName := officeWorkbookFileName(message.Name)
	outputPath := filepath.Join(tempDir, fileName)
	renderer := NewReportExportRenderer(pager)
	renderer.maxSheets = 1
	columns := officeFrozenColumns(mappings)
	result, err := renderer.Render(queryCtx, ReportExportRenderRequest{Columns: columns, OutputPath: outputPath}, nil)
	if err != nil {
		return "", "", 0, err
	}
	stablePath := filepath.Join(os.TempDir(), "office-push-"+strconv.FormatUint(uint64(message.ID), 10)+"-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".xlsx")
	if err := os.Rename(outputPath, stablePath); err != nil {
		return "", "", 0, fmt.Errorf("office push processor: move workbook: %w", err)
	}
	return stablePath, fileName, result.ProcessedRows, nil
}

func officeOracleConfigFromEnvironment() (reportoracle.Config, time.Duration, error) {
	configured, err := appconfig.LoadReportInputQueryConfig()
	if err != nil {
		return reportoracle.Config{}, 0, err
	}
	oracle := configured.Oracle
	if oracle.Host == "" || oracle.Username == "" || oracle.Password == "" || (oracle.ServiceName == "" && oracle.SID == "") || (oracle.ServiceName != "" && oracle.SID != "") {
		return reportoracle.Config{}, 0, fmt.Errorf("office push processor: default Oracle environment is incomplete")
	}
	return reportoracle.Config{
		Host: oracle.Host, Port: oracle.Port, ServiceName: oracle.ServiceName, SID: oracle.SID,
		Username: oracle.Username, Password: oracle.Password, Timezone: oracle.Timezone,
		ConnectTimeout: oracle.ConnectTimeout, MaxOpenConnections: oracle.MaxOpenConnections,
		MaxIdleConnections: oracle.MaxIdleConnections, ConnectionLifetime: oracle.ConnectionLifetime,
		ConnectionIdleTime: oracle.ConnectionIdleTime, PrefetchRows: oracle.PrefetchRows, FetchArraySize: oracle.ArraySize,
	}, oracle.QueryTimeout, nil
}

func newOfficeProcedurePager(ctx context.Context, connection officeOracleConnection, message model.OfficeMessage, mappings []OfficeColumnMapping) (*officeProcedurePager, error) {
	ref := reportoracle.ProcedureRef{Owner: message.ProcedureOwner, Package: message.PackageName, Name: message.ProcedureName, Overload: message.ProcedureOverload}
	arguments, err := connection.InspectProcedure(ctx, ref)
	if err != nil {
		return nil, err
	}
	if len(arguments) != 0 {
		return nil, fmt.Errorf("office push processor: configured procedure must not require arguments")
	}
	plan, err := reportoracle.BuildCallPlan(ref, nil)
	if err != nil {
		return nil, err
	}
	columns := officeSourceColumns(mappings)
	contract, err := connection.InspectResultSnapshotContract(ctx, reportoracle.ResultTableSnapshotRef(
		reportoracle.ResultTableRef{Owner: message.ResultTableOwner, Name: message.ResultTableName}, columns,
	))
	if err != nil {
		return nil, err
	}
	pagePlan, err := reportoracle.BuildResultPagePlan(contract, columns)
	if err != nil {
		return nil, err
	}
	countPlan, err := reportoracle.BuildResultCountPlan(contract)
	if err != nil {
		return nil, err
	}
	tx, err := connection.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	if err := connection.Execute(ctx, tx, plan, map[string]interface{}{}); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	rowCount, err := connection.CountResultRowsTx(ctx, tx, countPlan)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if rowCount > officePushMaximumRows {
		_ = tx.Rollback()
		return nil, fmt.Errorf("office push processor: Oracle result exceeds %d rows", officePushMaximumRows)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("office push processor: commit Oracle procedure: %w", err)
	}
	return &officeProcedurePager{connection: connection, plan: pagePlan}, nil
}

type officeProcedurePager struct {
	connection officeOracleConnection
	plan       reportoracle.ResultPagePlan
}

func (pager *officeProcedurePager) Read(ctx context.Context, _ []string, after *reportoracle.ResultCursor, limit int) (reportoracle.ResultPage, error) {
	return pager.connection.ReadResultPage(ctx, pager.plan, after, limit)
}

func (*officeProcedurePager) Close() error { return nil }

type officeSelectPager struct {
	rows          *sql.Rows
	requested     []string
	columnIndexes []int
	pending       []interface{}
	readRows      int64
}

func newOfficeSelectPager(ctx context.Context, connection officeOracleConnection, message model.OfficeMessage, mappings []OfficeColumnMapping, rawParameters model.JSONText) (*officeSelectPager, error) {
	schema, _, err := normalizeOfficeQueryParameters(message.SelectSQL, message.ParameterSchemaJSON)
	if err != nil {
		return nil, err
	}
	arguments, err := officeParameterArguments(schema, rawParameters)
	if err != nil {
		return nil, err
	}
	rows, err := connection.QuerySelect(ctx, message.SelectSQL, arguments...)
	if err != nil {
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, fmt.Errorf("office push processor: read SELECT columns: %w", err)
	}
	byName := make(map[string]int, len(columns))
	for index, column := range columns {
		key := strings.ToUpper(strings.TrimSpace(column))
		if _, duplicate := byName[key]; duplicate {
			rows.Close()
			return nil, fmt.Errorf("office push processor: SELECT returned duplicate columns")
		}
		byName[key] = index
	}
	requested := officeSourceColumns(mappings)
	indexes := make([]int, len(requested))
	for index, column := range requested {
		position, exists := byName[column]
		if !exists {
			rows.Close()
			return nil, fmt.Errorf("office push processor: configured SELECT column %s is missing", column)
		}
		indexes[index] = position
	}
	return &officeSelectPager{rows: rows, requested: requested, columnIndexes: indexes}, nil
}

func (pager *officeSelectPager) Read(ctx context.Context, columns []string, after *reportoracle.ResultCursor, limit int) (reportoracle.ResultPage, error) {
	if pager == nil || pager.rows == nil || ctx == nil || limit < 1 || len(columns) != len(pager.requested) {
		return reportoracle.ResultPage{}, fmt.Errorf("office push processor: invalid SELECT page request")
	}
	for index := range columns {
		if !strings.EqualFold(columns[index], pager.requested[index]) {
			return reportoracle.ResultPage{}, fmt.Errorf("office push processor: SELECT column contract changed")
		}
	}
	if after != nil && after.Key != strconv.FormatInt(pager.readRows, 10) {
		return reportoracle.ResultPage{}, fmt.Errorf("office push processor: SELECT cursor changed")
	}
	page := reportoracle.ResultPage{Columns: append([]string(nil), pager.requested...), Rows: make([]reportoracle.ResultRow, 0, limit)}
	for len(page.Rows) < limit {
		values, ok, err := pager.nextRow()
		if err != nil {
			return reportoracle.ResultPage{}, err
		}
		if !ok {
			return page, nil
		}
		pager.readRows++
		if pager.readRows > officePushMaximumRows {
			return reportoracle.ResultPage{}, fmt.Errorf("office push processor: Oracle result exceeds %d rows", officePushMaximumRows)
		}
		rowValues := make([]interface{}, len(pager.columnIndexes))
		for index, position := range pager.columnIndexes {
			rowValues[index] = values[position]
		}
		key := strconv.FormatInt(pager.readRows, 10)
		page.Rows = append(page.Rows, reportoracle.ResultRow{Key: key, Values: rowValues})
		page.NextKey = key
	}
	values, ok, err := pager.nextRow()
	if err != nil {
		return reportoracle.ResultPage{}, err
	}
	if ok {
		pager.pending = values
		page.HasNext = true
	}
	return page, nil
}

func (pager *officeSelectPager) nextRow() ([]interface{}, bool, error) {
	if pager.pending != nil {
		values := pager.pending
		pager.pending = nil
		return values, true, nil
	}
	if !pager.rows.Next() {
		if err := pager.rows.Err(); err != nil {
			return nil, false, fmt.Errorf("office push processor: iterate SELECT rows: %w", err)
		}
		return nil, false, nil
	}
	columns, err := pager.rows.Columns()
	if err != nil {
		return nil, false, fmt.Errorf("office push processor: read SELECT columns: %w", err)
	}
	values := make([]interface{}, len(columns))
	destinations := make([]interface{}, len(columns))
	for index := range values {
		destinations[index] = &values[index]
	}
	if err := pager.rows.Scan(destinations...); err != nil {
		return nil, false, fmt.Errorf("office push processor: scan SELECT row: %w", err)
	}
	return values, true, nil
}

func (pager *officeSelectPager) Close() error {
	if pager == nil || pager.rows == nil {
		return nil
	}
	return pager.rows.Close()
}

func officeFrozenColumns(mappings []OfficeColumnMapping) []frozenResultColumn {
	columns := make([]frozenResultColumn, len(mappings))
	for index, mapping := range mappings {
		columns[index] = frozenResultColumn{
			FieldID: strings.ToLower(mapping.SourceColumn), LogicalCode: strings.ToLower(mapping.SourceColumn),
			DatabaseColumn: mapping.SourceColumn, ValueType: mapping.ValueType,
			ExcelHeader: mapping.Header, PreviewHeader: mapping.Header, ExportOrder: mapping.Order,
			DisplayOrder: mapping.Order, PreviewVisible: true, ExportVisible: true, ExportAllowed: true, ExcelWidth: mapping.Width,
		}
	}
	return columns
}

func officeSourceColumns(mappings []OfficeColumnMapping) []string {
	columns := make([]string, len(mappings))
	for index, mapping := range mappings {
		columns[index] = mapping.SourceColumn
	}
	return columns
}

func officeWorkbookFileName(name string) string {
	name = strings.Map(func(character rune) rune {
		if character == '/' || character == '\\' || unicode.IsControl(character) {
			return '-'
		}
		return character
	}, strings.TrimSpace(name))
	if name == "" {
		name = "办公消息"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	return name + ".xlsx"
}
