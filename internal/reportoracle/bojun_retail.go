package reportoracle

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/godror/godror"
)

const (
	BojunRetailTable        = "YL_OBS.BJ_REPORT_RETAIL_SF"
	maxBojunRetailBatchSize = 1000
)

type BojunRetailRow struct {
	RetailID       uint64
	StoreCode      string
	DocNo          string
	RetailSaleType string
	StatusTime     time.Time
	OrderPhone     string
	PaidAmount     float64
	PushAmount     float64
	IsToShop       string
	PushStatus     int
	PushDate       string
	ItemsJSON      string
}

const bojunRetailColumns = `
M_RETAIL_ID, STORE_CODE, DOCNO, RETAILSALETYPE, STATUSTIME,
DM_VP_C_VIP_MOBILE, TOT_AMT_SF, TOT_AMT_TS, IS_TOSHOP,
NVL(STATUS, 0), PUSH_DATE, JSON_ITEM`

const bojunRetailAfterIDSQL = `
SELECT ` + bojunRetailColumns + `
FROM (
    SELECT ` + bojunRetailColumns + `
    FROM ` + BojunRetailTable + `
    WHERE M_RETAIL_ID > :1
    ORDER BY M_RETAIL_ID
)
WHERE ROWNUM <= :2
ORDER BY M_RETAIL_ID`

const bojunRetailTimeRangeSQL = `
SELECT ` + bojunRetailColumns + `
FROM (
    SELECT ` + bojunRetailColumns + `
    FROM ` + BojunRetailTable + `
    WHERE STATUSTIME >= :1
      AND STATUSTIME < :2
      AND M_RETAIL_ID > :3
    ORDER BY M_RETAIL_ID
)
WHERE ROWNUM <= :4
ORDER BY M_RETAIL_ID`

const bojunRetailMaxIDSQL = `SELECT NVL(MAX(M_RETAIL_ID), 0) FROM ` + BojunRetailTable

const bojunRetailPushStatusSQL = `
UPDATE ` + BojunRetailTable + `
SET STATUS = :1, PUSH_DATE = :2
WHERE M_RETAIL_ID = :3`

func (adapter *Adapter) QueryBojunRetailAfterID(ctx context.Context, afterID uint64, limit int) ([]BojunRetailRow, error) {
	if adapter == nil || adapter.db == nil {
		return nil, fmt.Errorf("query bojun Oracle retail orders: adapter is closed")
	}
	if err := validateBojunRetailBatchSize(limit); err != nil {
		return nil, err
	}
	return adapter.queryBojunRetailRows(ctx, bojunRetailAfterIDSQL, afterID, limit)
}

func (adapter *Adapter) QueryBojunRetailByStatusTime(
	ctx context.Context,
	start time.Time,
	end time.Time,
	afterID uint64,
	limit int,
) ([]BojunRetailRow, error) {
	if adapter == nil || adapter.db == nil {
		return nil, fmt.Errorf("query bojun Oracle retail orders by time: adapter is closed")
	}
	if start.IsZero() || !end.After(start) {
		return nil, fmt.Errorf("query bojun Oracle retail orders by time: invalid time range")
	}
	if err := validateBojunRetailBatchSize(limit); err != nil {
		return nil, err
	}
	return adapter.queryBojunRetailRows(ctx, bojunRetailTimeRangeSQL, start, end, afterID, limit)
}

func (adapter *Adapter) MaxBojunRetailID(ctx context.Context) (uint64, error) {
	if adapter == nil || adapter.db == nil {
		return 0, fmt.Errorf("query bojun Oracle maximum retail id: adapter is closed")
	}
	var value int64
	if err := adapter.db.QueryRowContext(ctx, bojunRetailMaxIDSQL).Scan(&value); err != nil {
		return 0, fmt.Errorf("query bojun Oracle maximum retail id: %w", err)
	}
	if value < 0 {
		return 0, fmt.Errorf("query bojun Oracle maximum retail id: negative value")
	}
	return uint64(value), nil
}

