package data_svc

import (
	"net/url"
	"testing"

	"gin-biz-web-api/model"
)

func TestResolveRawSourcePrefersConfiguredQueryKey(t *testing.T) {
	query := url.Values{
		"source":    []string{"qimai_order"},
		"shop_code": []string{"YX001"},
	}
	definitions := []model.SourceDefinition{
		{
			Code:           "qimai_order",
			Enabled:        true,
			SourceQueryKey: "shop_code",
		},
	}

	got := ResolveRawSource(query, "", "", definitions)

	if got.SourceCode != "YX001" {
		t.Fatalf("SourceCode = %q, want %q", got.SourceCode, "YX001")
	}
	if got.SourceQueryKey != "shop_code" {
		t.Fatalf("SourceQueryKey = %q, want %q", got.SourceQueryKey, "shop_code")
	}
}

func TestResolveRawSourceUsesDefaultAliasesInOrder(t *testing.T) {
	query := url.Values{
		"data_source": []string{"youzan_order"},
		"remark":      []string{"legacy_order"},
	}

	got := ResolveRawSource(query, "", "", nil)

	if got.SourceCode != "youzan_order" {
		t.Fatalf("SourceCode = %q, want %q", got.SourceCode, "youzan_order")
	}
	if got.SourceQueryKey != "data_source" {
		t.Fatalf("SourceQueryKey = %q, want %q", got.SourceQueryKey, "data_source")
	}
}

func TestResolveRawSourceFallsBackToBodyThenUnknown(t *testing.T) {
	bodySource := ResolveRawSource(url.Values{}, "body_source", "", nil)
	if bodySource.SourceCode != "body_source" {
		t.Fatalf("SourceCode = %q, want %q", bodySource.SourceCode, "body_source")
	}

	unknown := ResolveRawSource(url.Values{}, "", "", nil)
	if unknown.SourceCode != "unknown" {
		t.Fatalf("SourceCode = %q, want %q", unknown.SourceCode, "unknown")
	}
}
