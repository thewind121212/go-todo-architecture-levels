package level06_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const module = "example.com/todo/level06/"

// TestDependencyRule keeps the hexagon's dependencies pointing inward: domain, ports and service
// may import only the standard library and the packages listed here. Importing Gin, GORM, Mongo,
// zerolog or an adapter from the core fails this test. Test files are skipped: from this level on
// they use testify and the generated mocks.
func TestDependencyRule(t *testing.T) {
	allowed := map[string][]string{
		"internal/todo/domain":  {"github.com/google/uuid"},
		"internal/todo/ports":   {module + "internal/todo/domain"},
		"internal/todo/service": {module + "internal/todo/domain", module + "internal/todo/ports"},
	}
	for dir, allow := range allowed {
		for file, imports := range importsIn(t, dir) {
			for _, imp := range imports {
				if !isStdlib(imp) && !slices.Contains(allow, imp) {
					t.Errorf("%s imports %s: %s may import only the standard library and %v", file, imp, dir, allow)
				}
			}
		}
	}
}

// importsIn parses every non-test Go file in dir and returns its imports by file name.
func importsIn(t *testing.T, dir string) map[string][]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][]string{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range f.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			result[file] = append(result[file], path)
		}
	}
	if len(result) == 0 {
		t.Fatalf("no Go files in %s: did the package move?", dir)
	}
	return result
}

// isStdlib reports whether path is a standard library package: those have no dot in their first element.
func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
