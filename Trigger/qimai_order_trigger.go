package Trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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

	var verificationInfo map[string]string
	for _, tokenData := range tokenDataList {
		if tokenData.AccountName == "野选" {
			// 解析verification_info
			if viJSON, ok := tokenData.VerificationInfo.(string); ok {
				if err := json.Unmarshal([]byte(viJSON), &verificationInfo); err != nil {
					return fmt.Errorf("解析verification_info失败: %w", err)
				}
				break
			} else if viMap, ok := tokenData.VerificationInfo.(map[string]interface{}); ok {
				verificationInfo = make(map[string]string)
				for k, v := range viMap {
					if vStr, ok := v.(string); ok {
						verificationInfo[k] = vStr
					}
				}
				break
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

	// 6. 将订单详情存入qimai_data表
	qimaiData := &model.QimaiData{
		RawDataID:    rawData.ID,
		OrderNo:      orderNo,
		OrderDetails: fmt.Sprintf("%v", orderDetail),
		Status:       "processed",
	}

	_, err = t.qimaiDataDAO.Create(ctx, qimaiData)
	if err != nil {
		return fmt.Errorf("存储订单数据失败: %w", err)
	}

	log.Printf("企迈订单处理成功，订单号: %s", orderNo)
	return nil
}
