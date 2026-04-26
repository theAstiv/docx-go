package docx_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// TestRoundTrip_Fixtures opens every .docx under testdata/fixtures/, saves it,
// reopens the saved bytes, and asserts that paragraph texts are identical.
// Files beginning with "~$" (Word lock files) are skipped.
// The test is skipped entirely when no fixtures are found so CI passes before
// the fixture corpus is committed.
func TestRoundTrip_Fixtures(t *testing.T) {
	fixtureRoot := filepath.Join("..", "testdata", "fixtures")
	if _, err := os.Stat(fixtureRoot); os.IsNotExist(err) {
		t.Skipf("fixture directory %s not found", fixtureRoot)
	}

	var paths []string
	if err := filepath.WalkDir(fixtureRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasSuffix(path, ".docx") && !strings.HasPrefix(base, "~$") {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk fixtures: %v", err)
	}

	if len(paths) == 0 {
		t.Skip("no .docx fixtures found")
	}

	for _, path := range paths {
		// Use source/filename as the subtest name for readability.
		rel, _ := filepath.Rel(fixtureRoot, path)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			t.Parallel()
			roundTripFixture(t, path)
		})
	}
}

func roundTripFixture(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// --- open original ---
	orig, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	origTexts := paragraphTexts(orig)

	// --- save ---
	var buf bytes.Buffer
	if err := orig.Save(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}

	// --- reopen ---
	rt, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen after save: %v", err)
	}
	rtTexts := paragraphTexts(rt)

	// --- compare ---
	if len(rtTexts) != len(origTexts) {
		t.Fatalf("paragraph count changed: original=%d round-tripped=%d",
			len(origTexts), len(rtTexts))
	}
	for i := range origTexts {
		if rtTexts[i] != origTexts[i] {
			t.Errorf("paragraph[%d] mismatch:\n  original:     %q\n  round-tripped: %q",
				i, origTexts[i], rtTexts[i])
		}
	}
}

func paragraphTexts(d *docx.Document) []string {
	var out []string
	for p := range d.Body().Paragraphs() {
		out = append(out, p.Text())
	}
	return out
}
