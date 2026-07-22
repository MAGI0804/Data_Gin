package data_dao

import (
	"context"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestMallWeatherExportProfileDAORejectsInvalidStateBeforeDatabase(t *testing.T) {
	if _, err := (&MallWeatherExportProfileDAO{}).Save(context.Background(), nil, nil); err == nil {
		t.Fatal("Save() accepted an unconfigured DAO")
	}
	if _, err := (&MallWeatherExportProfileDAO{}).List(context.Background(), nil); err == nil {
		t.Fatal("List() accepted an unconfigured DAO")
	}
}

func TestMallWeatherExportProfileDuplicateErrorsAreConflicts(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "mysql duplicate", err: &mysqlDriver.MySQLError{Number: 1062, Message: "duplicate"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !isMallWeatherExportProfileDuplicate(test.err) {
				t.Fatalf("isMallWeatherExportProfileDuplicate(%v) = false", test.err)
			}
		})
	}
	if isMallWeatherExportProfileDuplicate(context.Canceled) {
		t.Fatal("context cancellation was classified as a duplicate")
	}
}
