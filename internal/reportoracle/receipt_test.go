package reportoracle

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildRunReceiptPlanUsesValidatedOwnerAndRunBind(t *testing.T) {
	plan, err := BuildRunReceiptPlan("report_owner")
	if err != nil {
		t.Fatalf("BuildRunReceiptPlan() error = %v", err)
	}
	if !strings.Contains(plan.readStatement, "FROM REPORT_OWNER.REPORT_RUN_RECEIPT WHERE RUN_ID = :1") ||
		!strings.Contains(plan.writeStatement, "INSERT INTO REPORT_OWNER.REPORT_RUN_RECEIPT") {
		t.Fatalf("plan = %#v", plan)
	}
	if _, err := BuildRunReceiptPlan("REPORT; DROP TABLE X"); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("unsafe owner error = %v", err)
	}
}
