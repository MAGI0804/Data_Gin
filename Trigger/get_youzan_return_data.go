package Trigger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"gin-biz-web-api/pkg/config"
)

type YouzanRefundResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total   int `json:"total"`
		Refunds []struct {
			NodeKdtID      int64  `json:"node_kdt_id"`
			Reason         int    `json:"reason"`
			KdtID          int64  `json:"kdt_id"`
			ReturnGoods    bool   `json:"return_goods"`
			Created        string `json:"created"`
			RefundFee      string `json:"refund_fee"`
			Modified       string `json:"modified"`
			CSStatus       int    `json:"cs_status"`
			RefundID       string `json:"refund_id"`
			Tid            string `json:"tid"`
			DeliveryStatus int    `json:"delivery_status"`
			Status         string `json:"status"`
		} `json:"refunds"`
	} `json:"data"`
}

type RefundOrder struct {
	RefundID       string  `json:"refund_id"`
	TID            string  `json:"tid"`
	Status         string  `json:"status"`
	NodeKdtID      int64   `json:"node_kdt_id"`
	KdtID          int64   `json:"kdt_id"`
	Reason         int     `json:"reason"`
	ReturnGoods    bool    `json:"return_goods"`
	CSStatus       int     `json:"cs_status"`
	DeliveryStatus int     `json:"delivery_status"`
	RefundFee      float64 `json:"refund_fee"`
	Created        string  `json:"created"`
	Modified       string  `json:"modified"`
	Payment        float64 `json:"payment"`
	TotalFee       float64 `json:"total_fee"`
	TotalAmt       float64 `json:"totalAmt"`
}

func GetYouzanRefundOrders(accessToken string, nodeKdtID int64) ([]RefundOrder, error) {
	now := time.Now().Unix()
	fiveMinutesAgo := now - 5*60
	return GetYouzanRefundOrdersByRange(accessToken, fiveMinutesAgo, now)
}

func GetYouzanRefundOrdersByRange(accessToken string, createTimeStart, createTimeEnd int64) ([]RefundOrder, error) {
	url := fmt.Sprintf("%s?access_token=%s", config.GetString("cfg.youzan.refund_url"), accessToken)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	var allRefundOrders []RefundOrder
	pageNo := 1
	pageSize := 100
	maxPages := 100
	hasMore := true

	for hasMore && pageNo <= maxPages {
		params := map[string]interface{}{
			"create_time_start": createTimeStart,
			"create_time_end":   createTimeEnd,
			"page_no":           pageNo,
			"page_size":         pageSize,
		}

		jsonData, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("序列化请求数据失败: %w", err)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %w", err)
		}

		for key, value := range headers {
			req.Header.Set(key, value)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("发送请求失败: %w", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("读取响应失败: %w", err)
		}

		var result YouzanRefundResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}

		if !result.Success {
			return nil, fmt.Errorf("API请求失败: %s", result.Message)
		}

		for _, refund := range result.Data.Refunds {
			refundFee, _ := strconv.ParseFloat(refund.RefundFee, 64)
			negativeFee := -refundFee

			allRefundOrders = append(allRefundOrders, RefundOrder{
				RefundID:       refund.RefundID,
				TID:            refund.Tid,
				Status:         refund.Status,
				NodeKdtID:      refund.NodeKdtID,
				KdtID:          refund.KdtID,
				Reason:         refund.Reason,
				ReturnGoods:    refund.ReturnGoods,
				CSStatus:       refund.CSStatus,
				DeliveryStatus: refund.DeliveryStatus,
				RefundFee:      refundFee,
				Created:        refund.Created,
				Modified:       refund.Modified,
				Payment:        negativeFee,
				TotalFee:       negativeFee,
				TotalAmt:       negativeFee,
			})
		}

		if len(result.Data.Refunds) < pageSize {
			hasMore = false
		} else {
			pageNo++
		}
	}

	return allRefundOrders, nil
}
