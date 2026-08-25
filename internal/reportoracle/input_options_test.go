package reportoracle

import (
	"database/sql"
	"strings"
	"testing"
)

func TestBuildInputOptionQueryUsesBoundExactName(t *testing.T) {
	statement, arguments, err := buildInputOptionQuery("SELECT store_id AS id, store_name AS name FROM stores ORDER BY store_name", "上海店", 100)
	if err != nil {
		t.Fatalf("buildInputOptionQuery() error = %v", err)
	}
	if !strings.Contains(statement, ") REPORT_INPUT_SOURCE WHERE name = :name FETCH FIRST 100 ROWS ONLY") {
		t.Fatalf("statement = %s", statement)
	}
	if strings.Contains(statement, "上海店") || len(arguments) != 1 {
		t.Fatalf("statement=%s arguments=%#v", statement, arguments)
	}
	named, ok := arguments[0].(sql.NamedArg)
	if !ok || named.Name != "name" || named.Value != "上海店" {
		t.Fatalf("argument = %#v", arguments[0])
	}
}

func TestBuildInputOptionQueryRejectsUnsafeConfiguredSQL(t *testing.T) {
	for _, statement := range []string{"DELETE FROM stores", "SELECT id, name FROM stores;", "SELECT id, name FROM stores -- x"} {
		if _, _, err := buildInputOptionQuery(statement, "", 100); err == nil {
			t.Fatalf("buildInputOptionQuery(%q) error = nil", statement)
		}
	}
}

func TestNormalizeInputOptionValues(t *testing.T) {
	for _, value := range []interface{}{"S001", []byte("S002"), int64(3), float64(4.5)} {
		if _, err := normalizeInputOptionID(value); err != nil {
			t.Fatalf("normalizeInputOptionID(%#v) error = %v", value, err)
		}
	}
	if _, err := normalizeInputOptionID(nil); err == nil {
		t.Fatal("normalizeInputOptionID(nil) error = nil")
	}
	if name, err := normalizeInputOptionName([]byte("上海店")); err != nil || name != "上海店" {
		t.Fatalf("normalizeInputOptionName() = %q, %v", name, err)
	}
}
