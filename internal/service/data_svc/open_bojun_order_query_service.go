package data_svc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gin-biz-web-api/internal/dao/data_dao"
	"gin-biz-web-api/internal/requestbody"
	"gin-biz-web-api/model"
	"gin-biz-web-api/pkg/database"
)

const (
	openBojunOrderDefaultPageSize = 50
	openBojunOrderMaxPageSize     = 100
	openBojunOrderMaxStoreCodes   = 20
	openBojunOrderMaxRangeDays    = 31
	openBojunOrderQueryTimeout    = 3 * time.Second
	openBojunOrderMaxLines        = 200
	openBojunOrderMaxItemsBytes   = 1 << 20
)

var (
	ErrOpenBojunOrderForbidden    = errors.New("open bojun order: forbidden")
	ErrOpenBojunOrderInvalidQuery = errors.New("open bojun order: invalid query")
	openBojunStoreCodePattern     = regexp.MustCompile(`^[A-Z0-9_-]{1,100}$`)
)

type OpenBojunOrderQueryService struct {
	orders      openBojunOrderReader
	permissions openBojunOrderPermissionReader
	now         func() time.Time
}

type openBojunOrderReader interface {
	ListOpenOrders(context.Context, data_dao.OpenBojunOrderQuery) ([]model.BojunRetailOrder, error)
}

type openBojunOrderPermissionReader interface {
	HasPermission(context.Context, uint, string, time.Time) (bool, error)
}

type OpenBojunOrderQueryResult struct {
	Items      []OpenBojunOrderDTO      `json:"items"`
	Pagination OpenBojunOrderPagination `json:"pagination"`
}

type OpenBojunOrderPagination struct {
	PageSize   int    `json:"pageSize"`
	NextCursor string `json:"nextCursor"`
	HasMore    bool   `json:"hasMore"`
}

type OpenBojunOrderDTO struct {
	OrderNo         string                  `json:"orderNo"`
	ExternalOrderNo string                  `json:"externalOrderNo"`
	OrderDate       string                  `json:"orderDate"`
	StoreCode       string                  `json:"storeCode"`
	StoreName       string                  `json:"storeName"`
	OrderTypeCode   string                  `json:"orderTypeCode"`
	OrderTypeName   string                  `json:"orderTypeName"`
	TotalLines      int                     `json:"totalLines"`
	TotalQuantity   int                     `json:"totalQuantity"`
	ListAmount      string                  `json:"listAmount"`
	ActualAmount    string                  `json:"actualAmount"`
	AverageDiscount string                  `json:"averageDiscount"`
	Currency        string                  `json:"currency"`
	RelatedOrderNo  string                  `json:"relatedOrderNo"`
	Items           []OpenBojunOrderLineDTO `json:"items"`
}

type OpenBojunOrderLineDTO struct {
	SKUNo        string `json:"skuNo"`
	ProductName  string `json:"productName"`
	Quantity     string `json:"quantity"`
	ActualAmount string `json:"actualAmount"`
}

type openBojunOrderCursor struct {
	BillDate int  `json:"billDate"`
	ID       uint `json:"id"`
}

func NewOpenBojunOrderQueryService() *OpenBojunOrderQueryService {
	return newOpenBojunOrderQueryService(
		data_dao.NewBojunRetailOrderDAO(database.DB),
		data_dao.NewMallWeatherPermissionDAO(database.DB),
		time.Now,
	)
}

func newOpenBojunOrderQueryService(
	orders openBojunOrderReader,
	permissions openBojunOrderPermissionReader,
	now func() time.Time,
) *OpenBojunOrderQueryService {
	if orders == nil || permissions == nil || now == nil {
		panic("open bojun order query service: nil dependency")
	}
	return &OpenBojunOrderQueryService{orders: orders, permissions: permissions, now: now}
}

