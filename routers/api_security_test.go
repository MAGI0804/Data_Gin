package routers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
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
