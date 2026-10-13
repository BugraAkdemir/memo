package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runSearch(t *testing.T, root string, args SearchContentArgs) (string, error) {
	t.Helper()
	raw, _ := json.Marshal(args)
	return SearchContent(context.Background(), raw, root, nil)
}

func TestSearchContent_FindsDefinitionAndEveryUse(t *testing.T) {
	root := writeTree(t, map[string]string{
		"main.go":          "package main\n\nfunc parseConfig() error { return nil }\n\nfunc main() { _ = parseConfig() }\n",
		"sub/other.go":     "package sub\n\nfunc g() { parseConfig() }\n",
		"README.md":        "nothing here\n",
		"sub/unrelated.go": "package sub\n",
	})
	out, err := runSearch(t, root, SearchContentArgs{Pattern: `parseConfig`})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"main.go:3:", "main.go:5:", "sub/other.go:3:", "3 match(es) in 2 file(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "README.md") {
		t.Errorf("a file without a match was listed:\n%s", out)
	}
}

func TestSearchContent_GlobIgnoreCaseAndRegex(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.go": "TODO fix\n",
		"b.md": "todo docs\n",
	})
	out, _ := runSearch(t, root, SearchContentArgs{Pattern: `todo`, IgnoreCase: true, Glob: "*.go"})
	if !strings.Contains(out, "a.go:1:") || strings.Contains(out, "b.md") {
		t.Errorf("glob/ignore_case wrong:\n%s", out)
	}
	if out, _ := runSearch(t, root, SearchContentArgs{Pattern: `todo`}); !strings.Contains(out, "b.md:1:") || strings.Contains(out, "a.go") {
		t.Errorf("case-sensitive search should only match lower-case todo:\n%s", out)
	}
	if out, _ := runSearch(t, root, SearchContentArgs{Pattern: `^T[A-Z]+O\b`}); !strings.Contains(out, "a.go:1:") {
		t.Errorf("regex not applied:\n%s", out)
	}
}

func TestSearchContent_SkipsDependenciesBinariesAndHugeFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"src/real.go":               "needle\n",
		"node_modules/pkg/index.js": "needle\n",
		".git/config":               "needle\n",
		"bin/blob":                  "needle\x00\x01binary",
		"big.txt":                   strings.Repeat("x", searchContentMaxFileSize+10) + "needle",
	})
	out, _ := runSearch(t, root, SearchContentArgs{Pattern: "needle"})
	if !strings.Contains(out, "src/real.go:1:") || !strings.Contains(out, "1 match(es)") {
		t.Fatalf("only src/real.go should match:\n%s", out)
	}
}

func TestSearchContent_ContextLinesAndLimit(t *testing.T) {
	root := writeTree(t, map[string]string{"f.txt": "a\nb\nHIT\nc\nd\ne\nf\nHIT\ng\n"})
	out, _ := runSearch(t, root, SearchContentArgs{Pattern: "HIT", Context: 1})
	for _, want := range []string{"f.txt:2- b", "f.txt:3: HIT", "f.txt:4- c", "--", "f.txt:8: HIT"} {
		if !strings.Contains(out, want) {
			t.Errorf("context output missing %q:\n%s", want, out)
		}
	}
	many := strings.Repeat("HIT\n", 50)
	root = writeTree(t, map[string]string{"m.txt": many})
	out, _ = runSearch(t, root, SearchContentArgs{Pattern: "HIT", MaxResults: 5})
	if !strings.Contains(out, "stopped at 5 matches") || strings.Count(out, "m.txt:") != 5 {
		t.Errorf("limit not enforced:\n%s", out)
	}
}

func TestSearchContent_ErrorsAreActionable(t *testing.T) {
	root := writeTree(t, map[string]string{"f.txt": "x\n"})
	if _, err := runSearch(t, root, SearchContentArgs{Pattern: "func("}); err == nil || !strings.Contains(err.Error(), "invalid regular expression") {
		t.Errorf("a bad regex should say so and how to escape: %v", err)
	}
	if _, err := runSearch(t, root, SearchContentArgs{Pattern: ""}); err == nil {
		t.Error("an empty pattern must be rejected")
	}
	if out, err := runSearch(t, root, SearchContentArgs{Pattern: "zzz"}); err != nil || !strings.Contains(out, "No matches") {
		t.Errorf("no match: %q, %v", out, err)
	}
	if _, err := runSearch(t, root, SearchContentArgs{Pattern: "x", Path: "../../etc"}); err == nil {
		t.Error("a path outside the project must be refused")
	}
}

func TestSearchContent_SingleFilePath(t *testing.T) {
	root := writeTree(t, map[string]string{"a.go": "one\n", "b.go": "one\n"})
	out, _ := runSearch(t, root, SearchContentArgs{Pattern: "one", Path: "a.go"})
	if !strings.Contains(out, "a.go:1:") || strings.Contains(out, "b.go") {
		t.Errorf("single-file search wrong:\n%s", out)
	}
}
