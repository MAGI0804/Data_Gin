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

func TestPipelineServiceLegacyStepIsVisibleAndGeneratedInDefaultStage(t *testing.T) {
	service, _ := newPipelineIntegrityTestService(t)

	detail, err := service.GetPipeline(t.Context(), 3)
	if err != nil {
		t.Fatalf("GetPipeline() error = %v", err)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].Step.StageID != 301 {
		t.Fatalf("GetPipeline().Steps = %#v, want one normalized legacy step", detail.Steps)
	}
	if len(detail.Stages) != 4 || len(detail.Stages[0].Steps) != 1 || detail.Stages[0].Steps[0].Step.ID != 3010 {
		t.Fatalf("GetPipeline().Stages = %#v, want legacy step once in fetch stage", detail.Stages)
	}

	steps, err := service.GetStageSteps(t.Context(), 301)
	if err != nil {
		t.Fatalf("GetStageSteps() error = %v", err)
	}
	if len(steps) != 1 || steps[0].Step.ID != 3010 || steps[0].Step.StageID != 301 {
		t.Fatalf("GetStageSteps() = %#v, want normalized legacy step in stage 301", steps)
	}

	config, err := service.GenerateStageConfig(t.Context(), 301)
	if err != nil {
		t.Fatalf("GenerateStageConfig() error = %v", err)
	}
	if !strings.Contains(config.GeneratedConfigJSON, `"step_code":"fetch_orders"`) {
		t.Fatalf("generated config omitted legacy step: %s", config.GeneratedConfigJSON)
	}
}

func TestPipelineServicePublishRejectsStaleStageConfig(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	_, err := service.PublishStageConfig(t.Context(), 301)
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("PublishStageConfig() error = %v, want stale config error", err)
	}
	store.assertNoWrites(t)
}

func TestPipelineServicePublishKeepsPublishedSnapshotIdempotent(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	config, err := service.PublishStageConfig(t.Context(), 302)
	if err != nil {
		t.Fatalf("PublishStageConfig() error = %v", err)
	}
	if config.TargetRefID != 77 {
		t.Fatalf("PublishStageConfig().TargetRefID = %d, want 77", config.TargetRefID)
	}
	store.assertNoWrites(t)
}

func TestPipelineServiceEnsureDefaultStagesFillsOnlyMissingTypes(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)

	if err := service.ensureDefaultStages(t.Context(), 4); err != nil {
		t.Fatalf("ensureDefaultStages() error = %v", err)
	}
	stageTypes := store.insertedStageTypes()
	if strings.Join(stageTypes, ",") != "process,push,log" {
		t.Fatalf("inserted stage types = %v, want [process push log]", stageTypes)
	}
}

func TestPipelineServiceLegacyStepUpdateRequiresMatchingDefaultStage(t *testing.T) {
	tests := []struct {
		name      string
		stageID   uint
		update    func(*PipelineService, uint, *requestbody.MethodStepUpdateRequest) error
		wantError bool
	}{
		{
			name:    "pipeline scope accepts fetch default",
			stageID: 301,
			update: func(service *PipelineService, stageID uint, req *requestbody.MethodStepUpdateRequest) error {
				_, err := service.UpdateStepInPipeline(t.Context(), 3, 3010, req)
				return err
			},
		},
		{
			name:      "pipeline scope rejects process stage",
			stageID:   302,
			wantError: true,
			update: func(service *PipelineService, stageID uint, req *requestbody.MethodStepUpdateRequest) error {
				_, err := service.UpdateStepInPipeline(t.Context(), 3, 3010, req)
				return err
			},
		},
		{
			name:    "stage scope accepts fetch default",
			stageID: 301,
			update: func(service *PipelineService, stageID uint, req *requestbody.MethodStepUpdateRequest) error {
				_, err := service.UpdateStepInStage(t.Context(), stageID, 3010, req)
				return err
			},
		},
		{
			name:      "stage scope rejects process stage",
			stageID:   302,
			wantError: true,
			update: func(service *PipelineService, stageID uint, req *requestbody.MethodStepUpdateRequest) error {
				_, err := service.UpdateStepInStage(t.Context(), stageID, 3010, req)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, store := newPipelineIntegrityTestService(t)
			err := test.update(service, test.stageID, &requestbody.MethodStepUpdateRequest{
				StageID: test.stageID, Code: "fetch_orders", Name: "拉取订单", MethodType: "request", TimeoutSeconds: 30,
				Params: []requestbody.MethodParamRequest{{Location: "url", Name: "url", ValueSource: "static", Value: "https://example.test/orders"}},
			})
			if (err != nil) != test.wantError {
				t.Fatalf("update error = %v, wantError %v", err, test.wantError)
			}
			if test.wantError {
				store.assertNoWrites(t)
			} else if !store.updatedStepToStage(301) {
				t.Fatal("legacy step update did not persist stage_id 301")
			}
		})
	}
}

