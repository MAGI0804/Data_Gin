package config

import (
	"gin-biz-web-api/pkg/config"
)

func init() {
	config.Add("cfg.youzan", func() map[string]interface{} {
		return map[string]interface{}{
			"client_id":     config.Get("Youzan.ClientID", ""),
			"client_secret": config.Get("Youzan.ClientSecret", ""),
			"grant_id":      config.Get("Youzan.GrantID", ""),

			"token_url":      config.Get("Youzan.TokenURL", "https://open.youzanyun.com/auth/token"),
			"orders_url":     config.Get("Youzan.OrdersURL", "https://open.youzanyun.com/api/youzan.trades.sold.get/4.0.4"),
			"decrypt_url":    config.Get("Youzan.DecryptURL", "https://open.youzanyun.com/api/youzan.cloud.secret.decrypt.batch/1.0.0"),
			"value_card_url": config.Get("Youzan.ValueCardURL", "https://open.youzanyun.com/api/youzan.cardvoucher.valuecard.pay.rcd.bypub.search/3.0.1"),
			"refund_url":     config.Get("Youzan.RefundURL", "https://open.youzanyun.com/api/youzan.trade.refund.search/3.0.1"),

			"node_kdt_id": config.GetInt64("Youzan.NodeKdtID", 0),
		}
	})
}
