package level05_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const module = "example.com/todo/level05/"

// TestDependencyRule makes the Dependency Rule executable: the business core (entity and usecase)
// may import only the standard library and the packages listed here. Importing Gin, GORM, Mongo,
// zerolog, an adapter or the infrastructure from there fails this test. Each package may import
// itself, because its external _test package does.
func TestDependencyRule(t *testing.T) {
	allowed := map[string][]string{
		"internal/entity":  {"github.com/google/uuid", module + "internal/entity"},
		"internal/usecase": {module + "internal/entity", module + "internal/usecase"},
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

// importsIn parses every Go file in dir, tests included, and returns its imports by file name.
func importsIn(t *testing.T, dir string) map[string][]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no Go files in %s: did the package move?", dir)
	}
	result := map[string][]string{}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range f.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			result[file] = append(result[file], path)
		}
	}
	return result
}

// isStdlib reports whether path is a standard library package: those have no dot in their first element.
func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
