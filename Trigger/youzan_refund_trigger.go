package Trigger

import (
	"context"
	"fmt"
	"log"
	"time"

	send "gin-biz-web-api/Trigger/Send_Data"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/pkg/config"
)

type YouzanRefundTrigger struct {
	returnDAO *data_dao.YouzanReturnDAO
}

func NewYouzanRefundTrigger() *YouzanRefundTrigger {
	return &YouzanRefundTrigger{
		returnDAO: data_dao.NewYouzanReturnDAO(),
	}
}

func (t *YouzanRefundTrigger) Trigger(ctx context.Context) error {
	nodeKdtID := config.GetInt64("cfg.youzan.node_kdt_id")
	return t.TriggerWithParams(ctx, nodeKdtID, "SUCCESS")
}

func (t *YouzanRefundTrigger) TriggerWithParams(ctx context.Context, nodeKdtID int64, status string) error {
	if nodeKdtID == 0 {
		nodeKdtID = config.GetInt64("cfg.youzan.node_kdt_id")
	}

	orders, err := t.returnDAO.FindUnsyncedByNodeKdtIDAndStatus(ctx, nodeKdtID, status)
	if err != nil {
		return fmt.Errorf("查询退款订单数据失败: %w", err)
	}

	log.Printf("查询到符合条件的退款订单数量: %d (nodeKdtID=%d, status=%s)", len(orders), nodeKdtID, status)

	storecode := config.GetString("cfg.henglong.refund_store_code", "416201")
	mallitemcode := config.GetString("cfg.henglong.refund_mall_item_code", "E6600000099")
	salestype := "SR"

	dingtalkToken := config.GetString("cfg.dingtalk.default.token")
	dingtalkSecret := config.GetString("cfg.dingtalk.default.secret")

	for _, order := range orders {
		tid := order.RefundID
		payAmount := -order.RefundFee

		log.Printf("处理退款订单: refund_id=%s, refund_fee=%.2f, payAmount=%.2f", tid, order.RefundFee, payAmount)

		var completedAt *time.Time
		if order.Modified != nil {
			completedAt = order.Modified
		}

		response, err := send.SendSalesData(payAmount, tid, completedAt, storecode, mallitemcode, salestype)
		if err != nil {
			log.Printf("发送退款数据失败，退款ID: %s, 错误: %v", tid, err)
			continue
		}

		log.Printf("发送退款数据成功，退款ID: %s, 响应: %s", tid, response)

		uploadTime := time.Now().Format("2006-01-02 15:04:05")

		if send.Contains(response, "responsecode>0<") || send.Contains(response, "上传成功") {
			log.Printf("有赞退款数据上传成功，退款ID: %s", tid)

			message := fmt.Sprintf("上传幼岚杭州恒隆销售（退款）数据\n订单号: %s\n金额: %.2f\n上传时间: %s",
				tid, order.RefundFee, uploadTime)

			dingResp, err := send.SendDingTalkMessage(dingtalkToken, dingtalkSecret, message)
			if err != nil {
				log.Printf("发送钉钉消息失败: %v", err)
			} else {
				log.Printf("钉钉消息发送成功: %s", dingResp)
			}
		} else {
			log.Printf("有赞退款数据上传失败，退款ID: %s, 响应: %s", tid, response)

			message := fmt.Sprintf("上传幼岚杭州恒隆销售（退款）数据失败\n订单号: %s\n金额: %.2f\n上传时间: %s\n响应: %s",
				tid, order.RefundFee, uploadTime, response)

			send.SendDingTalkMessage(dingtalkToken, dingtalkSecret, message)
		}

		if err := t.returnDAO.MarkAsSynced(ctx, order.ID); err != nil {
			log.Printf("标记退款订单已同步失败，退款ID: %s, 错误: %v", tid, err)
		} else {
			log.Printf("标记退款订单已同步成功，退款ID: %s", tid)
		}
	}

	return nil
}
