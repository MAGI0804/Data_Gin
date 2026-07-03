package shanghaimall

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

func PushPanlong(ctx context.Context, order RetailOrder) (*PushResult, error) {
	return pushSyandata(ctx, TargetPanlong, syandataTargetConfig(TargetPanlong), order)
}

func PushXintiandi(ctx context.Context, order RetailOrder) (*PushResult, error) {
	return pushSyandata(ctx, TargetXintiandi, syandataTargetConfig(TargetXintiandi), order)
}

func pushSyandata(ctx context.Context, target Target, cfg syandataConfig, order RetailOrder) (*PushResult, error) {
	if err := order.validate(); err != nil {
		return nil, err
	}
	if cfg.URL == "" || cfg.AppKey == "" || cfg.AppID == "" || cfg.AppSecret == "" || cfg.TerminalNumber == "" {
		return nil, fmt.Errorf("%s syandata config is incomplete", target)
	}

	billType := "1"
	exactBillType := "10101"
	if order.IsRefund() {
		billType = "6"
		exactBillType = "10601"
	}

	data := map[string]interface{}{
		"terminalNumber":    cfg.TerminalNumber,
		"saleTime":          order.SaleTime,
		"billType":          billType,
		"exactBillType":     exactBillType,
		"billSerialNumber":  order.DocNo,
		"thirdPartyOrderNo": order.DocNo,
		"totalNum":          1,
		"totalFee":          order.Amount,
		"paidAmount":        order.Amount,
		"receivableAmount":  order.Amount,
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("method", "gogo.open.auto.routing")
	form.Set("timestamp", time.Now().Format("20060102150405"))
	form.Set("messageFormat", "json")
	form.Set("appKey", cfg.AppKey)
	form.Set("v", "1.0")
	form.Set("signMethod", "MD5")
	form.Set("lowerMethod", "com.gooagoo.exportbill")
	form.Set("appId", cfg.AppID)
	form.Set("data", string(dataJSON))
	form.Set("sign", syandataSign(cfg.AppSecret, form))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
		Target:       target,
		HTTPStatus:   resp.StatusCode,
		RequestBody:  data,
		ResponseBody: string(respBytes),
	}
	_ = json.Unmarshal(respBytes, &result.ResponseJSON)
	result.Success = result.ResponseJSON["rescode"] == "OPEN_SUCCESS"
	if !result.Success {
		return result, fmt.Errorf("%s syandata push failed: %s", target, result.ResponseBody)
	}
	return result, nil
}

func syandataSign(appSecret string, form url.Values) string {
	keys := make([]string, 0, len(form))
	for key := range form {
		if key == "sign" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := form.Get(key)
		if value == "" {
			continue
		}
		parts = append(parts, key+"="+value)
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + "&key=" + appSecret))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
