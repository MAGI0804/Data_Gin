package routers

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterAPIRoutesDoesNotRegisterDemoHandlers(t *testing.T) {
	t.Parallel()

	path := filepath.Join("api.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	forbidden := map[string]struct{}{
		"apiTest":    {},
		"apiExample": {},
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		if _, exists := forbidden[identifier.Name]; exists {
			t.Errorf("RegisterAPIRoutes must not register demo handler %s", identifier.Name)
		}
		return true
	})
}

func TestHealthRoutesAcceptDockerHeadProbe(t *testing.T) {
	t.Parallel()

	router := gin.New()
	registerHealthRoutes(router, router.Group("/api"))

	for _, path := range []string{"/health", "/api/health"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodHead, path, nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("HEAD %s status = %d, want %d", path, response.Code, http.StatusOK)
			}
			if response.Body.Len() != 0 {
				t.Fatalf("HEAD %s body = %q, want empty", path, response.Body.String())
			}
		})
	}
}

func TestDatabaseReadiness(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		ping       databasePing
		wantStatus int
		wantBody   bool
	}{
		{name: "database available", method: http.MethodGet, ping: func(context.Context) error { return nil }, wantStatus: http.StatusOK, wantBody: true},
		{name: "database unavailable", method: http.MethodGet, ping: func(context.Context) error { return errors.New("connection refused") }, wantStatus: http.StatusServiceUnavailable, wantBody: true},
		{name: "missing database", method: http.MethodGet, wantStatus: http.StatusServiceUnavailable, wantBody: true},
		{name: "head omits body", method: http.MethodHead, ping: func(context.Context) error { return nil }, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Handle(tt.method, "/api/ready", databaseReadiness(tt.ping))
			request := httptest.NewRequest(tt.method, "/api/ready", nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if tt.wantBody && response.Body.Len() == 0 {
				t.Fatal("response body is empty")
			}
			if !tt.wantBody && response.Body.Len() != 0 {
				t.Fatalf("response body = %q, want empty", response.Body.String())
			}
		})
	}
}
