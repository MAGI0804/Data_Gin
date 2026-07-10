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

func TestClientEndpointUsesInternalWhenEnabled(t *testing.T) {
	cfg := OSSConfig{
		Region:      "oss-cn-shanghai",
		Endpoint:    "https://oss-cn-shanghai.aliyuncs.com",
		UseInternal: true,
	}

	got := clientEndpoint(cfg)
	want := "https://oss-cn-shanghai-internal.aliyuncs.com"
	if got != want {
		t.Fatalf("clientEndpoint() = %q, want %q", got, want)
	}
}

func TestClientEndpointKeepsCNameWhenInternalEnabled(t *testing.T) {
	cfg := OSSConfig{
		Region:      "cn-shanghai",
		Endpoint:    "https://warehouse.youlankids.com",
		UseInternal: true,
		UseCName:    true,
	}

	got := clientEndpoint(cfg)
	want := "https://warehouse.youlankids.com"
	if got != want {
		t.Fatalf("clientEndpoint() = %q, want %q", got, want)
	}
}

func TestOSSClientUploadPlanMarksMultipart(t *testing.T) {
	client := &OSSClient{
		cfg: OSSConfig{
			Region:                  "cn-shanghai",
			Endpoint:                "https://oss-cn-shanghai.aliyuncs.com",
			MultipartThresholdBytes: 64 * 1024 * 1024,
			PartSizeBytes:           64 * 1024 * 1024,
			ParallelNum:             3,
			EnableCheckpoint:        true,
			CheckpointDir:           "/tmp/oss-checkpoints",
		},
	}

	plan := client.UploadPlan(500 * 1024 * 1024)
	if !plan.Multipart {
		t.Fatal("UploadPlan().Multipart = false, want true for large file")
	}
	if plan.PartSizeBytes != 64*1024*1024 || plan.ParallelNum != 3 || !plan.EnableCheckpoint {
		t.Fatalf("UploadPlan() = %+v, want configured multipart settings", plan)
	}
}
