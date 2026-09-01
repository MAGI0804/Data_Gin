package reportoracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/godror/godror"
)

const BusinessOverviewPaymentTable = "YL_DBS.BJ_REPORT_RETAIL_DAY_SF"

const businessOverviewPaymentsSQL = `
SELECT a.BILLDATE,
       a.C_STORE_ID,
       a.STORE_NAME,
       a.STORE_CODE,
       a.C_PAYWAY_ID,
       a.PAYAMOUNT,
       a.PAYWAY_NAME AS "付款方式"
FROM ` + BusinessOverviewPaymentTable + ` a
WHERE a.BILLDATE = :1
  AND a.STORE_CODE = :2`

type BusinessOverviewPaymentRow struct {
	BillDate   int64
	StoreID    int64
	StoreName  string
	StoreCode  string
	PaywayID   int64
	PayAmount  float64
	PaywayName string
}

func (adapter *Adapter) QueryBusinessOverviewPayments(
	ctx context.Context,
	billDate int,
	storeCode string,
) ([]BusinessOverviewPaymentRow, error) {
	if adapter == nil || adapter.db == nil {
		return nil, fmt.Errorf("query business overview payments: adapter is closed")
	}
	storeCode = strings.TrimSpace(storeCode)
	if billDate <= 0 || storeCode == "" {
		return nil, fmt.Errorf("query business overview payments: invalid filter")
	}
	rows, err := adapter.db.QueryContext(
		ctx,
		businessOverviewPaymentsSQL,
		billDate,
		storeCode,
		godror.PrefetchCount(adapter.prefetchRows),
		godror.FetchArraySize(adapter.fetchArraySize),
	)
	if err != nil {
		return nil, fmt.Errorf("query business overview payments: %w", err)
	}
	defer rows.Close()

	result := make([]BusinessOverviewPaymentRow, 0, 8)
	for rows.Next() {
		var billDateValue, storeID, paywayID sql.NullInt64
		var storeName, rowStoreCode, paywayName sql.NullString
		var payAmount sql.NullFloat64
		if err := rows.Scan(&billDateValue, &storeID, &storeName, &rowStoreCode, &paywayID, &payAmount, &paywayName); err != nil {
			return nil, fmt.Errorf("scan business overview payment: %w", err)
		}
		result = append(result, BusinessOverviewPaymentRow{
			BillDate: billDateValue.Int64, StoreID: storeID.Int64,
			StoreName: strings.TrimSpace(storeName.String), StoreCode: strings.TrimSpace(rowStoreCode.String),
			PaywayID: paywayID.Int64, PayAmount: payAmount.Float64, PaywayName: strings.TrimSpace(paywayName.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate business overview payments: %w", err)
	}
	return result, nil
}
