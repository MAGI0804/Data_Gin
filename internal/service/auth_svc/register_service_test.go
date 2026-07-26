package auth_svc

import (
	"testing"

	"gin-biz-web-api/internal/requests/auth_request"
)

func TestRegisterServiceRejectsConsoleAdminBeforeDatabase(t *testing.T) {
	service := NewRegisterService()
	for _, account := range []string{"admin", "Admin", "ADMIN", " admin "} {
		t.Run(account, func(t *testing.T) {
			token := service.CreateUserToken(nil, auth_request.SignupUsingEmailRequest{Account: account})
			if token != "" {
				t.Fatalf("CreateUserToken(%q) token = %q, want empty", account, token)
			}
		})
	}
}
