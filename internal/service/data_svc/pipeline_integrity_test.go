package data_svc

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/pkg/database"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPipelineServiceRejectsUnknownPipelineBeforeCreatingStages(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	_, err := service.GetPipelineStages(t.Context(), 999)
	if err == nil {
		t.Fatal("GetPipelineStages() error = nil, want error for an unknown pipeline")
	}
	store.assertNoWrites(t)
}

func TestPipelineServiceCreateStageRejectsUnknownPipeline(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	_, err := service.CreateStage(t.Context(), 999, &requestbody.PipelineStageCreateRequest{
		StageType: "fetch",
		Name:      "数据获取",
	})
	if err == nil {
		t.Fatal("CreateStage() error = nil, want error for an unknown pipeline")
	}
	store.assertNoWrites(t)
}

func TestPipelineServiceCreateStepRejectsStageFromAnotherPipeline(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	_, err := service.CreateStep(t.Context(), 1, &requestbody.MethodStepCreateRequest{
		StageID:    101,
		Code:       "fetch_orders",
		Name:       "拉取订单",
		MethodType: "request",
	})
	if err == nil {
		t.Fatal("CreateStep() error = nil, want cross-pipeline stage error")
	}
	store.assertNoWrites(t)
}

func TestPipelineServiceUpdateStepInPipelineRejectsStepFromAnotherPipeline(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	_, err := service.UpdateStepInPipeline(t.Context(), 1, 201, &requestbody.MethodStepUpdateRequest{
		Code: "fetch_orders",
		Name: "拉取订单",
	})
	if err == nil {
		t.Fatal("UpdateStepInPipeline() error = nil, want cross-pipeline step error")
	}
	store.assertNoWrites(t)
}

func TestPipelineServiceUpdateStepInStageRejectsStepFromAnotherStage(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	_, err := service.UpdateStepInStage(t.Context(), 102, 201, &requestbody.MethodStepUpdateRequest{
		Code: "fetch_orders",
		Name: "拉取订单",
	})
	if err == nil {
		t.Fatal("UpdateStepInStage() error = nil, want cross-stage step error")
	}
	store.assertNoWrites(t)
}

func newPipelineIntegrityTestService(t *testing.T) (*PipelineService, *pipelineIntegrityStore) {
	t.Helper()

	store := &pipelineIntegrityStore{}
	rawDB := sql.OpenDB(pipelineIntegrityConnector{store: store})
	t.Cleanup(func() {
		if err := rawDB.Close(); err != nil {
			t.Errorf("close pipeline integrity test database: %v", err)
		}
	})

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn: rawDB, SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open pipeline integrity test database: %v", err)
	}

	previousDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previousDB })
	return NewPipelineService(), store
}

// pipelineIntegrityStore is deliberately limited to the read-only queries used
// before each ownership guard. It makes a regression test possible without a
// live MySQL server while preserving the concrete DAO construction used here.
type pipelineIntegrityStore struct {
	mu     sync.Mutex
	writes []string
}

func (s *pipelineIntegrityStore) assertNoWrites(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.writes) != 0 {
		t.Fatalf("unexpected writes before ownership validation: %v", s.writes)
	}
}

type pipelineIntegrityConnector struct {
	store *pipelineIntegrityStore
}

func (c pipelineIntegrityConnector) Connect(context.Context) (driver.Conn, error) {
	return &pipelineIntegrityConn{store: c.store}, nil
}

func (c pipelineIntegrityConnector) Driver() driver.Driver { return pipelineIntegrityDriver{} }

type pipelineIntegrityDriver struct{}

func (pipelineIntegrityDriver) Open(string) (driver.Conn, error) {
	return nil, fmt.Errorf("pipeline integrity test driver requires a connector")
}

type pipelineIntegrityConn struct {
	store *pipelineIntegrityStore
}

func (c *pipelineIntegrityConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepared statements are not supported by pipeline integrity test driver")
}

func (c *pipelineIntegrityConn) Close() error { return nil }

func (c *pipelineIntegrityConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("transactions are not supported by pipeline integrity test driver")
}

func (c *pipelineIntegrityConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return pipelineIntegrityRowsFor(query, args), nil
}

func (c *pipelineIntegrityConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.store.mu.Lock()
	c.store.writes = append(c.store.writes, query)
	c.store.mu.Unlock()
	return driver.RowsAffected(0), nil
}

var _ driver.QueryerContext = (*pipelineIntegrityConn)(nil)
var _ driver.ExecerContext = (*pipelineIntegrityConn)(nil)

func pipelineIntegrityRowsFor(query string, args []driver.NamedValue) driver.Rows {
	query = strings.ToLower(query)
	id := pipelineIntegrityArgumentID(args)
	switch {
	case strings.Contains(query, "pipeline_definitions"):
		if id == 999 {
			return &pipelineIntegrityRows{columns: pipelineIntegrityPipelineColumns}
		}
		return &pipelineIntegrityRows{columns: pipelineIntegrityPipelineColumns, values: [][]driver.Value{{id, "流水线", fmt.Sprintf("pipeline_%d", id), "", true, 1, 1}}}
	case strings.Contains(query, "pipeline_stages"):
		switch id {
		case 101:
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageColumns, values: [][]driver.Value{{101, 2, "fetch", "数据获取", 1, true, 1, 1}}}
		case 102:
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageColumns, values: [][]driver.Value{{102, 1, "fetch", "数据获取", 1, true, 1, 1}}}
		}
	case strings.Contains(query, "method_steps"):
		if id == 201 {
			return &pipelineIntegrityRows{columns: pipelineIntegrityStepColumns, values: [][]driver.Value{{201, 2, 101, "fetch_orders", "拉取订单", "request", 1, true, 30, "{}", 1, 1}}}
		}
	}
	return &pipelineIntegrityRows{}
}

func pipelineIntegrityArgumentID(args []driver.NamedValue) uint {
	if len(args) == 0 {
		return 0
	}
	switch value := args[0].Value.(type) {
	case uint:
		return value
	case uint64:
		return uint(value)
	case int64:
		return uint(value)
	case int:
		return uint(value)
	default:
		return 0
	}
}

var pipelineIntegrityPipelineColumns = []string{"id", "name", "code", "description", "enabled", "created_at", "updated_at"}
var pipelineIntegrityStageColumns = []string{"id", "pipeline_id", "stage_type", "name", "order_index", "enabled", "created_at", "updated_at"}
var pipelineIntegrityStepColumns = []string{"id", "pipeline_id", "stage_id", "code", "name", "method_type", "order_index", "enabled", "timeout_seconds", "generated_config_json", "created_at", "updated_at"}

type pipelineIntegrityRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *pipelineIntegrityRows) Columns() []string { return r.columns }
func (r *pipelineIntegrityRows) Close() error      { return nil }

func (r *pipelineIntegrityRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
