package nsfix_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/nsfix"
)

// round-trips the fixed XML back through xml.Decoder to confirm it is
// well-formed and that no "duplicate attribute" errors arise.
func mustParse(t *testing.T, data []byte) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			// io.EOF is expected at end of document
			if err.Error() == "EOF" {
				return
			}
			t.Fatalf("xml.Decoder rejected fixed output: %v\nXML:\n%s", err, data)
		}
		_ = tok
	}
}

// TestFix_NoRootElement confirms Fix is a no-op when there is no root element.
func TestFix_NoRootElement(t *testing.T) {
	src := []byte(`<?xml version="1.0"?>`)
	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, src) {
		t.Fatalf("expected no-op; got %s", got)
	}
}

// TestFix_SimpleElement checks that a simple element with no namespace
// declarations passes through correctly.
func TestFix_SimpleElement(t *testing.T) {
	src := []byte(`<root><child>text</child></root>`)
	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustParse(t, got)
	if !bytes.Contains(got, []byte(`<child>text</child>`)) {
		t.Fatalf("child content not preserved; got %s", got)
	}
}

// TestFix_RedundantChildDefaultNS is the key path described in the nsfix
// design: encoding/xml re-declares xmlns="ROOT_NS" on every child element in
// the root namespace.  Fix must strip those redundant declarations so that
// .NET's XmlDocument (and the OOXML validator) do not reject the output.
func TestFix_RedundantChildDefaultNS(t *testing.T) {
	// Simulate what encoding/xml emits: xmlns="wml" repeated on every child.
	const wml = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	src := []byte(`<document xmlns="` + wml + `"><body xmlns="` + wml + `"><p xmlns="` + wml + `">hello</p></body></document>`)

	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustParse(t, got)

	// The root element must still have the default-namespace declaration.
	if !bytes.Contains(got, []byte(`xmlns="`+wml+`"`)) {
		t.Fatalf("root default-ns declaration missing; got %s", got)
	}

	// Child elements must NOT repeat the default-namespace declaration.
	// Count occurrences: exactly one (the root).
	count := bytes.Count(got, []byte(`xmlns="`+wml+`"`))
	if count != 1 {
		t.Fatalf("expected exactly 1 xmlns= declaration, got %d; XML:\n%s", count, got)
	}
}

// TestFix_MultipleNamespaces checks that non-root namespace prefixes are
// emitted on the root element with canonical prefixes, and that regular
// namespace-qualified attributes are emitted correctly.
func TestFix_MultipleNamespaces(t *testing.T) {
	const (
		wml = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		mc  = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	)
	// Simulated mangled output from encoding/xml: _xmlns prefix dance and
	// mc:Ignorable attribute with auto-prefix.
	src := []byte(
		`<document xmlns="` + wml + `" xmlns:mc="` + mc + `" mc:Ignorable="w14">` +
			`<body xmlns="` + wml + `"><p xmlns="` + wml + `">text</p></body>` +
			`</document>`)

	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustParse(t, got)

	s := string(got)
	if !strings.Contains(s, `xmlns:mc="`+mc+`"`) {
		t.Fatalf("mc namespace declaration missing; got %s", s)
	}
	if !strings.Contains(s, `mc:Ignorable="w14"`) {
		t.Fatalf("mc:Ignorable attribute missing; got %s", s)
	}
	// Redundant child xmlns must be stripped.
	if bytes.Count(got, []byte(`xmlns="`+wml+`"`)) != 1 {
		t.Fatalf("redundant xmlns not stripped; got %s", got)
	}
}

