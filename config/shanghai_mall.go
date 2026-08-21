package config

import "gin-biz-web-api/pkg/config"

func init() {
	config.Add("cfg.shanghai_mall", func() map[string]interface{} {
		return map[string]interface{}{
			"timeout_seconds": config.GetInt("ShanghaiMall.TimeoutSeconds", 10),
			"syandata": map[string]interface{}{
				"url": config.Get("ShanghaiMall.Syandata.URL", "http://api.syandata.com/oapi/rest"),
			},
			"jialicheng": map[string]interface{}{
				"login_url": config.Get("ShanghaiMall.Jialicheng.LoginURL", ""),
				"sales_url": config.Get("ShanghaiMall.Jialicheng.SalesURL", ""),
			},
			"qiantan": map[string]interface{}{
				"token_url": config.Get("ShanghaiMall.Qiantan.TokenURL", ""),
				"post_url":  config.Get("ShanghaiMall.Qiantan.PostURL", ""),
			},
			"shangsheng": map[string]interface{}{
				"url": config.Get("ShanghaiMall.Shangsheng.URL", ""),
			},
			"xinjia_center": map[string]interface{}{
				"url":              config.Get("ShanghaiMall.XinjiaCenter.URL", ""),
				"product_code":     config.Get("ShanghaiMall.XinjiaCenter.ProductCode", ""),
				"store_code":       config.Get("ShanghaiMall.XinjiaCenter.StoreCode", ""),
				"bojun_store_code": config.Get("ShanghaiMall.XinjiaCenter.BojunStoreCode", ""),
			},
		}
	})
}