func (service *OpenBojunOrderQueryService) Query(
	ctx context.Context,
	actorUserID uint,
	request requestbody.OpenBojunOrderQueryRequest,
) (*OpenBojunOrderQueryResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("open bojun order query: nil context")
	}
	if err := service.authorize(ctx, actorUserID); err != nil {
		return nil, err
	}
	query, pageSize, err := normalizeOpenBojunOrderQuery(request)
	if err != nil {
		return nil, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, openBojunOrderQueryTimeout)
	defer cancel()
	orders, err := service.orders.ListOpenOrders(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("open bojun order query: list orders: %w", err)
	}

	hasMore := len(orders) > pageSize
	if hasMore {
		orders = orders[:pageSize]
	}
	items := make([]OpenBojunOrderDTO, 0, len(orders))
	for index := range orders {
		items = append(items, openBojunOrderDTO(&orders[index]))
	}
	nextCursor := ""
	if hasMore && len(orders) > 0 {
		nextCursor, err = encodeOpenBojunOrderCursor(openBojunOrderCursor{
			BillDate: orders[len(orders)-1].BillDate,
			ID:       orders[len(orders)-1].ID,
		})
		if err != nil {
			return nil, fmt.Errorf("open bojun order query: encode cursor: %w", err)
		}
	}
	return &OpenBojunOrderQueryResult{
		Items: items,
		Pagination: OpenBojunOrderPagination{
			PageSize: pageSize, NextCursor: nextCursor, HasMore: hasMore,
		},
	}, nil
}

func (service *OpenBojunOrderQueryService) authorize(ctx context.Context, actorUserID uint) error {
	if actorUserID == 0 {
		return ErrOpenBojunOrderForbidden
	}
	allowed, err := service.permissions.HasPermission(
		ctx,
		actorUserID,
		model.PermissionBojunOrderRead,
		service.now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("open bojun order query: authorize: %w", err)
	}
	if !allowed {
		return ErrOpenBojunOrderForbidden
	}
	return nil
}

func normalizeOpenBojunOrderQuery(
	request requestbody.OpenBojunOrderQueryRequest,
) (data_dao.OpenBojunOrderQuery, int, error) {
	start, err := time.Parse("2006-01-02", strings.TrimSpace(request.StartDate))
	if err != nil {
		return data_dao.OpenBojunOrderQuery{}, 0, fmt.Errorf("%w: invalid startDate", ErrOpenBojunOrderInvalidQuery)
	}
	end, err := time.Parse("2006-01-02", strings.TrimSpace(request.EndDate))
	if err != nil || end.Before(start) || end.Sub(start) > (openBojunOrderMaxRangeDays-1)*24*time.Hour {
		return data_dao.OpenBojunOrderQuery{}, 0, fmt.Errorf("%w: invalid endDate", ErrOpenBojunOrderInvalidQuery)
	}

	storeCodes, err := normalizeOpenBojunStoreCodes(request.StoreCodes)
	if err != nil {
		return data_dao.OpenBojunOrderQuery{}, 0, err
	}
	orderTypes, err := normalizeOpenBojunOrderTypes(request.OrderTypes)
	if err != nil {
		return data_dao.OpenBojunOrderQuery{}, 0, err
	}
	pageSize := request.PageSize
	if pageSize == 0 {
		pageSize = openBojunOrderDefaultPageSize
	}
	if pageSize < 1 || pageSize > openBojunOrderMaxPageSize {
		return data_dao.OpenBojunOrderQuery{}, 0, fmt.Errorf("%w: invalid pageSize", ErrOpenBojunOrderInvalidQuery)
	}

	startBillDate, _ := strconv.Atoi(start.Format("20060102"))
	endBillDate, _ := strconv.Atoi(end.Format("20060102"))
	query := data_dao.OpenBojunOrderQuery{
		StartBillDate: startBillDate,
		EndBillDate:   endBillDate,
		StoreCodes:    storeCodes,
		OrderTypes:    orderTypes,
		Limit:         pageSize + 1,
	}
	if cursorValue := strings.TrimSpace(request.Cursor); cursorValue != "" {
		cursor, err := decodeOpenBojunOrderCursor(cursorValue)
		if err != nil || cursor.BillDate < startBillDate || cursor.BillDate > endBillDate {
			return data_dao.OpenBojunOrderQuery{}, 0, fmt.Errorf("%w: invalid cursor", ErrOpenBojunOrderInvalidQuery)
		}
		query.BeforeBillDate = cursor.BillDate
		query.BeforeID = cursor.ID
	}
	return query, pageSize, nil
}

