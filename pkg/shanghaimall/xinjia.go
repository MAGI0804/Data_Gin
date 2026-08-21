package shanghaimall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	xinjiaCenterDateTimeLayout = "2006-01-02T15:04:05.000-0700"
	xinjiaCenterSaleTimeLayout = "2006-01-02 15:04:05"
	xinjiaCenterBizState       = "ineffect"
	xinjiaCenterReceiver       = "contract"
	xinjiaCenterPaymentCode    = "05"
)

var xinjiaCenterLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type xinjiaCenterPayment struct {
	PaymentCode string  `json:"paymentCode"`
	Total       float64 `json:"total"`
}

type xinjiaCenterSalesRequest struct {
	ProductCode string                `json:"productCode"`
	StoreCode   string                `json:"storeCode"`
	TenantCode  string                `json:"tenantCode"`
	BizState    string                `json:"bizState"`
	Effect      bool                  `json:"effect"`
	Receiver    string                `json:"receiver"`
	SaleDate    string                `json:"saleDate"`
	SaleCount   int                   `json:"saleCount"`
	SaleAmount  float64               `json:"saleAmount"`
	Remark      string                `json:"remark"`
	Payments    []xinjiaCenterPayment `json:"payments"`
}

func PushXinjiaCenter(ctx context.Context, order RetailOrder) (*PushResult, error) {
	return pushXinjiaCenter(ctx, xinjiaCenterConfigFromEnv(), order)
}

func pushXinjiaCenter(ctx context.Context, cfg xinjiaCenterConfig, order RetailOrder) (*PushResult, error) {
	if err := order.validate(); err != nil {
		return nil, err
	}
	if cfg.URL == "" || cfg.ProductCode == "" || cfg.StoreCode == "" || cfg.TenantCode == "" || cfg.Authorization == "" || cfg.Client == nil {
		return nil, fmt.Errorf("xinjia center config is incomplete")
	}

	saleDate, err := xinjiaCenterSaleDate(order.SaleTime)
	if err != nil {
		return nil, err
	}
	now := time.Now
	if cfg.Now != nil {
		now = cfg.Now
	}
	requestURL, err := xinjiaCenterURL(cfg.URL, now())
	if err != nil {
		return nil, err
	}

	total := order.Amount
	if order.IsRefund() && total > 0 {
		total = -total
	}
	requestBody := xinjiaCenterSalesRequest{
		ProductCode: cfg.ProductCode,
		StoreCode:   cfg.StoreCode,
		TenantCode:  cfg.TenantCode,
		BizState:    xinjiaCenterBizState,
		Effect:      false,
		Receiver:    xinjiaCenterReceiver,
		SaleDate:    saleDate,
		SaleCount:   order.normalizedQuantity(),
		SaleAmount:  total,
		Remark:      order.DocNo,
		Payments: []xinjiaCenterPayment{
			{PaymentCode: xinjiaCenterPaymentCode, Total: total},
		},
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal xinjia center sales request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create xinjia center sales request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", cfg.Authorization)
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

func xinjiaCenterSaleDate(value string) (string, error) {
	parsed, err := time.ParseInLocation(xinjiaCenterSaleTimeLayout, value, xinjiaCenterLocation)
	if err != nil {
		return "", fmt.Errorf("parse xinjia center sale time: %w", err)
	}
	return parsed.Format(xinjiaCenterDateTimeLayout), nil
}

func xinjiaCenterURL(rawURL string, now time.Time) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse xinjia center url: %w", err)
	}
	query := parsed.Query()
	query.Set("time", now.In(xinjiaCenterLocation).Format(xinjiaCenterDateTimeLayout))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
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
		"bizState":    body.BizState,
		"effect":      body.Effect,
		"receiver":    body.Receiver,
		"saleDate":    body.SaleDate,
		"saleCount":   body.SaleCount,
		"saleAmount":  body.SaleAmount,
		"remark":      body.Remark,
		"payments": []map[string]interface{}{
			{
				"paymentCode": body.Payments[0].PaymentCode,
				"total":       body.Payments[0].Total,
			},
		},
	}
}
