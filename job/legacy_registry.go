package job

import (
	"fmt"

	"gin-biz-web-api/pkg/config"
	"gin-biz-web-api/pkg/youzan"

	"github.com/hibiken/asynq"
)

type LegacyTaskDefinition struct {
	Code           string                 `json:"code"`
	Name           string                 `json:"name"`
	Category       string                 `json:"category"`
	SourceCode     string                 `json:"source_code"`
	SourceName     string                 `json:"source_name"`
	TaskType       string                 `json:"task_type"`
	Queue          string                 `json:"queue"`
	CronExpr       string                 `json:"cron_expr"`
	InputTable     string                 `json:"input_table"`
	OutputTable    string                 `json:"output_table"`
	TargetSystem   string                 `json:"target_system"`
	Handler        string                 `json:"handler"`
	Description    string                 `json:"description"`
	Editable       bool                   `json:"editable"`
	DefaultPayload map[string]interface{} `json:"default_payload"`
}

type LegacyTransformRuleDefinition struct {
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	SourceCode  string                 `json:"source_code"`
	SourceName  string                 `json:"source_name"`
	RuleType    string                 `json:"rule_type"`
	TriggerMode string                 `json:"trigger_mode"`
	InputTable  string                 `json:"input_table"`
	OutputTable string                 `json:"output_table"`
	Handler     string                 `json:"handler"`
	Description string                 `json:"description"`
	Editable    bool                   `json:"editable"`
	Config      map[string]interface{} `json:"config"`
	Steps       []string               `json:"steps"`
}

func IsStoppedLegacyTask(code string) bool {
	switch code {
	case "youzan_sales_push", "youzan_refund_push":
		return true
	default:
		return false
	}
}

