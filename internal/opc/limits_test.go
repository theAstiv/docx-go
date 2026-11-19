package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func zipOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func nested(depth int) string {
	return strings.Repeat("<a>", depth) + strings.Repeat("</a>", depth)
}

func TestReadZipLimits(t *testing.T) {
	lim := limits{entries: 3, entry: 100, total: 150, depth: 5}
	tests := []struct {
		name    string
		entries map[string]string
		wantErr bool
	}{
		{"within limits", map[string]string{"a.xml": nested(5), "b.bin": strings.Repeat("x", 100)}, false},
		{"too many entries", map[string]string{"a": "", "b": "", "c": "", "d": ""}, true},
		{"entry too large", map[string]string{"a.bin": strings.Repeat("x", 101)}, true},
		{"total too large", map[string]string{"a.bin": strings.Repeat("x", 80), "b.bin": strings.Repeat("x", 80)}, true},
		{"xml too deep", map[string]string{"word/document.xml": nested(6)}, true},
		{"rels too deep", map[string]string{"_rels/.rels": nested(6)}, true},
		{"upper-case xml name too deep", map[string]string{"Word/Document.XML": nested(6)}, true},
		{"deep non-xml part is not scanned", map[string]string{"a.bin": nested(6)}, false},
		{"malformed xml is left to the parser", map[string]string{"a.xml": "<a><b></a>"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := zipOf(t, tt.entries)
			_, err := readZip(bytes.NewReader(data), int64(len(data)), lim)
			if tt.wantErr {
				if !errors.Is(err, ErrLimitExceeded) {
					t.Fatalf("err = %v, want ErrLimitExceeded", err)
				}
			} else if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}

// TestReadZipLyingSize checks that an entry whose header understates its
// size cannot read past the limit.
func TestReadZipLyingSize(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{Name: "a.bin", Method: zip.Store, CompressedSize64: 200, UncompressedSize64: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte("x"), 200)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	parts, err := readZip(bytes.NewReader(data), int64(len(data)), limits{entries: 10, entry: 100, total: 100, depth: 10})
	if err == nil && len(parts["a.bin"]) > 100 {
		t.Fatalf("read %d bytes past the 100-byte limit", len(parts["a.bin"]))
	}
}
