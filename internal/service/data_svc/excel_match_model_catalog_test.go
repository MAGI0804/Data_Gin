package data_svc

import (
	"strings"
	"testing"

	"gin-biz-web-api/internal/dao/data_dao"
)

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
