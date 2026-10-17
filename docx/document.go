package docx

import (
	"bytes"
	"fmt"
	"io"

	"github.com/theAstiv/docx-go/internal/parts"
)

// ValidationIssue is a structured diagnostic returned by Document.Validate.
type ValidationIssue struct {
	Severity string
	Message  string
}

// Document is the top-level proxy for a .docx file.
// It wraps a parts.Package and provides ergonomic access to the document
// content without exposing encoding/xml or internal/opc to callers.
type Document struct {
	pkg *parts.Package
}

// New creates a new empty Document with a minimal valid structure.
func New() *Document {
	return &Document{pkg: parts.New()}
}

// Open reads a .docx from r and returns a Document proxy.
func Open(r io.ReaderAt, size int64) (*Document, error) {
	pkg, err := parts.Open(r, size)
	if err != nil {
		return nil, fmt.Errorf("docx: open: %w", err)
	}
	return &Document{pkg: pkg}, nil
}

// OpenBytes is a convenience wrapper for in-memory .docx data.
func OpenBytes(data []byte) (*Document, error) {
	return Open(bytes.NewReader(data), int64(len(data)))
}

// Save writes the document to w as a .docx ZIP.
func (d *Document) Save(w io.Writer) error {
	if err := d.pkg.Save(w); err != nil {
		return fmt.Errorf("docx: save: %w", err)
	}
	return nil
}

// Body returns the document body proxy.
func (d *Document) Body() *Body {
	return &Body{docPart: d.pkg.DocPart, st: docStory(d.pkg)}
}

// EvenAndOddHeaders reports whether separate even/odd page headers/footers are
// enabled for this document. Requires the settings part to be present.
func (d *Document) EvenAndOddHeaders() bool {
	if d.pkg.SetPart == nil {
		return false
	}
	b := d.pkg.SetPart.CT().EvenAndOddHeaders()
	return b != nil && *b
}

// SetEvenAndOddHeaders enables or disables separate even/odd page headers/footers.
// It creates the settings part on first call if not already present.
func (d *Document) SetEvenAndOddHeaders(enabled bool) {
	ct := d.pkg.EnsureSettingsPart().CT()
	v := enabled
	ct.SetEvenAndOddHeaders(&v)
}

// DefaultTabStop returns the default tab-stop width in twips, or nil if not set.
func (d *Document) DefaultTabStop() *Twip {
	if d.pkg.SetPart == nil {
		return nil
	}
	v := d.pkg.SetPart.CT().DefaultTabStop()
	if v == nil {
		return nil
	}
	t := Twip(*v)
	return &t
}

// SetDefaultTabStop sets the default tab-stop width. nil clears it.
// It creates the settings part on first call if not already present.
func (d *Document) SetDefaultTabStop(v *Twip) {
	ct := d.pkg.EnsureSettingsPart().CT()
	if v == nil {
		ct.SetDefaultTabStop(nil)
		return
	}
	n := int64(*v)
	ct.SetDefaultTabStop(&n)
}

// Validate performs basic structural checks and returns any issues found.
// It always returns []ValidationIssue (never error); structural problems are
// encoded as issues, not Go errors.
func (d *Document) Validate() []ValidationIssue {
	var issues []ValidationIssue
	if d.pkg.DocPart == nil || d.pkg.DocPart.Body() == nil {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Message:  "document has no body element",
		})
	}
	return issues
}