func TestPipelineServiceLegacyStepUpdateConflictDoesNotReplaceConfig(t *testing.T) {
	service, store := newPipelineIntegrityTestService(t)
	store.methodStepUpdateRowsAffected = 0

	_, err := service.UpdateStepInPipeline(t.Context(), 3, 3010, &requestbody.MethodStepUpdateRequest{
		StageID: 301, Code: "fetch_orders", Name: "拉取订单", MethodType: "request", TimeoutSeconds: 30,
		Params: []requestbody.MethodParamRequest{{Location: "url", Name: "url", ValueSource: "static", Value: "https://example.test/orders"}},
	})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("UpdateStepInPipeline() error = %v, want conflict", err)
	}
	if !store.hasConditionalLegacyUpdate() {
		t.Fatal("legacy update SQL does not constrain stage_id = 0")
	}
	if store.hasStepConfigWrites() {
		t.Fatal("legacy update conflict replaced params or outputs")
	}
}

func newPipelineIntegrityTestService(t *testing.T) (*PipelineService, *pipelineIntegrityStore) {
	t.Helper()

	store := &pipelineIntegrityStore{methodStepUpdateRowsAffected: 1}
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

// pipelineIntegrityStore supports only the query and write patterns exercised
// by these ownership and legacy-stage tests. It preserves the concrete DAO
// construction without requiring a live MySQL server.
type pipelineIntegrityStore struct {
	mu                           sync.Mutex
	writes                       []pipelineIntegrityWrite
	methodStepUpdateRowsAffected int64
}

type pipelineIntegrityWrite struct {
	query string
	args  []driver.NamedValue
}

func (s *pipelineIntegrityStore) assertNoWrites(t *testing.T) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.writes) != 0 {
		t.Fatalf("unexpected writes before ownership validation: %v", s.writes)
	}
}

func (s *pipelineIntegrityStore) insertedStageTypes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []string{}
	for _, write := range s.writes {
		if !strings.Contains(strings.ToLower(write.query), "insert into `pipeline_stages`") {
			continue
		}
		for _, arg := range write.args {
			if value, ok := arg.Value.(string); ok && containsString([]string{"fetch", "process", "push", "log"}, value) {
				result = append(result, value)
				break
			}
		}
	}
	return result
}

func (s *pipelineIntegrityStore) updatedStepToStage(stageID uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, write := range s.writes {
		if !strings.Contains(strings.ToLower(write.query), "update `method_steps`") {
			continue
		}
		for _, arg := range write.args {
			if pipelineIntegrityValueID(arg.Value) == stageID {
				return true
			}
		}
	}
	return false
}

func (s *pipelineIntegrityStore) hasConditionalLegacyUpdate() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, write := range s.writes {
		query := strings.ToLower(write.query)
		if strings.Contains(query, "update `method_steps`") && strings.Contains(query, "stage_id = 0") && strings.Contains(query, "pipeline_id =") {
			return true
		}
	}
	return false
}

