package data_svc

import (
	"testing"

	"gin-biz-web-api/model"
)

func TestBuildGeneratedStepConfigBuildsRequestSectionsAndCaptures(t *testing.T) {
	config, err := BuildGeneratedStepConfigMap(MethodStepDefinition{
		Step: model.MethodStep{Code: "fetch_orders", MethodType: "request", TimeoutSeconds: 15},
		Params: []model.MethodParam{
			{Location: "request", Name: "method", ValueSource: "static", Value: "post"},
			{Location: "url", Name: "url", ValueSource: "config", Value: "cfg.youzan.orders_url"},
			{Location: "query", Name: "access_token", ValueSource: "binding", Value: "steps.get_token.outputs.access_token", Required: true, Secret: true},
			{Location: "header", Name: "Content-Type", ValueSource: "static", Value: "application/json"},
			{Location: "body", Name: "page_size", ValueSource: "static", Value: "100", ValueType: "int"},
			{Location: "response", Name: "records_path", ValueSource: "static", Value: "data.full_order_info_list"},
		},
		Outputs: []model.MethodOutput{
			{Name: "records", SourcePath: "data.full_order_info_list", ValueType: "array", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildGeneratedStepConfigMap returned error: %v", err)
	}

	if config["method"] != "POST" {
		t.Fatalf("method = %v, want POST", config["method"])
	}
	if config["records_path"] != "data.full_order_info_list" {
		t.Fatalf("records_path = %v", config["records_path"])
	}

	queryParams := config["query_params"].([]map[string]interface{})
	tokenValue := queryParams[0]["value"].(map[string]interface{})
	if tokenValue["path"] != "steps.get_token.outputs.access_token" {
		t.Fatalf("binding path = %v", tokenValue["path"])
	}

	bodyParams := config["body_params"].([]map[string]interface{})
	if bodyParams[0]["value"] != int64(100) {
		t.Fatalf("body page_size = %#v, want int64(100)", bodyParams[0]["value"])
	}

	captures := config["captures"].([]map[string]interface{})
	if captures[0]["name"] != "records" {
		t.Fatalf("capture name = %v", captures[0]["name"])
	}
}

func TestBuildGeneratedStepConfigRejectsInvalidBinding(t *testing.T) {
	_, err := BuildGeneratedStepConfigMap(MethodStepDefinition{
		Step: model.MethodStep{Code: "fetch_orders", MethodType: "request"},
		Params: []model.MethodParam{
			{Location: "url", Name: "url", ValueSource: "static", Value: "https://example.com"},
			{Location: "query", Name: "access_token", ValueSource: "binding", Value: "get_token.access_token"},
		},
	})
	if err == nil {
		t.Fatal("expected invalid binding error")
	}
}

func TestBuildGeneratedStepConfigBuildsMappingFields(t *testing.T) {
	config, err := BuildGeneratedStepConfigMap(MethodStepDefinition{
		Step: model.MethodStep{Code: "map_order", MethodType: "mapping"},
		Params: []model.MethodParam{
			{Location: "mapping", Name: "table_name", ValueSource: "static", Value: "clean_orders"},
			{Location: "mapping", Name: "business_key_field", ValueSource: "static", Value: "order_no"},
			{Location: "field", Name: "order_no", ValueSource: "static", Value: "$.tid", ValueType: "string", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildGeneratedStepConfigMap returned error: %v", err)
	}
	if config["table_name"] != "clean_orders" {
		t.Fatalf("table_name = %v", config["table_name"])
	}
	fields := config["fields"].([]map[string]interface{})
	if fields[0]["source_path"] != "$.tid" {
		t.Fatalf("field source_path = %v", fields[0]["source_path"])
	}
}

func TestBuildGeneratedStepConfigBuildsDeliveryAsSeparatedFields(t *testing.T) {
	config, err := BuildGeneratedStepConfigMap(MethodStepDefinition{
		Step: model.MethodStep{Code: "push_sales", MethodType: "delivery"},
		Params: []model.MethodParam{
			{Location: "url", Name: "url", ValueSource: "config", Value: "cfg.henglong.sales_url"},
			{Location: "request", Name: "method", ValueSource: "static", Value: "POST"},
			{Location: "header", Name: "Content-Type", ValueSource: "static", Value: "application/json"},
			{Location: "body", Name: "order_no", ValueSource: "binding", Value: "steps.map_order.outputs.order_no"},
		},
		Outputs: []model.MethodOutput{
			{Name: "http_status", SourcePath: "http_status", ValueType: "int", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildGeneratedStepConfigMap returned error: %v", err)
	}

	headers := config["headers"].([]map[string]interface{})
	if headers[0]["name"] != "Content-Type" {
		t.Fatalf("header name = %v", headers[0]["name"])
	}
	bodyParams := config["body_params"].([]map[string]interface{})
	bodyValue := bodyParams[0]["value"].(map[string]interface{})
	if bodyValue["path"] != "steps.map_order.outputs.order_no" {
		t.Fatalf("body binding = %v", bodyValue["path"])
	}
}
