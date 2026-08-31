package data_svc

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
)

type fakeExcelMatchModelColumnSource struct {
	mu        sync.Mutex
	startOnce sync.Once
	columns   []data_dao.ExcelMatchModelColumn
	err       error
	calls     int
	started   chan struct{}
	release   <-chan struct{}
}

func (source *fakeExcelMatchModelColumnSource) ListModelColumns(context.Context) ([]data_dao.ExcelMatchModelColumn, error) {
	source.mu.Lock()
	source.calls++
	columns := append([]data_dao.ExcelMatchModelColumn(nil), source.columns...)
	err := source.err
	started := source.started
	release := source.release
	source.mu.Unlock()
	if started != nil {
		source.startOnce.Do(func() { close(started) })
	}
	if release != nil {
		<-release
	}
	return columns, err
}

func (source *fakeExcelMatchModelColumnSource) callCount() int {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.calls
}

func TestBuildExcelMatchModelCatalogAddsReadableMappings(t *testing.T) {
	rows := []data_dao.ExcelMatchModelColumn{
		{
			TableName:       "qimai_order_data",
			ColumnName:      "order_no",
			DataType:        "varchar",
			ColumnType:      "varchar(100)",
			ColumnComment:   "业务订单号",
			OrdinalPosition: 2,
			IsNullable:      "NO",
		},
		{
			TableName:       "bojun_retail_orders",
			ColumnName:      "c_store_name",
			DataType:        "varchar",
			ColumnType:      "varchar(255)",
			OrdinalPosition: 3,
			IsNullable:      "YES",
		},
		{
			TableName:       "bojun_retail_orders",
			ColumnName:      "docno",
			DataType:        "varchar",
			ColumnType:      "varchar(255)",
			OrdinalPosition: 2,
			IsNullable:      "NO",
		},
		{
			TableName:       "bojun_retail_orders",
			ColumnName:      "completed_at",
			DataType:        "datetime",
			ColumnType:      "datetime",
			OrdinalPosition: 4,
			IsNullable:      "YES",
		},
	}

	models := buildExcelMatchModelCatalog(rows)
	if len(models) != 2 {
		t.Fatalf("models length = %d, want 2", len(models))
	}
	if models[0].TableName != "bojun_retail_orders" || models[1].TableName != "qimai_order_data" {
		t.Fatalf("models order = %#v", []string{models[0].TableName, models[1].TableName})
	}

	bojun := models[0]
	if bojun.Name != "伯俊零售订单" || bojun.ModelName != "BojunRetailOrder" {
		t.Fatalf("bojun model identity = %#v", bojun)
	}
	if !strings.Contains(bojun.Mapping, "BojunRetailOrder") || !strings.Contains(bojun.Mapping, "bojun_retail_orders") {
		t.Fatalf("bojun mapping = %q", bojun.Mapping)
	}
	if len(bojun.Fields) != 3 || bojun.Fields[0].ColumnName != "docno" ||
		bojun.Fields[1].ColumnName != "c_store_name" || bojun.Fields[2].ColumnName != "completed_at" {
		t.Fatalf("bojun fields = %#v", bojun.Fields)
	}

	docNo := bojun.Fields[0]
	if docNo.Name != "伯俊零售单号" || docNo.ModelField != "DocNo" {
		t.Fatalf("docno identity = %#v", docNo)
	}
	if !strings.Contains(docNo.Mapping, "BojunRetailOrder.DocNo") || !strings.Contains(docNo.Mapping, "bojun_retail_orders.docno") {
		t.Fatalf("docno mapping = %q", docNo.Mapping)
	}
	if docNo.DataType != "varchar(255)" || docNo.Nullable {
		t.Fatalf("docno database metadata = %#v", docNo)
	}
	if !strings.Contains(docNo.Description, "数据库列") {
		t.Fatalf("docno description = %q", docNo.Description)
	}

	completedAt := bojun.Fields[2]
	if completedAt.Name != "订单完成时间" || completedAt.ModelField != "CompletedAt" ||
		completedAt.DataType != "datetime" || !completedAt.Nullable {
		t.Fatalf("completed_at metadata = %#v", completedAt)
	}
	if !strings.Contains(completedAt.Mapping, "BojunRetailOrder.CompletedAt") ||
		!strings.Contains(completedAt.Mapping, "bojun_retail_orders.completed_at") ||
		!strings.Contains(completedAt.Description, "extendedFields1") {
		t.Fatalf("completed_at mapping = %#v", completedAt)
	}

	qimaiOrderNo := models[1].Fields[0]
	if qimaiOrderNo.Name != "业务订单号" || qimaiOrderNo.ModelField != "OrderNo" {
		t.Fatalf("qimai order field = %#v", qimaiOrderNo)
	}
}

