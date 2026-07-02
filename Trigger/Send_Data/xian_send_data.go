package send

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SaleData struct {
	ShopID          string   `json:"shopId"`
	ShopName        string   `json:"shopName"`
	Time            string   `json:"time"`
	DealNo          string   `json:"dealNo"`
	Money           string   `json:"money"`
	DiscountMoney   string   `json:"discountMoney"`
	ReceivableMoney string   `json:"receivableMoney"`
	Type            string   `json:"type"`
	GoodsList       []string `json:"goodsList"`
}

type Response struct {
	Msg       string `json:"msg"`
	ErrorCode string `json:"errorCode"`
}

func createSign(shopId, appSecret, timestamp, dealNo, timeStr string) string {
	concatStr := strings.ToUpper(shopId + timestamp + dealNo + timeStr + appSecret)
	h := sha1.New()
	h.Write([]byte(concatStr))
	return hex.EncodeToString(h.Sum(nil))
}

func PostSale(url, shopId, appSecret, shopName, timeStr, dealNo, money, discountMoney, receivableMoney string, goodsList []string) (bool, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := createSign(shopId, appSecret, timestamp, dealNo, timeStr)

	data := SaleData{
		ShopID:          shopId,
		ShopName:        shopName,
		Time:            timeStr,
		DealNo:          dealNo,
		Money:           money,
		DiscountMoney:   discountMoney,
		ReceivableMoney: receivableMoney,
		Type:            "1",
		GoodsList:       goodsList,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("shopId", shopId)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("signature", signature)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var result Response
	err = json.Unmarshal(body, &result)
	if err != nil {
		return false, err
	}

	return result.ErrorCode == "0", nil
}

func main() {
	url := "http://39.108.179.120:8585/posbox/sendKukumaoDetail"
	shopId := "69d8ba436febc0a6fbf0f885"
	appSecret := "ad8ba436febc0a6f"
	shopName := "西岸野选"
	timeStr := "2026-04-10 00:00:00"
	dealNo := "TEST0410"
	money := "0.01"
	discountMoney := "0"
	receivableMoney := "0.01"
	goodsList := []string{}

	success, err := PostSale(url, shopId, appSecret, shopName, timeStr, dealNo, money, discountMoney, receivableMoney, goodsList)
	if err != nil {
		fmt.Println("请求失败:", err)
		return
	}
	fmt.Println("发送成功:", success)
}
