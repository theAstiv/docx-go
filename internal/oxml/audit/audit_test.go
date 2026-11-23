//go:build roundtripaudit

package audit_test

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/nsfix"
	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"

	// Every package that registers element types. A new oxml package that
	// calls registry.Register must be added here, or its types escape the
	// audit.
	_ "github.com/theAstiv/docx-go/internal/oxml/dml"
	_ "github.com/theAstiv/docx-go/internal/oxml/docprops"
	_ "github.com/theAstiv/docx-go/internal/oxml/wml"
)

// samplesPerType bounds the fixture samples kept for one element type.
const samplesPerType = 8

type sample struct {
	from string // fixture file and part, or the sample file
	data []byte
}

// TestRoundTripAudit reads every registered element type back from real
// content and requires the output to equal the input: samples are taken
// from the fixture corpus (testdata/fixtures/**/*.docx), and a type no
// fixture holds needs a hand-written sample in testdata/roundtrip, named
// <prefix>-<local>.xml. A type with neither fails the audit.
func TestRoundTripAudit(t *testing.T) {
	names := registry.Registered()
	sort.Slice(names, func(i, j int) bool { return label(names[i]) < label(names[j]) })
	if len(names) < 40 {
		t.Fatalf("only %d registered types: an oxml package is missing from the imports", len(names))
	}
	want := map[xml.Name]bool{}
	for _, n := range names {
		want[n] = true
	}
	samples := fixtureSamples(t, want)
	for _, n := range names {
		path := filepath.Join("..", "..", "..", "testdata", "roundtrip", label(n)+".xml")
		if data, err := os.ReadFile(path); err == nil {
			samples[n] = append(samples[n], sample{from: path, data: bytes.TrimSpace(data)})
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	for _, n := range names {
		t.Run(label(n), func(t *testing.T) {
			if len(samples[n]) == 0 {
				t.Fatalf("no round-trip sample: no fixture holds %s; add testdata/roundtrip/%s.xml", label(n), label(n))
			}
			for _, s := range samples[n] {
				el := registry.New(n)
				if el == nil {
					t.Fatalf("registry.New(%v) = nil", n)
				}
				if err := roundtrip.Fidelity(el, s.data); err != nil {
					t.Errorf("%s: %v", s.from, err)
				}
			}
		})
	}
}

func label(n xml.Name) string {
	p := ns.Prefix(n.Space)
	if p == "" {
		p = "x"
	}
	return p + "-" + n.Local
}

// fixtureSamples collects up to samplesPerType occurrences of each wanted
// element from the fixture corpus, each re-encoded on its own with the
// namespace declarations it needs.
func fixtureSamples(t *testing.T, want map[xml.Name]bool) map[xml.Name][]sample {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "testdata", "fixtures", "*", "*.docx"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	out := map[xml.Name][]sample{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, zf := range zr.File {
			if !strings.HasSuffix(zf.Name, ".xml") {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				t.Fatal(err)
			}
			part, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			collect(t, filepath.Base(f)+"!"+zf.Name, part, want, out)
		}
	}
	return out
}

// requiredParent names element types whose QName is shared with a
// different schema type elsewhere: w:tab is run content (CT_Tab, the type
// registered) inside w:r, but a tab stop (CT_TabStop) inside w:tabs.
var requiredParent = map[xml.Name]xml.Name{
	ns.Qn("w:tab"): ns.Qn("w:r"),
}

// collect adds the wanted elements of one part to out.
func collect(t *testing.T, from string, part []byte, want map[xml.Name]bool, out map[xml.Name][]sample) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(part))
	for {
		tok, err := dec.Token()
		if err != nil {
			return // io.EOF, or a part that is not XML
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		// Capture each top-level element and walk it: nested wanted
		// elements are sampled from inside the capture.
		re := new(raw.RawElement)
		if err := re.UnmarshalXML(dec, se); err != nil {
			t.Fatalf("%s: capture %v: %v", from, se.Name, err)
		}
		stripLayout(re)
		walk(t, from, re, xml.Name{}, want, out)
	}
}

// walk samples re and the wanted elements below it; parent is re's parent.
func walk(t *testing.T, from string, re *raw.RawElement, parent xml.Name, want map[xml.Name]bool, out map[xml.Name][]sample) {
	t.Helper()
	if want[re.QName] && len(out[re.QName]) < samplesPerType {
		if p, ok := requiredParent[re.QName]; !ok || p == parent {
			out[re.QName] = append(out[re.QName], sample{from: from, data: encode(t, from, re)})
		}
	}
	for _, c := range re.Children {
		if k, ok := c.(*raw.RawElement); ok {
			walk(t, from, k, re.QName, want, out)
		}
	}
}

// encode writes re on its own, with the namespace declarations it needs.
func encode(t *testing.T, from string, re *raw.RawElement) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := re.MarshalXML(enc, xml.StartElement{}); err != nil {
		t.Fatalf("%s: re-encode %v: %v", from, re.QName, err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatal(err)
	}
	fixed, err := nsfix.Fix(buf.Bytes())
	if err != nil {
		t.Fatalf("%s: nsfix %v: %v", from, re.QName, err)
	}
	return fixed
}

// stripLayout drops whitespace-only text from elements that have element
// children: indentation in a pretty-printed part, which is not content in
// WordprocessingML's element-only types. Text of leaf elements (a w:t
// holding a space) is kept.
func stripLayout(re *raw.RawElement) {
	hasElem := false
	for _, c := range re.Children {
		if _, ok := c.(*raw.RawElement); ok {
			hasElem = true
			break
		}
	}
	kept := re.Children[:0]
	for _, c := range re.Children {
		switch x := c.(type) {
		case xml.CharData:
			if hasElem && len(bytes.TrimSpace(x)) == 0 {
				continue
			}
		case *raw.RawElement:
			stripLayout(x)
		}
		kept = append(kept, c)
	}
	re.Children = kept
}
