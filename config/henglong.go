package config

import (
	"gin-biz-web-api/pkg/config"
)

func init() {
	config.Add("cfg.henglong", func() map[string]interface{} {
		return map[string]interface{}{
			"sales_api_url": config.Get("HengLong.SalesAPIURL", "https://wlkpos.hanglung.com.cn:8280/HLD/salestrans.asmx"),
			"username":      config.Get("HengLong.Username", ""),
			"password":      config.Get("HengLong.Password", ""),
			"license_key":   config.Get("HengLong.LicenseKey", ""),

			"store_code":     config.Get("HengLong.StoreCode", ""),
			"till_id":        config.Get("HengLong.TillID", "01"),
			"mall_item_code": config.Get("HengLong.MallItemCode", ""),
			"mall_id":        config.Get("HengLong.MallID", ""),
			"cashier":        config.Get("HengLong.Cashier", ""),
			"plu_code":       config.Get("HengLong.PLUCode", ""),

			"refund_store_code":      config.Get("HengLong.RefundStoreCode", "416201"),
			"refund_mall_item_code":  config.Get("HengLong.RefundMallItemCode", "E6600000099"),

			"sync": map[string]interface{}{
				"shop_code":      config.Get("HengLong.Sync.ShopCode", ""),
				"status":         config.Get("HengLong.Sync.Status", "70"),
				"store_code":     config.Get("HengLong.Sync.StoreCode", ""),
				"mall_item_code": config.Get("HengLong.Sync.MallItemCode", ""),
			},
		}
	})
}
