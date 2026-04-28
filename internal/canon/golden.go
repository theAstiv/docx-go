package canon

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files instead of asserting them")

// Assert canonicalises got and either writes it to testdata/golden/xml/{name}.xml
// (when -update is passed) or asserts it equals the stored snapshot.
//
// name should be a stable identifier like "fixtures/word/plain/word/document".
func Assert(t *testing.T, name string, got []byte) {
	t.Helper()

	canonical, err := XML(got)
	if err != nil {
		t.Fatalf("canon.Assert: canonicalise: %v", err)
	}

	path := goldenPath(t, name)

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("canon.Assert: mkdir: %v", err)
		}
		if err := os.WriteFile(path, canonical, 0o644); err != nil { //nolint:gosec // G306: golden test artifacts are intentionally world-readable
			t.Fatalf("canon.Assert: write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("canon.Assert: golden file %s does not exist; run with -update to create it", path)
		}
		t.Fatalf("canon.Assert: read golden: %v", err)
	}

	if !equalBytes(canonical, want) {
		t.Errorf("canon.Assert: %s: output differs from golden\ngot:  %s\nwant: %s", name, canonical, want)
	}
}

// goldenPath returns the path to the golden file for a test name.
// It resolves relative to the module root by walking up from the test's
// working directory until go.mod is found.
func goldenPath(t *testing.T, name string) string {
	t.Helper()
	root := moduleRoot(t)
	return filepath.Join(root, "testdata", "golden", "xml", filepath.FromSlash(name)+".xml")
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