func normalizeOpenBojunStoreCodes(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > openBojunOrderMaxStoreCodes {
		return nil, fmt.Errorf("%w: invalid storeCodes", ErrOpenBojunOrderInvalidQuery)
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if !openBojunStoreCodePattern.MatchString(value) {
			return nil, fmt.Errorf("%w: invalid storeCodes", ErrOpenBojunOrderInvalidQuery)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func normalizeOpenBojunOrderTypes(values []string) ([]string, error) {
	allowed := map[string]struct{}{"CMR": {}, "RET": {}, "EXP": {}}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if _, ok := allowed[value]; !ok {
			return nil, fmt.Errorf("%w: invalid orderTypes", ErrOpenBojunOrderInvalidQuery)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func openBojunOrderDTO(order *model.BojunRetailOrder) OpenBojunOrderDTO {
	return OpenBojunOrderDTO{
		OrderNo:         order.DocNo,
		ExternalOrderNo: order.OtherDocNo,
		OrderDate:       formatOpenBojunBillDate(order.BillDate),
		StoreCode:       order.StoreCode,
		StoreName:       order.StoreName,
		OrderTypeCode:   order.OrderTypeCode,
		OrderTypeName:   order.OrderTypeName,
		TotalLines:      order.TotalLines,
		TotalQuantity:   order.TotalQty,
		ListAmount:      strconv.FormatFloat(order.TotalAmtList, 'f', 2, 64),
		ActualAmount:    strconv.FormatFloat(order.TotalAmtActual, 'f', 2, 64),
		AverageDiscount: strconv.FormatFloat(order.AvgDiscount, 'f', 4, 64),
		Currency:        "CNY",
		RelatedOrderNo:  order.RelatedNormalNo,
		Items:           openBojunOrderLines(order.ItemsJSON),
	}
}

func openBojunOrderLines(raw string) []OpenBojunOrderLineDTO {
	if len(raw) == 0 || len(raw) > openBojunOrderMaxItemsBytes {
		return []OpenBojunOrderLineDTO{}
	}
	var values []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []OpenBojunOrderLineDTO{}
	}
	if len(values) > openBojunOrderMaxLines {
		values = values[:openBojunOrderMaxLines]
	}
	items := make([]OpenBojunOrderLineDTO, 0, len(values))
	for _, value := range values {
		items = append(items, OpenBojunOrderLineDTO{
			SKUNo:        truncateOpenBojunOrderString(stringFromAny(value["no"]), 128),
			ProductName:  truncateOpenBojunOrderString(stringFromAny(value["mProductName"]), 500),
			Quantity:     formatOpenBojunOrderNumber(floatFromAny(value["qty"]), -1),
			ActualAmount: formatOpenBojunOrderNumber(floatFromAny(value["totAmtActual"]), 2),
		})
	}
	return items
}

func formatOpenBojunOrderNumber(value float64, precision int) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "0"
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func truncateOpenBojunOrderString(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func formatOpenBojunBillDate(value int) string {
	parsed, err := time.Parse("20060102", strconv.Itoa(value))
	if err != nil {
		return ""
	}
	return parsed.Format("2006-01-02 15:04:05")
}

func encodeOpenBojunOrderCursor(cursor openBojunOrderCursor) (string, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeOpenBojunOrderCursor(value string) (openBojunOrderCursor, error) {
	if len(value) > 512 {
		return openBojunOrderCursor{}, ErrOpenBojunOrderInvalidQuery
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return openBojunOrderCursor{}, err
	}
	var cursor openBojunOrderCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.BillDate <= 0 || cursor.ID == 0 {
		return openBojunOrderCursor{}, ErrOpenBojunOrderInvalidQuery
	}
	return cursor, nil
}
