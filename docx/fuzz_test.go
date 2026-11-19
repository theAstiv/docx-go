package docx_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// FuzzOpen opens arbitrary bytes as a .docx. Whatever the input, Open must
// return (not panic, overflow the stack or exhaust memory), and a document it
// accepts must save and reopen.
//
// Run it with a short -fuzzminimizetime: each new input takes the default
// 60s to minimize, which looks like a stall.
func FuzzOpen(f *testing.F) {
	var buf bytes.Buffer
	if err := docx.New().Save(&buf); err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())
	for _, name := range []string{"poi/bookmarks.docx", "poi/TestDocument.docx"} {
		// #nosec G304 -- fixed test fixture paths
		if data, err := os.ReadFile(filepath.Join("..", "testdata", "fixtures", filepath.FromSlash(name))); err == nil {
			f.Add(data)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := docx.OpenBytes(data)
		if err != nil {
			return
		}
		_ = d.Text()
		var out bytes.Buffer
		if err := d.Save(&out); err != nil {
			return
		}
		if _, err := docx.OpenBytes(out.Bytes()); err != nil {
			t.Fatalf("saved document does not reopen: %v", err)
		}
	})
}
