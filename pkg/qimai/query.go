package qimai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// OrderParams 订单参数
type OrderParams struct {
	BizType int    `json:"bizType"`
	OrderNo string `json:"orderNo"`
}

// BusinessRecordParams 营业额记录参数
type BusinessRecordParams struct {
	EndDate   string `json:"end_date"`
	StartDate string `json:"start_date"`
	ShopCode  string `json:"shopCode"`
}

// RequestBody 请求体
type RequestBody struct {
	OpenId    string      `json:"openId"`
	GrantCode string      `json:"grantCode"`
	Nonce     string      `json:"nonce"`
	Timestamp int64       `json:"timestamp"`
	Token     string      `json:"token"`
	Params    interface{} `json:"params"`
}

// GetOrderDetail 获取订单详情
func GetOrderDetail(openId, grantCode, openKey, nonce, orderNo string, bizType int) (map[string]interface{}, error) {
	url := "https://openapi.qmai.cn/v3/order/getDetail"
	timestamp := time.Now().Unix()

	// 构建token参数
	param := map[string]string{
		"openId":    openId,
		"grantCode": grantCode,
		"timestamp": strconv.FormatInt(timestamp, 10),
		"nonce":     "11886",
	}

	// 生成token
	token := GenerateToken(param, openKey)

	// 构建请求体
	body := RequestBody{
		OpenId:    openId,
		GrantCode: grantCode,
		Nonce:     nonce,
		Timestamp: timestamp,
		Token:     token,
		Params: OrderParams{
			BizType: bizType,
			OrderNo: orderNo,
		},
	}

	// 发送请求
	return sendRequest(url, body)
}

// GetBusinessRecord 获取营业额记录
func GetBusinessRecord(openId, grantCode, openKey, nonce, shopCode, startDate, endDate string) (map[string]interface{}, error) {
	url := "https://openapi.qmai.cn/v3/dataone/finance/summary/businessRecord"
	timestamp := time.Now().Unix()

	// 构建token参数
	param := map[string]string{
		"openId":    openId,
		"grantCode": grantCode,
		"timestamp": strconv.FormatInt(timestamp, 10),
		"nonce":     "11886",
	}

	// 生成token
	token := GenerateToken(param, openKey)

	// 构建请求体
	body := RequestBody{
		OpenId:    openId,
		GrantCode: grantCode,
		Nonce:     nonce,
		Timestamp: timestamp,
		Token:     token,
		Params: BusinessRecordParams{
			EndDate:   endDate,
			StartDate: startDate,
			ShopCode:  shopCode,
		},
	}

	// 发送请求
	return sendRequest(url, body)
}

// sendRequest 发送HTTP请求
func sendRequest(url string, body interface{}) (map[string]interface{}, error) {
	// 序列化请求体
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return result, nil
}
