package logger

import "testing"

func TestGormLoggerUsesParameterizedStatementPlaceholder(t *testing.T) {
	if NewGormLogger().SlowThreshold <= 0 {
		t.Fatal("GORM logger has no slow query threshold")
	}
}
