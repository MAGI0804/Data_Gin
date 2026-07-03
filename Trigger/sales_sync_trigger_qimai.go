package Trigger

import (
	"context"
	"fmt"
	"log"
	"time"

	send "gin-biz-web-api/Trigger/Send_Data"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/config"

	"github.com/google/uuid"
)

type SalesSyncTrigger struct {
	qimaiDataDAO   *data_dao.QimaiDataDAO
	logDAO         *data_dao.DeliveryLogDAO
	pipelineRunDAO *data_dao.PipelineRunDAO
}

func NewSalesSyncTrigger() *SalesSyncTrigger {
	return &SalesSyncTrigger{
		qimaiDataDAO:   data_dao.NewQimaiDataDAO(),
		logDAO:         data_dao.NewDeliveryLogDAO(),
		pipelineRunDAO: data_dao.NewPipelineRunDAO(),
	}
}

func (t *SalesSyncTrigger) Trigger(ctx context.Context) error {
	shopCode := config.GetString("cfg.henglong.sync.shop_code")
	status := config.GetString("cfg.henglong.sync.status", "70")
	storecode := config.GetString("cfg.henglong.sync.store_code")
	mallitemcode := config.GetString("cfg.henglong.sync.mall_item_code")
	return t.TriggerWithParams(ctx, shopCode, status, storecode, mallitemcode)
}

func (t *SalesSyncTrigger) TriggerWithParams(ctx context.Context, shopCode, status, storecode, mallitemcode string) error {
	orders, err := t.qimaiDataDAO.FindByShopCodeAndStatus(ctx, shopCode, status)
	if err != nil {
		return fmt.Errorf("查询订单数据失败: %w", err)
	}

	log.Printf("查询到符合条件的订单数量: %d (shopCode=%s, status=%s)", len(orders), shopCode, status)
	traceID := uuid.NewString()
	runID, err := t.pipelineRunDAO.Create(ctx, &model.PipelineRun{
		TraceID:      traceID,
		RunType:      "delivery",
		TriggerType:  "schedule",
		Status:       "running",
		TotalCount:   len(orders),
		StartedAt:    &model.TimeNormal{Time: time.Now()},
		ErrorMessage: fmt.Sprintf("qimai_order -> 杭州恒隆 shopCode=%s status=%s", shopCode, status),
	})
	if err != nil {
		return fmt.Errorf("创建企迈推送运行日志失败: %w", err)
	}

	dingtalkToken := config.GetString("cfg.dingtalk.default.token")
	dingtalkSecret := config.GetString("cfg.dingtalk.default.secret")

	successCount := 0
	failedCount := 0
	for _, order := range orders {
		tid := order.OrderNo
		payAmount := float64(order.ActualAmount) / 100.0

		log.Printf("处理订单: order_no=%s, actual_amount=%d, payAmount=%.2f", tid, order.ActualAmount, payAmount)

		result, err := send.SendSalesDataWithResult(payAmount, tid, order.CompletedAt, storecode, mallitemcode, "SA")
		response := ""
		if result != nil {
			response = result.ResponseBody
		}
		if err != nil {
			log.Printf("发送销售数据失败，订单号: %s, 错误: %v", tid, err)
			t.writeQimaiDeliveryLog(ctx, traceID, runID, order.ID, tid, result, err)
			failedCount++
			continue
		}
		t.writeQimaiDeliveryLog(ctx, traceID, runID, order.ID, tid, result, nil)

		log.Printf("发送销售数据成功，订单号: %s, 响应: %s", tid, response)

		if result != nil && result.Success {
			successCount++
			log.Printf("企迈销售数据上传成功，订单号: %s", tid)

			message := fmt.Sprintf("企迈销售数据上传成功\n日期: %s\n交易ID: %s\n金额: %.2f",
				send.GetYesterdayDate(), tid, payAmount)

			dingResp, err := send.SendDingTalkMessage(dingtalkToken, dingtalkSecret, message)
			if err != nil {
				log.Printf("发送钉钉消息失败: %v", err)
			} else {
				log.Printf("钉钉消息发送成功: %s", dingResp)
			}
		} else {
			failedCount++
			log.Printf("企迈销售数据上传失败，订单号: %s, 响应: %s", tid, response)

			message := fmt.Sprintf("企迈销售数据上传失败\n日期: %s\n交易ID: %s\n响应: %s",
				send.GetYesterdayDate(), tid, response)

			send.SendDingTalkMessage(dingtalkToken, dingtalkSecret, message)
		}

		if err := t.qimaiDataDAO.MarkAsSynced(ctx, order.ID); err != nil {
			log.Printf("标记订单已同步失败，订单号: %s, 错误: %v", tid, err)
		} else {
			log.Printf("标记订单已同步成功，订单号: %s", tid)
		}
	}

	statusResult := "success"
	if failedCount > 0 && successCount > 0 {
		statusResult = "partial_success"
	} else if failedCount > 0 {
		statusResult = "failed"
	}
	return t.pipelineRunDAO.Finish(ctx, runID, statusResult, successCount, failedCount, "")
}

func (t *SalesSyncTrigger) writeQimaiDeliveryLog(
	ctx context.Context,
	traceID string,
	runID uint,
	cleanRecordID uint,
	businessKey string,
	result *send.SalesDataResult,
	pushErr error,
) {
	logItem := &model.DeliveryLog{
		TraceID:         traceID,
		RunID:           runID,
		CleanRecordID:   cleanRecordID,
		DestinationID:   0,
		SourceCode:      "qimai_order",
		DestinationCode: "hangzhou_henglong",
		DestinationName: "杭州恒隆",
		BusinessKey:     businessKey,
		SentAt:          &model.TimeNormal{Time: time.Now()},
	}
	if result != nil {
		logItem.RequestBody = result.RequestBody
		logItem.ResponseBody = result.ResponseBody
		logItem.HTTPStatus = result.HTTPStatus
		logItem.Success = result.Success
	}
	if pushErr != nil {
		logItem.Success = false
		logItem.ErrorMessage = pushErr.Error()
	}
	_, _ = t.logDAO.Create(ctx, logItem)
}