func LegacyTaskDefinitions() []LegacyTaskDefinition {
	return []LegacyTaskDefinition{
		{
			Code:        "youzan_order_fetch",
			Name:        "有赞订单拉取",
			Category:    "fetch",
			SourceCode:  "youzan_order",
			SourceName:  "有赞订单",
			TaskType:    TypeYouzanSync,
			Queue:       YouzanSyncQueueName,
			CronExpr:    "",
			InputTable:  "有赞订单 API",
			OutputTable: "youzan_order_data",
			Handler:     "job/youzan_sync_job.go",
			Description: "从有赞拉取订单，默认拉取最近 5 分钟数据并写入 youzan_order_data；自动触发已停用，可手动执行。",
			Editable:    true,
			DefaultPayload: map[string]interface{}{
				"start_time": "",
				"end_time":   "",
			},
		},
		{
			Code:        "youzan_refund_fetch",
			Name:        "有赞退款拉取",
			Category:    "fetch",
			SourceCode:  "youzan_refund",
			SourceName:  "有赞退款",
			TaskType:    TypeYouzanReturn,
			Queue:       YouzanReturnQueueName,
			CronExpr:    "",
			InputTable:  "有赞退款 API",
			OutputTable: "youzan_return_data",
			Handler:     "job/youzan_return_job.go",
			Description: "从有赞拉取退款订单，默认使用 cfg.youzan.node_kdt_id；自动触发已停用，可手动执行。",
			Editable:    true,
			DefaultPayload: map[string]interface{}{
				"node_kdt_id": configInt64("cfg.youzan.node_kdt_id", 0),
			},
		},
		{
			Code:        "youzan_distribution_order_fetch",
			Name:        "有赞分销订单拉取",
			Category:    "fetch",
			SourceCode:  "youzan_distribution_order",
			SourceName:  "有赞分销订单",
			TaskType:    TypeYouzanDistributionOrderSync,
			Queue:       DefaultQueueName,
			CronExpr:    configString("cfg.youzan.distribution_cron_expr", "10 1 * * *"),
			InputTable:  "youzan.trades.sold.get/4.0.4",
			OutputTable: "youzan_distribution_orders",
			Handler:     "internal/service/data_svc/youzan_distribution_order_service.go",
			Description: "每天按下单时间拉取前一整天的有赞分销订单，所有非空 fans_nickname 解密成功后才写入独立分销订单表；手动补拉可选择下单时间或订单完成时间。",
			Editable:    true,
			DefaultPayload: map[string]interface{}{
				"time_filter": string(youzan.OrderTimeFilterCreated),
				"start_time":  "",
				"end_time":    "",
			},
		},
		{
			Code:        "bojun_order_fetch",
			Name:        "伯俊订单补拉",
			Category:    "fetch",
			SourceCode:  "bojun_order",
			SourceName:  "伯俊订单",
			TaskType:    TypeBojunOrderFetch,
			Queue:       DefaultQueueName,
			CronExpr:    "",
			InputTable:  "伯俊 middleretail.query",
			OutputTable: "raw_data / bojun_retail_orders",
			Handler:     "internal/service/data_svc/bojun_order_service.go",
			Description: "按开始时间和结束时间补拉伯俊订单；docno 已存在的数据不覆盖，未存在的数据写入并触发后续推送。",
			Editable:    true,
			DefaultPayload: map[string]interface{}{
				"start_time": "",
				"end_time":   "",
			},
		},
		{
			Code:         "youzan_sales_push",
			Name:         "有赞订单销售推送",
			Category:     "delivery",
			SourceCode:   "youzan_order",
			SourceName:   "有赞订单",
			TaskType:     TypeYouzanSalesSync,
			Queue:        YouzanReturnQueueName,
			CronExpr:     "",
			InputTable:   "youzan_order_data",
			OutputTable:  "杭州恒隆销售接口",
			TargetSystem: "杭州恒隆",
			Handler:      "job/youzan_sales_sync_job.go",
			Description:  "将未同步的有赞订单推送到杭州恒隆销售系统；自动触发已停用，可手动执行。",
			Editable:     true,
			DefaultPayload: map[string]interface{}{
				"node_kdt_id": configInt64("cfg.youzan.node_kdt_id", 0),
			},
		},
		{
			Code:         "youzan_refund_push",
			Name:         "有赞退款销售推送",
			Category:     "delivery",
			SourceCode:   "youzan_refund",
			SourceName:   "有赞退款",
			TaskType:     TypeYouzanRefundSync,
			Queue:        YouzanReturnQueueName,
			CronExpr:     "",
			InputTable:   "youzan_return_data",
			OutputTable:  "杭州恒隆销售接口",
			TargetSystem: "杭州恒隆",
			Handler:      "job/youzan_return_job.go",
			Description:  "将未同步的有赞退款单按退款销售类型推送到杭州恒隆销售系统；自动触发已停用，可手动执行。",
			Editable:     true,
			DefaultPayload: map[string]interface{}{
				"node_kdt_id": configInt64("cfg.youzan.node_kdt_id", 0),
			},
		},
		{
			Code:         "qimai_sales_push",
			Name:         "企迈订单销售推送",
			Category:     "delivery",
			SourceCode:   "qimai_order",
			SourceName:   "企迈订单",
			TaskType:     TypeSalesSync,
			Queue:        SalesSyncQueueName,
			CronExpr:     "@every 1m",
			InputTable:   "qimai_order_data",
			OutputTable:  "杭州恒隆销售接口",
			TargetSystem: "杭州恒隆",
			Handler:      "job/sales_sync_job.go",
			Description:  "将符合门店和状态条件的企迈订单推送到杭州恒隆销售系统。",
			Editable:     true,
			DefaultPayload: map[string]interface{}{
				"shop_code":      configString("cfg.henglong.sync.shop_code", ""),
				"status":         configString("cfg.henglong.sync.status", "70"),
				"store_code":     configString("cfg.henglong.sync.store_code", ""),
				"mall_item_code": configString("cfg.henglong.sync.mall_item_code", ""),
			},
		},
		{
			Code:         "xian_order_push",
			Name:         "西岸野选订单推送",
			Category:     "delivery",
			SourceCode:   "qimai_order",
			SourceName:   "企迈订单",
			TaskType:     TypeXianOrderSync,
			Queue:        XianOrderSyncQueueName,
			CronExpr:     "@every 1m",
			InputTable:   "qimai_order_data",
			OutputTable:  "西岸销售接口",
			TargetSystem: "西岸",
			Handler:      "job/xian_order_sync_job.go",
			Description:  "将符合西岸门店条件的企迈订单推送到西岸接口。",
			Editable:     true,
			DefaultPayload: map[string]interface{}{
				"shop_code": configString("cfg.xian.sync.shop_code", ""),
				"status":    configString("cfg.xian.sync.status", "70"),
			},
		},
		{
			Code:         "qimai_order_enrich",
			Name:         "企迈订单详情补数",
			Category:     "process",
			SourceCode:   "qimai_order",
			SourceName:   "企迈订单",
			TaskType:     TypeDataProcess,
			Queue:        DefaultQueueName,
			CronExpr:     "",
			InputTable:   "raw_data",
			OutputTable:  "qimai_order_data",
			TargetSystem: "企迈",
			Handler:      "Trigger/qimai_order_trigger.go",
			Description:  "原始数据 remark=qimai_order 时触发企迈订单详情查询并写入 qimai_order_data。",
			Editable:     true,
			DefaultPayload: map[string]interface{}{
				"raw_data_id": 0,
			},
		},
	}
}

