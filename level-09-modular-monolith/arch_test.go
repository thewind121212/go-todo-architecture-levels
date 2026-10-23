package level09_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const module = "example.com/todo/level09/"

// TestDependencyRule keeps the dependencies inside each package pointing inward: the core
// packages may import only the standard library and the packages listed here. Importing
// Gin, GORM, Mongo or an adapter from the core fails this test. Test files are skipped:
// they use testify and the generated mocks.
//
// The todo module's rules are level 8's. The query side still does not import the domain;
// the projector now reads the module's contract events instead of its domain events.
func TestDependencyRule(t *testing.T) {
	const (
		todo      = module + "internal/modules/todo/"
		domain    = todo + "internal/domain"
		ports     = todo + "internal/ports"
		query     = todo + "internal/application/query"
		codec     = todo + "internal/codec"
		contracts = todo + "contracts"
		outbox    = module + "internal/platform/outbox"
		eventbus  = module + "internal/platform/eventbus"
	)
	allowed := map[string][]string{
		"internal/modules/todo/internal/domain":                 {"github.com/google/uuid"},
		"internal/modules/todo/internal/ports":                  {domain, query},
		"internal/modules/todo/internal/application/command":    {domain, ports},
		"internal/modules/todo/internal/application/query":      {"github.com/google/uuid"},
		"internal/modules/todo/internal/application/projection": {query, contracts, eventbus},
		// The one place where domain events become contract events.
		"internal/modules/todo/internal/application/integration": {domain, codec, contracts, outbox, eventbus},
		"internal/modules/todo/internal/codec":                   {domain, outbox},
		// The published contract depends on nothing, so any module can import it cheaply.
		"internal/modules/todo/contracts":                {},
		"internal/modules/activity/internal/application": {"github.com/google/uuid", contracts, eventbus},
		// The platform knows no database (adapters implement its ports) and no module.
		"internal/platform/outbox":   {"github.com/rs/zerolog"},
		"internal/platform/eventbus": {},
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

// TestModuleBoundaries checks what the compiler cannot. Go's internal/ rule already stops
// one module from importing another's internal packages, but it allows the root package
// (module.go). These rules close that gap:
//   - the activity module may import only the todo module's contracts;
//   - the todo module imports nothing from the activity module: it does not know who listens;
//   - the platform imports no module: it is shared by all of them.
func TestModuleBoundaries(t *testing.T) {
	const (
		todo     = module + "internal/modules/todo"
		activity = module + "internal/modules/activity"
		modules  = module + "internal/modules"
	)
	rules := []struct {
		dir       string
		forbidden string   // an import with this path, or under it, breaks the rule...
		except    []string // ...unless it is one of these
	}{
		{"internal/modules/activity", todo, []string{todo + "/contracts"}},
		{"internal/modules/todo", activity, nil},
		{"internal/platform", modules, nil},
	}
	for _, rule := range rules {
		for file, imports := range importsUnder(t, rule.dir) {
			for _, imp := range imports {
				inside := imp == rule.forbidden || strings.HasPrefix(imp, rule.forbidden+"/")
				if inside && !slices.Contains(rule.except, imp) {
					t.Errorf("%s imports %s: %s may not import %s (allowed: %v)", file, imp, rule.dir, rule.forbidden, rule.except)
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
		result[file] = parseImports(t, file)
	}
	if len(result) == 0 {
		t.Fatalf("no Go files in %s: did the package move?", dir)
	}
	return result
}

// importsUnder is importsIn for dir and every directory below it.
func importsUnder(t *testing.T, dir string) map[string][]string {
	t.Helper()
	result := map[string][]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		result[path] = parseImports(t, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Fatalf("no Go files under %s: did the module move?", dir)
	}
	return result
}

func parseImports(t *testing.T, file string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	var imports []string
	for _, spec := range f.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		imports = append(imports, path)
	}
	return imports
}

// isStdlib reports whether path is a standard library package: those have no dot in their first element.
func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
