package job

import (
	"fmt"

	"gin-biz-web-api/pkg/config"

	"github.com/hibiken/asynq"
)

type LegacyTaskDefinition struct {
	Code           string                 `json:"code"`
	Name           string                 `json:"name"`
	Category       string                 `json:"category"`
	TaskType       string                 `json:"task_type"`
	Queue          string                 `json:"queue"`
	CronExpr       string                 `json:"cron_expr"`
	Description    string                 `json:"description"`
	DefaultPayload map[string]interface{} `json:"default_payload"`
}

func LegacyTaskDefinitions() []LegacyTaskDefinition {
	return []LegacyTaskDefinition{
		{
			Code:        "youzan_order_fetch",
			Name:        "有赞订单拉取",
			Category:    "fetch",
			TaskType:    TypeYouzanSync,
			Queue:       YouzanSyncQueueName,
			CronExpr:    "@every 1m",
			Description: "从有赞拉取订单，默认拉取最近 5 分钟数据并写入 youzan_order_data。",
			DefaultPayload: map[string]interface{}{
				"start_time": "",
				"end_time":   "",
			},
		},
		{
			Code:        "youzan_refund_fetch",
			Name:        "有赞退款拉取",
			Category:    "fetch",
			TaskType:    TypeYouzanReturn,
			Queue:       YouzanReturnQueueName,
			CronExpr:    "@every 1m",
			Description: "从有赞拉取退款订单，默认使用 cfg.youzan.node_kdt_id。",
			DefaultPayload: map[string]interface{}{
				"node_kdt_id": configInt64("cfg.youzan.node_kdt_id", 0),
			},
		},
		{
			Code:        "youzan_sales_push",
			Name:        "有赞订单销售推送",
			Category:    "delivery",
			TaskType:    TypeYouzanSalesSync,
			Queue:       YouzanReturnQueueName,
			CronExpr:    "@every 1m",
			Description: "将未同步的有赞订单推送到杭州恒隆销售系统。",
			DefaultPayload: map[string]interface{}{
				"node_kdt_id": configInt64("cfg.youzan.node_kdt_id", 0),
			},
		},
		{
			Code:        "youzan_refund_push",
			Name:        "有赞退款销售推送",
			Category:    "delivery",
			TaskType:    TypeYouzanRefundSync,
			Queue:       YouzanReturnQueueName,
			CronExpr:    "@every 1m",
			Description: "将未同步的有赞退款单按退款销售类型推送到杭州恒隆销售系统。",
			DefaultPayload: map[string]interface{}{
				"node_kdt_id": configInt64("cfg.youzan.node_kdt_id", 0),
			},
		},
		{
			Code:        "qimai_sales_push",
			Name:        "企迈订单销售推送",
			Category:    "delivery",
			TaskType:    TypeSalesSync,
			Queue:       SalesSyncQueueName,
			CronExpr:    "@every 1m",
			Description: "将符合门店和状态条件的企迈订单推送到杭州恒隆销售系统。",
			DefaultPayload: map[string]interface{}{
				"shop_code":      configString("cfg.henglong.sync.shop_code", ""),
				"status":         configString("cfg.henglong.sync.status", "70"),
				"store_code":     configString("cfg.henglong.sync.store_code", ""),
				"mall_item_code": configString("cfg.henglong.sync.mall_item_code", ""),
			},
		},
		{
			Code:        "xian_order_push",
			Name:        "西岸野选订单推送",
			Category:    "delivery",
			TaskType:    TypeXianOrderSync,
			Queue:       XianOrderSyncQueueName,
			CronExpr:    "@every 1m",
			Description: "将符合西岸门店条件的企迈订单推送到西岸接口。",
			DefaultPayload: map[string]interface{}{
				"shop_code": configString("cfg.xian.sync.shop_code", ""),
				"status":    configString("cfg.xian.sync.status", "70"),
			},
		},
		{
			Code:        "qimai_order_enrich",
			Name:        "企迈订单详情补数",
			Category:    "process",
			TaskType:    TypeDataProcess,
			Queue:       DefaultQueueName,
			CronExpr:    "",
			Description: "原始数据 remark=qimai_order 时触发企迈订单详情查询并写入 qimai_order_data。",
			DefaultPayload: map[string]interface{}{
				"raw_data_id": 0,
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