func LegacyTransformRuleDefinitions() []LegacyTransformRuleDefinition {
	return []LegacyTransformRuleDefinition{
		{
			Code:        "qimai_order_http_enrich",
			Name:        "企迈订单详情补数清洗",
			SourceCode:  "qimai_order",
			SourceName:  "企迈订单",
			RuleType:    "http_enrich",
			TriggerMode: "data:process",
			InputTable:  "raw_data",
			OutputTable: "qimai_order_data",
			Handler:     "Trigger/qimai_order_trigger.go",
			Description: "接收 remark=qimai_order 的原始数据后，读取 params.orderNo，请求企迈订单详情接口，并把返回数据落到 qimai_order_data。",
			Editable:    true,
			Config: map[string]interface{}{
				"match_metadata": map[string]interface{}{"remark": "qimai_order"},
				"order_no_path":  "$.params.orderNo",
				"credential":     "token_data.account_name=野选.verification_info",
				"target_table":   "qimai_order_data",
			},
			Steps: []string{
				"解析 raw_data.metadata，remark 必须为 qimai_order",
				"解析 raw_data.raw_content.params.orderNo",
				"读取 token_data 中 account_name=野选 的 verification_info",
				"请求企迈 GetOrderDetail",
				"字段映射后写入 qimai_order_data",
			},
		},
		{
			Code:        "youzan_order_direct_store",
			Name:        "有赞订单直存清洗",
			SourceCode:  "youzan_order",
			SourceName:  "有赞订单",
			RuleType:    "script",
			TriggerMode: "youzan:sync",
			InputTable:  "有赞订单 API",
			OutputTable: "youzan_order_data",
			Handler:     "job/youzan_sync_job.go",
			Description: "有赞订单拉取后直接转换为 YOUZAN_ORDER_DATA 并写入 youzan_order_data。",
			Editable:    true,
			Config: map[string]interface{}{
				"target_table": "youzan_order_data",
				"converter":    "ConvertToModel",
				"sync_flag":    "synced=0",
			},
			Steps: []string{
				"获取有赞 access_token",
				"按时间窗口拉取订单",
				"ConvertToModel 转换字段",
				"写入 youzan_order_data",
			},
		},
		{
			Code:        "youzan_refund_direct_store",
			Name:        "有赞退款直存清洗",
			SourceCode:  "youzan_refund",
			SourceName:  "有赞退款",
			RuleType:    "script",
			TriggerMode: "youzan:return",
			InputTable:  "有赞退款 API",
			OutputTable: "youzan_return_data",
			Handler:     "job/youzan_return_job.go",
			Description: "有赞退款拉取后直接转换为 YOUZAN_RETURN_DATA 并写入 youzan_return_data。",
			Editable:    true,
			Config: map[string]interface{}{
				"target_table": "youzan_return_data",
				"converter":    "ConvertRefundToModel",
				"sync_flag":    "synced=0",
			},
			Steps: []string{
				"获取有赞 access_token",
				"按 node_kdt_id 拉取退款单",
				"ConvertRefundToModel 转换字段",
				"写入 youzan_return_data",
			},
		},
	}
}

