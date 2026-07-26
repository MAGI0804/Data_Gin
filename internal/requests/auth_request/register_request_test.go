package auth_request

import "testing"

func TestRejectReservedAccount(t *testing.T) {
	tests := []struct {
		name    string
		account string
		wantErr bool
	}{
		{name: "exact admin", account: "admin", wantErr: true},
		{name: "uppercase admin", account: "ADMIN", wantErr: true},
		{name: "mixed case admin", account: "Admin", wantErr: true},
		{name: "admin with whitespace", account: " admin ", wantErr: true},
		{name: "ordinary account", account: "admin2", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := rejectReservedAccount(tt.account, nil)
			_, exists := errs["account"]
			if exists != tt.wantErr {
				t.Fatalf("rejectReservedAccount(%q) error exists = %t, want %t", tt.account, exists, tt.wantErr)
			}
		})
	}
}
