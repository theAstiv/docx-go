package parts

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"regexp"
	"testing"
)

// paragraphAndRunIDs matches the id attributes Word writes on w:p and w:r,
// whatever prefix the writer chose for their namespace.
var paragraphAndRunIDs = regexp.MustCompile(`:(paraId|textId|rsidR|rsidRPr|rsidRDefault|rsidP)="[0-9A-F]{8}"`)

// TestSave_PreservesParagraphAndRunAttrs checks that an unchanged save keeps
// every rsid and w14 id that Word wrote on paragraphs and runs.
func TestSave_PreservesParagraphAndRunAttrs(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/word/plain_paragraph.docx")
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Open(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := pkg.Save(&out); err != nil {
		t.Fatal(err)
	}

	want := paragraphAndRunIDs.FindAll(zipEntry(t, src, "word/document.xml"), -1)
	got := paragraphAndRunIDs.FindAll(zipEntry(t, out.Bytes(), "word/document.xml"), -1)
	if len(want) == 0 {
		t.Fatal("fixture has no paragraph or run ids; the test proves nothing")
	}
	if len(got) != len(want) {
		t.Fatalf("saved document has %d paragraph/run id attributes, original has %d", len(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("attribute %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func zipEntry(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return b
}