func (s *pipelineIntegrityStore) hasStepConfigWrites() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, write := range s.writes {
		query := strings.ToLower(write.query)
		if strings.Contains(query, "method_params") || strings.Contains(query, "method_outputs") {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
	return pipelineIntegrityTx{}, nil
}

func (c *pipelineIntegrityConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return pipelineIntegrityRowsFor(query, args), nil
}

func (c *pipelineIntegrityConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.store.mu.Lock()
	rowsAffected := int64(1)
	if strings.Contains(strings.ToLower(query), "update `method_steps`") {
		rowsAffected = c.store.methodStepUpdateRowsAffected
	}
	copiedArgs := append([]driver.NamedValue(nil), args...)
	c.store.writes = append(c.store.writes, pipelineIntegrityWrite{query: query, args: copiedArgs})
	c.store.mu.Unlock()
	return pipelineIntegrityResult(rowsAffected), nil
}

var _ driver.QueryerContext = (*pipelineIntegrityConn)(nil)
var _ driver.ExecerContext = (*pipelineIntegrityConn)(nil)

type pipelineIntegrityTx struct{}

func (pipelineIntegrityTx) Commit() error   { return nil }
func (pipelineIntegrityTx) Rollback() error { return nil }

type pipelineIntegrityResult int64

func (r pipelineIntegrityResult) LastInsertId() (int64, error) { return int64(r), nil }
func (r pipelineIntegrityResult) RowsAffected() (int64, error) { return int64(r), nil }

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
		case 3:
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageColumns, values: [][]driver.Value{
				{301, 3, "fetch", "数据获取", 1, true, 1, 1},
				{302, 3, "process", "数据处理", 2, true, 1, 1},
				{303, 3, "push", "数据推送", 3, true, 1, 1},
				{304, 3, "log", "日志记录", 4, true, 1, 1},
			}}
		case 4:
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageColumns, values: [][]driver.Value{{401, 4, "fetch", "已有获取阶段", 9, false, 1, 1}}}
		case 301:
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageColumns, values: [][]driver.Value{{301, 3, "fetch", "数据获取", 1, true, 1, 1}}}
		case 302:
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageColumns, values: [][]driver.Value{{302, 3, "process", "数据处理", 2, true, 1, 1}}}
		}
	case strings.Contains(query, "method_steps"):
		if id == 201 {
			return &pipelineIntegrityRows{columns: pipelineIntegrityStepColumns, values: [][]driver.Value{{201, 2, 101, "fetch_orders", "拉取订单", "request", 1, true, 30, "{}", 1, 1}}}
		}
		if id == 3 || id == 3010 {
			return &pipelineIntegrityRows{columns: pipelineIntegrityStepColumns, values: [][]driver.Value{{3010, 3, 0, "fetch_orders", "拉取订单", "request", 1, true, 30, "{}", 1, 1}}}
		}
	case strings.Contains(query, "method_params"):
		return &pipelineIntegrityRows{columns: pipelineIntegrityParamColumns, values: [][]driver.Value{{3011, 3010, "url", "url", "static", "https://example.test/orders", "string", true, false, "", 0, 1, 1}}}
	case strings.Contains(query, "method_outputs"):
		return &pipelineIntegrityRows{columns: pipelineIntegrityOutputColumns}
	case strings.Contains(query, "stage_generated_configs"):
		if id == 301 {
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageConfigColumns, values: [][]driver.Value{{901, 3, 301, "fetch", `{"stage_id":301,"stage_type":"fetch","stage_name":"数据获取","target_ref_type":"source_definition","steps":[]}`, "source_definition", 0, 1, 1, 1}}}
		}
		if id == 302 {
			return &pipelineIntegrityRows{columns: pipelineIntegrityStageConfigColumns, values: [][]driver.Value{{902, 3, 302, "process", `{}`, "transform_rule", 77, 1, 1, 1}}}
		}
		return &pipelineIntegrityRows{columns: pipelineIntegrityStageConfigColumns}
	}
	return &pipelineIntegrityRows{}
}

func pipelineIntegrityArgumentID(args []driver.NamedValue) uint {
	if len(args) == 0 {
		return 0
	}
	return pipelineIntegrityValueID(args[0].Value)
}

func pipelineIntegrityValueID(input driver.Value) uint {
	switch value := input.(type) {
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
var pipelineIntegrityParamColumns = []string{"id", "step_id", "location", "name", "value_source", "value", "value_type", "required", "secret", "description", "order_index", "created_at", "updated_at"}
var pipelineIntegrityOutputColumns = []string{"id", "step_id", "name", "source_path", "value_type", "required", "description", "order_index", "created_at", "updated_at"}
var pipelineIntegrityStageConfigColumns = []string{"id", "pipeline_id", "stage_id", "stage_type", "generated_config_json", "target_ref_type", "target_ref_id", "version", "created_at", "updated_at"}

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