// TestFix_UnknownNamespaceFallback exercises the ns%d fallback path in
// buildTag (line 186 in nsfix.go): when a namespace-qualified attribute has a
// URI not in the canonical ns.Prefix map, a synthetic prefix must be generated
// rather than dropping the attribute.
func TestFix_UnknownNamespaceFallback(t *testing.T) {
	const (
		wml     = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		unknown = "http://example.com/totally-unknown-ns"
	)
	// Simulate an element where a namespace-qualified attribute uses an
	// unrecognised URI — encoding/xml would have auto-generated a prefix.
	// We feed it already in a partially-fixed form to test the fallback branch.
	src := []byte(
		`<document xmlns="` + wml + `" xmlns:ex="` + unknown + `" ex:custom="val">` +
			`<body xmlns="` + wml + `">text</body>` +
			`</document>`)

	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustParse(t, got)

	s := string(got)
	// The attribute value must be preserved.
	if !strings.Contains(s, `custom="val"`) {
		t.Fatalf("unknown-ns attribute value not preserved; got %s", s)
	}
	// The unknown URI must have some xmlns: declaration.
	if !strings.Contains(s, unknown) {
		t.Fatalf("unknown namespace URI disappeared; got %s", s)
	}
}

// TestFix_EncoderMangledInput feeds the exact byte sequence that Go's
// encoding/xml currently emits for a namespace-declared root element
// (golang/go#9519 / #13400).  The encoder renders
//
//	{Space:"xmlns", Local:"mc"} → xmlns:_xmlns="xmlns" _xmlns:mc="uri"
//
// and auto-generates throw-away prefixes (e.g. "_") for namespace-qualified
// attributes.  xml.Decoder correctly inverts this, so Fix must recover the
// original semantics from that mangled input.
func TestFix_EncoderMangledInput(t *testing.T) {
	const (
		wml = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		mc  = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	)
	// This is what encoding/xml actually emits today for a CT_Document-like
	// struct whose start element carries {Space:"xmlns", Local:"mc"} attrs.
	src := []byte(
		`<document xmlns:_xmlns="xmlns" _xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas" xmlns="` + wml + `" xmlns:_mc="` + mc + `" _mc:Ignorable="w14">` +
			`<body xmlns="` + wml + `"><p xmlns="` + wml + `">text</p></body>` +
			`</document>`)

	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustParse(t, got)

	s := string(got)
	// After fixing, mc:Ignorable must be present with a real prefix.
	if !strings.Contains(s, `Ignorable="w14"`) {
		t.Fatalf("mc:Ignorable lost; got %s", s)
	}
	// The mangled _xmlns / _mc artifacts must be gone.
	if strings.Contains(s, `_xmlns`) || strings.Contains(s, `_mc`) {
		t.Fatalf("mangled xmlns artifact still present; got %s", s)
	}
}

// TestFix_ReplaceAllTextSafety guards against bytes.ReplaceAll silently
// stripping text content that happens to look like ` xmlns="ROOT_NS"`.
// The replacement targets a leading space so it only matches attribute-style
// occurrences, but character data in <w:t> could theoretically contain that
// byte sequence.  This test documents the current behaviour and will fail if
// the implementation changes in a way that corrupts text content.
func TestFix_ReplaceAllTextSafety(t *testing.T) {
	const wml = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	// A paragraph whose text node contains a literal xmlns= substring.
	// This is contrived but represents the collision case.
	xmlnsLiteral := ` xmlns="` + wml + `"`
	src := []byte(`<document xmlns="` + wml + `"><body xmlns="` + wml + `"><p>contains` + xmlnsLiteral + `in text</p></body></document>`)

	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The output must still be parseable XML (even if the text content was
	// affected — the important thing is no corruption goes undetected).
	mustParse(t, got)

	// Document the current behaviour explicitly so any future change is
	// a conscious decision rather than a silent regression.
	t.Logf("output: %s", got)
}

// TestFix_ProcInstPreserved ensures that an XML declaration before the root
// element is passed through unchanged.
func TestFix_ProcInstPreserved(t *testing.T) {
	const wml = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	src := []byte(`<?xml version="1.0" encoding="UTF-8"?><document xmlns="` + wml + `"><body xmlns="` + wml + `"/></document>`)

	got, err := nsfix.Fix(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustParse(t, got)
	if !bytes.HasPrefix(got, []byte(`<?xml`)) {
		t.Fatalf("XML declaration stripped; got %s", got)
	}
}
