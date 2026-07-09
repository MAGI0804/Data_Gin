package storage

import "testing"

func TestNormalizeOSSRegion(t *testing.T) {
	tests := map[string]string{
		"cn-shanghai":                            "cn-shanghai",
		"oss-cn-shanghai":                        "cn-shanghai",
		" https://oss-cn-shanghai.aliyuncs.com ": "cn-shanghai",
		"http://oss-cn-hangzhou.aliyuncs.com":    "cn-hangzhou",
		"":                                       "",
	}

	for input, want := range tests {
		if got := normalizeOSSRegion(input); got != want {
			t.Fatalf("normalizeOSSRegion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOSSClientPublicURLUsesNormalizedRegionFallback(t *testing.T) {
	client := &OSSClient{
		cfg: OSSConfig{
			Region: "oss-cn-shanghai",
			Bucket: "youlan-warehouse",
		},
	}

	got := client.PublicURL("data-warehouse/result.xlsx")
	want := "https://youlan-warehouse.oss-cn-shanghai.aliyuncs.com/data-warehouse/result.xlsx"
	if got != want {
		t.Fatalf("PublicURL() = %q, want %q", got, want)
	}
}
