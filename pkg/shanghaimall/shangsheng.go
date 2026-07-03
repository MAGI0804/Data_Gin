package shanghaimall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func PushShangsheng(ctx context.Context, order RetailOrder) (*PushResult, error) {
	return pushShangsheng(ctx, shangshengConfigFromEnv(), order)
}

func pushShangsheng(ctx context.Context, cfg shangshengConfig, order RetailOrder) (*PushResult, error) {
	if err := order.validate(); err != nil {
		return nil, err
	}
	if cfg.URL == "" || cfg.GID == "" || cfg.MID == "" || cfg.VSN == "" {
		return nil, fmt.Errorf("shangsheng config is incomplete")
	}

	orderType := "C"
	if order.IsRefund() {
		orderType = "V"
	}
	body := map[string]interface{}{
		"gid":       cfg.GID,
		"mid":       cfg.MID,
		"vsn":       cfg.VSN,
		"id":        order.DocNo + time.Now().Format("20060102150405"),
		"amount":    order.Amount,
		"orderNo":   order.DocNo,
		"orderType": orderType,
		"tranDate":  saleDate(order.SaleTime),
		"tranTime":  order.SaleTime,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("method", "commonOrderForThird")
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := &PushResult{
		Target:       TargetShangsheng,
		HTTPStatus:   resp.StatusCode,
		RequestBody:  body,
		ResponseBody: string(respBytes),
	}
	_ = json.Unmarshal(respBytes, &result.ResponseJSON)
	result.Success = resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	if !result.Success {
		return result, fmt.Errorf("shangsheng push failed: %s", result.ResponseBody)
	}
	return result, nil
}

func saleDate(saleTime string) string {
	parsed, err := time.Parse("2006-01-02 15:04:05", saleTime)
	if err != nil {
		return time.Now().Format("2006-01-02")
	}
	return parsed.Format("2006-01-02")
}
