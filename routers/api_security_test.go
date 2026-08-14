package routers

import (
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