func ScheduledLegacyTaskDefinitions() []LegacyTaskDefinition {
	definitions := LegacyTaskDefinitions()
	scheduled := make([]LegacyTaskDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if definition.CronExpr != "" {
			scheduled = append(scheduled, definition)
		}
	}
	return scheduled
}

func NewLegacyTask(code string, payload map[string]interface{}) (*asynq.Task, error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}

	switch code {
	case "youzan_order_fetch":
		return NewYouzanSyncTask(YouzanSyncPayload{
			StartTime: stringValue(payload, "start_time"),
			EndTime:   stringValue(payload, "end_time"),
		})
	case "youzan_refund_fetch":
		return NewYouzanReturnTask(YouzanReturnPayload{
			NodeKdtID: int64Value(payload, "node_kdt_id"),
		})
	case "youzan_distribution_order_fetch":
		return NewYouzanDistributionOrderSyncTask(YouzanDistributionOrderSyncPayload{
			TimeFilter: youzan.OrderTimeFilter(stringValue(payload, "time_filter")),
			StartTime:  stringValue(payload, "start_time"),
			EndTime:    stringValue(payload, "end_time"),
		})
	case "bojun_order_fetch":
		return NewBojunOrderFetchTask(BojunOrderFetchPayload{
			StartTime: stringValue(payload, "start_time"),
			EndTime:   stringValue(payload, "end_time"),
		})
	case "youzan_sales_push":
		return NewYouzanSalesSyncTask(YouzanSalesSyncPayload{
			NodeKdtID: int64Value(payload, "node_kdt_id"),
		})
	case "youzan_refund_push":
		return NewYouzanRefundSyncTask(YouzanRefundSyncPayload{
			NodeKdtID: int64Value(payload, "node_kdt_id"),
		})
	case "qimai_sales_push":
		return NewSalesSyncTask(SalesSyncPayload{
			ShopCode:     stringValue(payload, "shop_code"),
			Status:       stringValue(payload, "status"),
			StoreCode:    stringValue(payload, "store_code"),
			MallItemCode: stringValue(payload, "mall_item_code"),
		})
	case "xian_order_push":
		return NewXianOrderSyncTask(XianOrderSyncPayload{
			ShopCode: stringValue(payload, "shop_code"),
			Status:   stringValue(payload, "status"),
		})
	case "qimai_order_enrich":
		rawDataID := int64Value(payload, "raw_data_id")
		if rawDataID <= 0 {
			return nil, fmt.Errorf("legacy task %q requires raw_data_id", code)
		}
		return NewDataProcessTask(uint(rawDataID)), nil
	default:
		return nil, fmt.Errorf("unsupported legacy task %q", code)
	}
}

func stringValue(payload map[string]interface{}, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", value)
}

func int64Value(payload map[string]interface{}, key string) int64 {
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		var parsed int64
		_, _ = fmt.Sscan(fmt.Sprintf("%v", typed), &parsed)
		return parsed
	}
}

func configString(path, fallback string) string {
	if config.Instance() == nil {
		return fallback
	}
	return config.GetString(path, fallback)
}

func configInt64(path string, fallback int64) int64 {
	if config.Instance() == nil {
		return fallback
	}
	return config.GetInt64(path, fallback)
}
