package docx_test

import (
	"bytes"
	"errors"
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

// TestMutationFixture_SetPropertyPersists opens a real Word fixture, mutates
// one paragraph property and one run property, saves, reopens, and verifies:
//   - the two mutations persisted
//   - all paragraph texts are unchanged (no text corruption)
//   - the sibling paragraph's SpaceBefore is untouched (neighbour isolation)
func TestMutationFixture_SetPropertyPersists(t *testing.T) {
	fixturePath := filepath.Join("..", "testdata", "fixtures", "word", "plain_paragraph.docx")
	data, err := os.ReadFile(fixturePath)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("fixture not found: %s", fixturePath)
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	orig, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}

	// Collect paragraphs and record pre-mutation state.
	var paras []*docx.Paragraph
	for p := range orig.Body().Paragraphs() {
		paras = append(paras, p)
	}
	if len(paras) == 0 {
		t.Skip("fixture has no paragraphs")
	}
	origTexts := paragraphTexts(orig)

	// Record sibling SpaceBefore before mutation (for neighbour-isolation check).
	var origSibSB *docx.Twip
	if len(paras) > 1 {
		origSibSB = paras[1].Format().SpaceBefore()
	}

	// --- Mutation 1: set a distinctive SpaceBefore on paragraph 0. ---
	const wantSB docx.Twip = 720 // 0.5 inch; unlikely to already be set
	sb := wantSB
	paras[0].Format().SetSpaceBefore(&sb)

	// --- Mutation 2: force Bold on the first run of paragraph 0 (if any). ---
	var firstRunText string
	var mutatedRun bool
	for r := range paras[0].Runs() {
		firstRunText = r.Text()
		r.Font().SetBold(docx.On())
		mutatedRun = true
		break
	}

	// Save → reopen.
	var buf bytes.Buffer
	if err := orig.Save(&buf); err != nil {
		t.Fatalf("save after mutation: %v", err)
	}
	rt, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen after mutation: %v", err)
	}

	// All paragraph texts must be byte-identical.
	rtTexts := paragraphTexts(rt)
	if len(rtTexts) != len(origTexts) {
		t.Fatalf("paragraph count changed: was %d, now %d", len(origTexts), len(rtTexts))
	}
	for i := range origTexts {
		if rtTexts[i] != origTexts[i] {
			t.Errorf("paragraph[%d] text changed: was %q, now %q", i, origTexts[i], rtTexts[i])
		}
	}

	var rtParas []*docx.Paragraph
	for p := range rt.Body().Paragraphs() {
		rtParas = append(rtParas, p)
	}

	// Mutation 1 must have persisted.
	if got := rtParas[0].Format().SpaceBefore(); got == nil || *got != wantSB {
		t.Errorf("SpaceBefore not persisted: got %v, want %v", got, wantSB)
	}

	// Mutation 2 must have persisted.
	if mutatedRun {
		var rtFirstRun *docx.Run
		for r := range rtParas[0].Runs() {
			rtFirstRun = r
			break
		}
		if rtFirstRun == nil {
			t.Fatal("first run disappeared after mutation round-trip")
		}
		if rtFirstRun.Text() != firstRunText {
			t.Errorf("first run text changed: was %q, now %q", firstRunText, rtFirstRun.Text())
		}
		if b := rtFirstRun.Font().Bold(); b == nil || !*b {
			t.Error("Bold not persisted after mutation round-trip")
		}
	}

	// Sibling paragraph (index 1) SpaceBefore must be untouched.
	if len(paras) > 1 && len(rtParas) > 1 {
		rtSibSB := rtParas[1].Format().SpaceBefore()
		switch {
		case origSibSB == nil && rtSibSB != nil:
			t.Errorf("sibling paragraph[1] SpaceBefore was nil, now %v", rtSibSB)
		case origSibSB != nil && rtSibSB == nil:
			t.Errorf("sibling paragraph[1] SpaceBefore was %v, now nil", *origSibSB)
		case origSibSB != nil && rtSibSB != nil && *origSibSB != *rtSibSB:
			t.Errorf("sibling paragraph[1] SpaceBefore changed: was %v, now %v", *origSibSB, *rtSibSB)
		}
	}
}
