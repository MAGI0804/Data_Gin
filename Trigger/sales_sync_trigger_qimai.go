package Trigger

import (
	"context"
	"fmt"
	"log"

	send "gin-biz-web-api/Trigger/Send_Data"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/pkg/config"
)

type SalesSyncTrigger struct {
	qimaiDataDAO *data_dao.QimaiDataDAO
}

func NewSalesSyncTrigger() *SalesSyncTrigger {
	return &SalesSyncTrigger{
		qimaiDataDAO: data_dao.NewQimaiDataDAO(),
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

	dingtalkToken := config.GetString("cfg.dingtalk.default.token")
	dingtalkSecret := config.GetString("cfg.dingtalk.default.secret")

	for _, order := range orders {
		tid := order.OrderNo
		payAmount := float64(order.ActualAmount) / 100.0

		log.Printf("处理订单: order_no=%s, actual_amount=%d, payAmount=%.2f", tid, order.ActualAmount, payAmount)

		response, err := send.SendSalesData(payAmount, tid, order.CompletedAt, storecode, mallitemcode, "SA")
		if err != nil {
			log.Printf("发送销售数据失败，订单号: %s, 错误: %v", tid, err)
			continue
		}

		log.Printf("发送销售数据成功，订单号: %s, 响应: %s", tid, response)

		if send.Contains(response, "responsecode>0<") || send.Contains(response, "上传成功") {
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

	return nil
}