func (adapter *Adapter) UpdateBojunRetailPushStatus(ctx context.Context, retailID uint64, success bool, pushDate string) error {
	if adapter == nil || adapter.db == nil {
		return fmt.Errorf("update bojun Oracle push status: adapter is closed")
	}
	if retailID == 0 {
		return fmt.Errorf("update bojun Oracle push status: retail id is required")
	}
	pushDate = strings.TrimSpace(pushDate)
	if len(pushDate) != 8 {
		return fmt.Errorf("update bojun Oracle push status: push date must use yyyyMMdd")
	}
	status := 0
	if success {
		status = 1
	}
	result, err := adapter.db.ExecContext(ctx, bojunRetailPushStatusSQL, status, pushDate, retailID)
	if err != nil {
		return fmt.Errorf("update bojun Oracle push status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update bojun Oracle push status rows affected: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("update bojun Oracle push status: retail id %d not found", retailID)
	}
	return nil
}

func (adapter *Adapter) queryBojunRetailRows(ctx context.Context, statement string, arguments ...interface{}) ([]BojunRetailRow, error) {
	arguments = append(arguments,
		godror.PrefetchCount(adapter.prefetchRows),
		godror.FetchArraySize(adapter.fetchArraySize),
		godror.ClobAsString(),
	)
	rows, err := adapter.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query bojun Oracle retail orders: %w", err)
	}
	defer rows.Close()

	result := make([]BojunRetailRow, 0)
	for rows.Next() {
		row, err := scanBojunRetailRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bojun Oracle retail orders: %w", err)
	}
	return result, nil
}

type bojunRetailScanner interface {
	Scan(dest ...interface{}) error
}

func scanBojunRetailRow(scanner bojunRetailScanner) (BojunRetailRow, error) {
	var (
		retailID                       int64
		storeCode, docNo, retailType   sql.NullString
		statusTime                     sql.NullTime
		orderPhone, isToShop, pushDate sql.NullString
		paidAmount, pushAmount         sql.NullFloat64
		pushStatus                     sql.NullInt64
		itemsJSON                      interface{}
	)
	if err := scanner.Scan(
		&retailID, &storeCode, &docNo, &retailType, &statusTime,
		&orderPhone, &paidAmount, &pushAmount, &isToShop,
		&pushStatus, &pushDate, &itemsJSON,
	); err != nil {
		return BojunRetailRow{}, fmt.Errorf("scan bojun Oracle retail order: %w", err)
	}
	if retailID <= 0 || strings.TrimSpace(docNo.String) == "" || !statusTime.Valid {
		return BojunRetailRow{}, fmt.Errorf("scan bojun Oracle retail order: required field is empty")
	}
	jsonText, err := bojunRetailText(itemsJSON)
	if err != nil {
		return BojunRetailRow{}, fmt.Errorf("scan bojun Oracle retail order JSON_ITEM: %w", err)
	}
	return BojunRetailRow{
		RetailID: uint64(retailID), StoreCode: strings.TrimSpace(storeCode.String), DocNo: strings.TrimSpace(docNo.String),
		RetailSaleType: strings.TrimSpace(retailType.String), StatusTime: statusTime.Time,
		OrderPhone: strings.TrimSpace(orderPhone.String), PaidAmount: paidAmount.Float64, PushAmount: pushAmount.Float64,
		IsToShop: strings.ToUpper(strings.TrimSpace(isToShop.String)), PushStatus: int(pushStatus.Int64),
		PushDate: strings.TrimSpace(pushDate.String), ItemsJSON: strings.TrimSpace(jsonText),
	}, nil
}

func bojunRetailText(value interface{}) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	case godror.Lob:
		return readBojunRetailLOB(typed.Reader)
	case *godror.Lob:
		if typed == nil {
			return "", nil
		}
		return readBojunRetailLOB(typed.Reader)
	default:
		return "", fmt.Errorf("unsupported Oracle text type %T", value)
	}
}

func readBojunRetailLOB(reader io.Reader) (string, error) {
	if reader == nil {
		return "", nil
	}
	value, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func validateBojunRetailBatchSize(limit int) error {
	if limit < 1 || limit > maxBojunRetailBatchSize {
		return fmt.Errorf("query bojun Oracle retail orders: batch size must be between 1 and %d", maxBojunRetailBatchSize)
	}
	return nil
}
