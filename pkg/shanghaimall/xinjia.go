package shanghaimall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type xinjiaCenterSalesRequest struct {
	ProductCode string  `json:"productCode"`
	StoreCode   string  `json:"storeCode"`
	TenantCode  string  `json:"tenantCode"`
	SaleCount   int     `json:"saleCount"`
	Total       float64 `json:"total"`
	Remark      string  `json:"remark"`
}

func PushXinjiaCenter(ctx context.Context, order RetailOrder) (*PushResult, error) {
	return pushXinjiaCenter(ctx, xinjiaCenterConfigFromEnv(), order)
}

func pushXinjiaCenter(ctx context.Context, cfg xinjiaCenterConfig, order RetailOrder) (*PushResult, error) {
	if err := order.validate(); err != nil {
		return nil, err
	}
	if cfg.URL == "" || cfg.ProductCode == "" || cfg.StoreCode == "" || cfg.Client == nil {
		return nil, fmt.Errorf("xinjia center config is incomplete")
	}

	total := order.Amount
	if order.IsRefund() && total > 0 {
		total = -total
	}
	requestBody := xinjiaCenterSalesRequest{
		ProductCode: cfg.ProductCode,
		StoreCode:   cfg.StoreCode,
		TenantCode:  order.SaleTime,
		SaleCount:   order.normalizedQuantity(),
		Total:       total,
		Remark:      order.DocNo,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal xinjia center sales request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create xinjia center sales request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send xinjia center sales request: %w", err)
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read xinjia center sales response: %w", err)
	}
	result := &PushResult{
		Target:       TargetXinjiaCenter,
		HTTPStatus:   resp.StatusCode,
		RequestBody:  xinjiaCenterRequestLogBody(requestBody),
		ResponseBody: string(responseBytes),
	}
	_ = json.Unmarshal(responseBytes, &result.ResponseJSON)
	result.Success = xinjiaCenterResponseSuccess(resp.StatusCode, result.ResponseJSON)
	if !result.Success {
		return result, fmt.Errorf("xinjia center sales push failed with status %d: %s", resp.StatusCode, result.ResponseBody)
	}
	return result, nil
}

func xinjiaCenterResponseSuccess(status int, response map[string]interface{}) bool {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return false
	}
	value, exists := response["success"]
	if !exists {
		return true
	}
	success, ok := value.(bool)
	return ok && success
}

func xinjiaCenterRequestLogBody(body xinjiaCenterSalesRequest) map[string]interface{} {
	return map[string]interface{}{
		"productCode": body.ProductCode,
		"storeCode":   body.StoreCode,
		"tenantCode":  body.TenantCode,
		"saleCount":   body.SaleCount,
		"total":       body.Total,
		"remark":      body.Remark,
	}
}