func TestBuildExcelMatchModelCatalogExplainsUnknownTablesWithoutComments(t *testing.T) {
	models := buildExcelMatchModelCatalog([]data_dao.ExcelMatchModelColumn{{
		TableName:       "custom_store_mappings",
		ColumnName:      "source_code",
		DataType:        "varchar",
		ColumnType:      "varchar(64)",
		OrdinalPosition: 1,
		IsNullable:      "YES",
	}})

	if len(models) != 1 || len(models[0].Fields) != 1 {
		t.Fatalf("catalog = %#v", models)
	}
	model := models[0]
	field := model.Fields[0]
	if model.Name != "custom_store_mappings" || model.ModelName != "custom_store_mappings" {
		t.Fatalf("unknown model identity = %#v", model)
	}
	if field.Name != "source_code" || field.ModelField != "source_code" {
		t.Fatalf("unknown field identity = %#v", field)
	}
	if !strings.Contains(model.Description, "custom_store_mappings") ||
		!strings.Contains(field.Description, "varchar(64)") ||
		!strings.Contains(field.Description, "custom_store_mappings.source_code") {
		t.Fatalf("fallback explanations: model=%q field=%q", model.Description, field.Description)
	}
}

func TestExcelMatchModelCatalogCache(t *testing.T) {
	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	source := &fakeExcelMatchModelColumnSource{columns: []data_dao.ExcelMatchModelColumn{{
		TableName: "qimai_order_data", ColumnName: "order_no", ColumnType: "varchar(100)", OrdinalPosition: 1,
	}}}
	cache := newExcelMatchModelCatalogCache(source, 5*time.Minute)
	cache.now = func() time.Time { return now }

	first, err := cache.List(context.Background())
	if err != nil || len(first) != 1 {
		t.Fatalf("first List() models=%#v error=%v", first, err)
	}
	first[0].Fields[0].Name = "mutated"
	second, err := cache.List(context.Background())
	if err != nil || second[0].Fields[0].Name == "mutated" {
		t.Fatalf("cached List() models=%#v error=%v", second, err)
	}
	if calls := source.callCount(); calls != 1 {
		t.Fatalf("source calls = %d, want 1", calls)
	}

	now = now.Add(6 * time.Minute)
	if _, err := cache.List(context.Background()); err != nil {
		t.Fatalf("expired List() error = %v", err)
	}
	if calls := source.callCount(); calls != 2 {
		t.Fatalf("source calls after expiry = %d, want 2", calls)
	}
}

func TestExcelMatchModelCatalogCacheDoesNotCacheFailures(t *testing.T) {
	source := &fakeExcelMatchModelColumnSource{err: errors.New("database unavailable")}
	cache := newExcelMatchModelCatalogCache(source, time.Minute)
	if _, err := cache.List(context.Background()); err == nil {
		t.Fatal("first List() error = nil")
	}
	source.mu.Lock()
	source.err = nil
	source.columns = []data_dao.ExcelMatchModelColumn{{TableName: "orders", ColumnName: "id", OrdinalPosition: 1}}
	source.mu.Unlock()
	models, err := cache.List(context.Background())
	if err != nil || len(models) != 1 {
		t.Fatalf("second List() models=%#v error=%v", models, err)
	}
	if calls := source.callCount(); calls != 2 {
		t.Fatalf("source calls = %d, want 2", calls)
	}
}

func TestExcelMatchModelCatalogCacheCollapsesConcurrentMisses(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	source := &fakeExcelMatchModelColumnSource{
		columns: []data_dao.ExcelMatchModelColumn{{TableName: "orders", ColumnName: "id", OrdinalPosition: 1}},
		started: started,
		release: release,
	}
	cache := newExcelMatchModelCatalogCache(source, time.Minute)
	const callers = 8
	errorsByCaller := make(chan error, callers)
	for range callers {
		go func() {
			_, err := cache.List(context.Background())
			errorsByCaller <- err
		}()
	}
	<-started
	close(release)
	for range callers {
		if err := <-errorsByCaller; err != nil {
			t.Fatalf("List() error = %v", err)
		}
	}
	if calls := source.callCount(); calls != 1 {
		t.Fatalf("source calls = %d, want 1", calls)
	}
}
