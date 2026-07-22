package caiyun

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseLifeIndexV3MapsKnownAndPreservesUnknownTypes(t *testing.T) {
	raw, err := os.ReadFile("testdata/life_index_v3.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	bundle, err := ParseLifeIndexV3(raw)
	if err != nil {
		t.Fatalf("ParseLifeIndexV3() error=%v", err)
	}
	if len(bundle.Days) != 2 || bundle.Days[0].Date.Format("2006-01-02") != "2026-07-22" || bundle.Days[1].Date.Format("2006-01-02") != "2026-07-23" {
		t.Fatalf("days=%+v", bundle.Days)
	}
	firstDay := bundle.Days[0]
	if len(firstDay.Items) != 2 || firstDay.Items[0].Type != 1 || firstDay.Items[0].Code != "AIR_CONDITIONER" {
		t.Fatalf("first day items=%+v", firstDay.Items)
	}
	if firstDay.Items[1].Type != 26 || firstDay.Items[1].Level == nil || *firstDay.Items[1].Level != 4 || firstDay.Items[1].Description != "紫外线较强" {
		t.Fatalf("duplicate last-wins item=%+v", firstDay.Items[1])
	}
	unknown := bundle.Days[1].Items[1]
	if unknown.Type != 99 || unknown.Code != "UNKNOWN_99" || !unknown.UnknownType || unknown.Level != nil || len(unknown.ProviderJSON) == 0 {
		t.Fatalf("unknown item=%+v", unknown)
	}
	for _, code := range []string{"DUPLICATE_TYPE", "UNKNOWN_TYPE", "MISSING_LEVEL"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func TestParseLifeIndexV3MergesDuplicateDatesAndSortsTypes(t *testing.T) {
	raw := []byte(`{"data":[
		{"date":"2026-07-22","lifeindex":[{"type":8,"level":1,"desc":"a","detail":"a"}]},
		{"date":"2026-07-22","lifeindex":[{"type":1,"level":2,"desc":"b","detail":"b"}]}
	]}`)
	bundle, err := ParseLifeIndexV3(raw)
	if err != nil {
		t.Fatalf("ParseLifeIndexV3() error=%v", err)
	}
	if len(bundle.Days) != 1 || len(bundle.Days[0].Items) != 2 || bundle.Days[0].Items[0].Type != 1 || bundle.Days[0].Items[1].Type != 8 {
		t.Fatalf("bundle=%+v", bundle)
	}
	if !hasParseWarning(bundle.Warnings, "DUPLICATE_DATE") {
		t.Fatalf("warnings=%+v", bundle.Warnings)
	}
}

func TestParseLifeIndexV3ContainsBadItemsWithoutLosingGoodDays(t *testing.T) {
	raw := []byte(`{"data":[
		{"date":"not-a-date","lifeindex":[]},
		{"date":"2026-07-22","lifeindex":[
			{"type":"bad","level":1,"desc":"bad","detail":"bad"},
			{"type":8,"level":101,"desc":" good ","detail":" retained "}
		]}
	]}`)
	bundle, err := ParseLifeIndexV3(raw)
	if err != nil {
		t.Fatalf("ParseLifeIndexV3() error=%v", err)
	}
	if len(bundle.Days) != 1 || len(bundle.Days[0].Items) != 1 || bundle.Days[0].Items[0].Level != nil || bundle.Days[0].Items[0].Description != "good" {
		t.Fatalf("bundle=%+v", bundle)
	}
	for _, code := range []string{"INVALID_DATE", "INVALID_ITEM", "INVALID_LEVEL"} {
		if !hasParseWarning(bundle.Warnings, code) {
			t.Fatalf("warnings=%+v missing=%s", bundle.Warnings, code)
		}
	}
}

func TestParseLifeIndexV3TruncatesStorageTextWithWarnings(t *testing.T) {
	description := strings.Repeat("描", maximumLifeIndexDescRunes+1)
	detail := strings.Repeat("详", maximumLifeIndexDetailRunes+1)
	raw := []byte(`{"data":[{"date":"2026-07-22","lifeindex":[{"type":1,"level":1,"desc":"` + description + `","detail":"` + detail + `"}]}]}`)
	bundle, err := ParseLifeIndexV3(raw)
	if err != nil {
		t.Fatalf("ParseLifeIndexV3() error=%v", err)
	}
	item := bundle.Days[0].Items[0]
	if len([]rune(item.Description)) != maximumLifeIndexDescRunes || len([]rune(item.Detail)) != maximumLifeIndexDetailRunes {
		t.Fatalf("text lengths=%d,%d", len([]rune(item.Description)), len([]rune(item.Detail)))
	}
	if warningCount(bundle.Warnings, "TEXT_TRUNCATED") != 2 {
		t.Fatalf("warnings=%+v", bundle.Warnings)
	}
}

func TestParseLifeIndexV3RejectsUnsafeEnvelope(t *testing.T) {
	tooManyDays := `{"data":[`
	for index := 0; index < maximumLifeIndexDays+1; index++ {
		if index > 0 {
			tooManyDays += ","
		}
		tooManyDays += `{"date":"2026-07-22","lifeindex":[]}`
	}
	tooManyDays += `]}`
	tests := []string{
		`not-json`, `{"data":null}`, `{"data":[]}`,
		`{"data":[{"date":"bad","lifeindex":[]}]}`,
		`{"data":[{"date":"2026-07-22","lifeindex":{}}]}`,
		tooManyDays,
	}
	for _, raw := range tests {
		_, err := ParseLifeIndexV3([]byte(raw))
		var parseError *ParseError
		if !errors.As(err, &parseError) || parseError.EndpointKind != EndpointLifeIndexV3 {
			t.Fatalf("ParseLifeIndexV3(%q) error=%v", raw, err)
		}
		if strings.Contains(err.Error(), raw) {
			t.Fatalf("error leaked response: %v", err)
		}
	}
}

func TestLifeIndexIdentityCoversDocumentedTypes(t *testing.T) {
	for _, indexType := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 30, 31, 32, 33, 35, 36} {
		code, name, known := lifeIndexIdentity(indexType)
		if !known || code == "" || name == "" {
			t.Fatalf("lifeIndexIdentity(%d)=%q,%q,%t", indexType, code, name, known)
		}
	}
	if code, _, known := lifeIndexIdentity(0); known || code != "UNKNOWN_LIFEINDEX" {
		t.Fatalf("lifeIndexIdentity(0)=%q,known=%t", code, known)
	}
}

func hasParseWarning(warnings []ParseWarning, code string) bool {
	return warningCount(warnings, code) > 0
}

func warningCount(warnings []ParseWarning, code string) int {
	count := 0
	for _, warning := range warnings {
		if warning.Code == code {
			count++
		}
	}
	return count
}
