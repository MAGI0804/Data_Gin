package reportoracle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const RunReceiptTableName = "REPORT_RUN_RECEIPT"

var ErrRunReceiptNotFound = errors.New("oracle report run receipt not found")

type RunReceiptPlan struct {
	readStatement  string
	writeStatement string
}

type RunReceipt struct {
	RunID        string
	ReportCode   string
	ContractHash string
	RowCount     int64
	CompletedAt  time.Time
}

func BuildRunReceiptPlan(owner string) (RunReceiptPlan, error) {
	table, err := NormalizeResultTableRef(ResultTableRef{Owner: owner, Name: RunReceiptTableName})
	if err != nil {
		return RunReceiptPlan{}, err
	}
	return RunReceiptPlan{readStatement: fmt.Sprintf(
		"SELECT RUN_ID, REPORT_CODE, CONTRACT_HASH, ROW_COUNT, COMPLETED_AT FROM %s.%s WHERE RUN_ID = :1", table.Owner, table.Name,
	), writeStatement: fmt.Sprintf(
		"INSERT INTO %s.%s (RUN_ID, REPORT_CODE, CONTRACT_HASH, ROW_COUNT, COMPLETED_AT) VALUES (:1, :2, :3, :4, :5)", table.Owner, table.Name,
	)}, nil
}

func (adapter *Adapter) ReadRunReceipt(ctx context.Context, plan RunReceiptPlan, runID string) (RunReceipt, error) {
	if adapter == nil || adapter.db == nil || strings.TrimSpace(plan.readStatement) == "" || strings.TrimSpace(runID) == "" {
		return RunReceipt{}, fmt.Errorf("read oracle report run receipt: invalid request")
	}
	var receipt RunReceipt
	err := adapter.db.QueryRowContext(ctx, plan.readStatement, runID).Scan(
		&receipt.RunID, &receipt.ReportCode, &receipt.ContractHash, &receipt.RowCount, &receipt.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RunReceipt{}, ErrRunReceiptNotFound
	}
	if err != nil {
		return RunReceipt{}, fmt.Errorf("read oracle report run receipt: %w", err)
	}
	if receipt.RunID != runID || strings.TrimSpace(receipt.ReportCode) == "" || len(strings.TrimSpace(receipt.ContractHash)) != 64 || receipt.RowCount < 0 || receipt.CompletedAt.IsZero() {
		return RunReceipt{}, fmt.Errorf("read oracle report run receipt: invalid stored receipt")
	}
	return receipt, nil
}

func (adapter *Adapter) WriteRunReceipt(ctx context.Context, tx *sql.Tx, plan RunReceiptPlan, receipt RunReceipt) error {
	if adapter == nil || tx == nil || strings.TrimSpace(plan.writeStatement) == "" || strings.TrimSpace(receipt.RunID) == "" ||
		strings.TrimSpace(receipt.ReportCode) == "" || len(strings.TrimSpace(receipt.ContractHash)) != 64 || receipt.RowCount < 0 || receipt.CompletedAt.IsZero() {
		return fmt.Errorf("write oracle report run receipt: invalid request")
	}
	if _, err := tx.ExecContext(ctx, plan.writeStatement, receipt.RunID, receipt.ReportCode, receipt.ContractHash, receipt.RowCount, receipt.CompletedAt.UTC()); err != nil {
		return fmt.Errorf("write oracle report run receipt: %w", err)
	}
	return nil
}
