package Trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"gin-biz-web-api/internal/dao/auth_dao"
	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/qimai"
)

// QimaiOrderTrigger 企迈订单触发器
type QimaiOrderTrigger struct {
	tokenDataDAO *auth_dao.TokenDataDAO
	rawDataDAO   *data_dao.RawDataDAO
	qimaiDataDAO *data_dao.QimaiDataDAO
}

// NewQimaiOrderTrigger 创建企迈订单触发器实例
func NewQimaiOrderTrigger() *QimaiOrderTrigger {
	return &QimaiOrderTrigger{
		tokenDataDAO: auth_dao.NewTokenDataDAO(),
		rawDataDAO:   data_dao.NewRawDataDAO(),
		qimaiDataDAO: data_dao.NewQimaiDataDAO(),
	}
}

// Trigger 触发企迈订单处理
func (t *QimaiOrderTrigger) Trigger(ctx context.Context, rawData *model.RawData) error {
	// 1. 检查remark是否为qimai_order
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(rawData.Metadata), &metadata); err != nil {
		return fmt.Errorf("解析元数据失败: %w", err)
	}

	remark, ok := metadata["remark"].(string)
	if !ok || remark != "qimai_order" {
		return nil // 不是企迈订单，不处理
	}

	// 2. 解析原始数据，提取orderNo
	var rawContent map[string]interface{}
	if err := json.Unmarshal([]byte(rawData.RawContent), &rawContent); err != nil {
		return fmt.Errorf("解析原始数据失败: %w", err)
	}

	params, ok := rawContent["params"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("原始数据格式错误，缺少params字段")
	}

	orderNo, ok := params["orderNo"].(string)
	if !ok {
		return fmt.Errorf("原始数据格式错误，缺少orderNo字段")
	}

	// 3. 从token_data表中获取account_name为野选的verification_info
	tokenDataList, err := t.tokenDataDAO.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("查询token数据失败: %w", err)
	}

	log.Printf("查询到token数据数量: %d", len(tokenDataList))

	var verificationInfo map[string]string
	for _, tokenData := range tokenDataList {
		log.Printf("token数据: account_name=%s, verification_info类型=%T, verification_info值=%v", tokenData.AccountName, tokenData.VerificationInfo, tokenData.VerificationInfo)
		if tokenData.AccountName == "野选" {
			log.Printf("找到account_name为野选的token数据")
			// 解析verification_info
			// 处理指针类型
			viValue := tokenData.VerificationInfo
			// 解引用指针，直到得到实际值
			for {
				if ptr, ok := viValue.(*interface{}); ok {
					log.Printf("解引用一级指针")
					viValue = *ptr
				} else if ptr2, ok := viValue.(**interface{}); ok {
					log.Printf("解引用二级指针")
					viValue = **ptr2
				} else {
					break
				}
			}
			log.Printf("解引用后verification_info类型=%T, 值=%v", viValue, viValue)

			// 处理[]uint8类型（字节切片，GORM读取TEXT类型的结果）
			if viBytes, ok := viValue.([]uint8); ok {
				log.Printf("verification_info是[]uint8类型，转换为string")
				viValue = string(viBytes)
				log.Printf("转换后verification_info类型=%T, 值=%v", viValue, viValue)
			}

			if viJSON, ok := viValue.(string); ok {
				log.Printf("verification_info是string类型: %s", viJSON)
				if err := json.Unmarshal([]byte(viJSON), &verificationInfo); err != nil {
					log.Printf("解析verification_info失败: %v", err)
					return fmt.Errorf("解析verification_info失败: %w", err)
				}
				log.Printf("解析verification_info成功: %v", verificationInfo)
				break
			} else if viMap, ok := viValue.(map[string]interface{}); ok {
				log.Printf("verification_info是map类型: %v", viMap)
				verificationInfo = make(map[string]string)
				for k, v := range viMap {
					if vStr, ok := v.(string); ok {
						verificationInfo[k] = vStr
					}
				}
				log.Printf("解析verification_info成功: %v", verificationInfo)
				break
			} else {
				log.Printf("verification_info类型不支持: %T", viValue)
			}
		}
	}

	if verificationInfo == nil {
		return fmt.Errorf("未找到account_name为野选的token数据")
	}

	// 4. 验证token并获取值
	openId := verificationInfo["openId"]
	grantCode := verificationInfo["grantCode"]
	openKey := verificationInfo["open_key"]
	nonce := verificationInfo["nonce"]

	// 5. 调用GetOrderDetail获取订单详情
	orderDetail, err := qimai.GetOrderDetail(openId, grantCode, openKey, nonce, orderNo, 7)
	if err != nil {
		return fmt.Errorf("获取订单详情失败: %w", err)
	}

	// 6. 将订单详情存入qimai_order_data表
	orderData := &model.QIMAI_ORDER_DATA{
		RawDataID: rawData.ID,
		OrderNo:   orderNo,
		Status:    "processed",
	}

	// 映射API返回的数据到模型
	if data, ok := orderDetail["data"].(map[string]interface{}); ok {
		if v, ok := data["orderType"].(float64); ok {
			orderData.OrderType = int(v)
		}
		if v, ok := data["source"].(float64); ok {
			orderData.Source = int(v)
		}
		if v, ok := data["userId"].(string); ok {
			orderData.UserID = v
		}
		if v, ok := data["multiStoreId"].(string); ok {
			orderData.MultiStoreID = v
		}
		if v, ok := data["scene"].(string); ok {
			orderData.Scene = v
		}
		if v, ok := data["status"].(string); ok {
			orderData.Status = v
		}
		if v, ok := data["orderNo"].(string); ok {
			orderData.OrderNo = v
		}
		if v, ok := data["payNo"].(string); ok {
			orderData.PayNo = v
		}
		if v, ok := data["thirdPayNo"].(string); ok {
			orderData.ThirdPayNo = v
		}
		if v, ok := data["storeOrderNo"].(string); ok {
			orderData.StoreOrderNo = v
		}
		if v, ok := data["totalAmount"].(string); ok {
			if amount, err := strconv.Atoi(v); err == nil {
				orderData.TotalAmount = amount
			}
		}
		if v, ok := data["actualAmount"].(string); ok {
			if amount, err := strconv.Atoi(v); err == nil {
				orderData.ActualAmount = amount
			}
		}
		if v, ok := data["discountAmount"].(string); ok {
			if amount, err := strconv.Atoi(v); err == nil {
				orderData.DiscountAmount = amount
			}
		}
		if v, ok := data["itemAmount"].(string); ok {
			if amount, err := strconv.Atoi(v); err == nil {
				orderData.ItemAmount = amount
			}
		}
		if v, ok := data["packAmount"].(string); ok {
			if amount, err := strconv.Atoi(v); err == nil {
				orderData.PackAmount = amount
			}
		}
		if v, ok := data["freightAmount"].(string); ok {
			if amount, err := strconv.Atoi(v); err == nil {
				orderData.FreightAmount = amount
			}
		}
		if v, ok := data["buyerRemarks"].(string); ok {
			orderData.BuyerRemarks = v
		}
		if v, ok := data["shopCode"].(string); ok {
			orderData.ShopCode = v
		}
		if v, ok := data["shopName"].(string); ok {
			orderData.ShopName = v
		}
		if v, ok := data["contactTel"].(string); ok {
			orderData.ContactTel = v
		}
		if v, ok := data["contactName"].(string); ok {
			orderData.ContactName = v
		}
	}

	_, err = t.qimaiDataDAO.Create(ctx, orderData)
	if err != nil {
		return fmt.Errorf("存储订单数据失败: %w", err)
	}

	log.Printf("企迈订单处理成功，订单号: %s", orderNo)
	return nil
}
