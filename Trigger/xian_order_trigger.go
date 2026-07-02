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

type XianOrderTrigger struct {
	qimaiDataDAO *data_dao.QimaiDataDAO
}

func NewXianOrderTrigger() *XianOrderTrigger {
	return &XianOrderTrigger{
		qimaiDataDAO: data_dao.NewQimaiDataDAO(),
	}
}

func (t *XianOrderTrigger) Trigger(ctx context.Context) error {
	shopCode := config.GetString("cfg.xian.sync.shop_code")
	status := config.GetString("cfg.xian.sync.status", "70")

	orders, err := t.qimaiDataDAO.FindByShopCodeAndStatus(ctx, shopCode, status)
	if err != nil {
		return fmt.Errorf("查询订单数据失败: %w", err)
	}

	log.Printf("查询到符合条件的订单数量: %d (shopCode=%s, status=%s)", len(orders), shopCode, status)

	url := config.GetString("cfg.xian.url")
	shopId := config.GetString("cfg.xian.shop_id")
	appSecret := config.GetString("cfg.xian.app_secret")
	shopName := config.GetString("cfg.xian.shop_name")

	dingtalkToken := config.GetString("cfg.dingtalk.xian.token")
	dingtalkSecret := config.GetString("cfg.dingtalk.xian.secret")

	for _, order := range orders {
		dealNo := order.OrderNo
		money := fmt.Sprintf("%.2f", float64(order.ActualAmount)/100.0)
		discountMoney := "0"
		receivableMoney := money
		goodsList := []string{}

		timeStr := ""
		if order.CompletedAt != nil {
			timeStr = order.CompletedAt.Format("2006-01-02 15:04:05")
		} else {
			timeStr = time.Now().Format("2006-01-02 15:04:05")
		}

		log.Printf("处理订单: order_no=%s, actual_amount=%d, money=%s", dealNo, order.ActualAmount, money)

		success, err := send.PostSale(url, shopId, appSecret, shopName, timeStr, dealNo, money, discountMoney, receivableMoney, goodsList)
		if err != nil {
			log.Printf("发送销售数据失败，订单号: %s, 错误: %v", dealNo, err)

			message := fmt.Sprintf("西岸野选数据推送失败\n订单号: %s\n金额: %s\n错误: %v", dealNo, money, err)
			send.SendDingTalkMessage(dingtalkToken, dingtalkSecret, message)
			continue
		}

		if success {
			log.Printf("西岸野选销售数据上传成功，订单号: %s", dealNo)

			message := fmt.Sprintf("西岸野选数据推送成功，金额为：%s，订单号为：%s，时间为：%s", money, dealNo, timeStr)
			send.SendDingTalkMessage(dingtalkToken, dingtalkSecret, message)
		} else {
			log.Printf("西岸野选销售数据上传失败，订单号: %s", dealNo)

			message := fmt.Sprintf("西岸野选数据推送失败，金额为：%s，订单号为：%s，时间为：%s", money, dealNo, timeStr)
			send.SendDingTalkMessage(dingtalkToken, dingtalkSecret, message)
		}

		if err := t.qimaiDataDAO.MarkAsSynced(ctx, order.ID); err != nil {
			log.Printf("标记订单已同步失败，订单号: %s, 错误: %v", dealNo, err)
		} else {
			log.Printf("标记订单已同步成功，订单号: %s", dealNo)
		}
	}

	return nil
}
