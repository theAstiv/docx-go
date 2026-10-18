package template

import (
	"errors"
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/internal/bridge"
	itemplate "github.com/theAstiv/docx-go/internal/template"
)

// Sentinel errors. Every Issue's Err wraps one of them; errors.Is also works
// on the *Error that Execute returns.
var (
	// ErrMissingKey: a value tag names a key the data does not contain
	// (Strict mode; a warning in Lenient mode).
	ErrMissingKey = itemplate.ErrMissingKey
	// ErrMalformedTag: "{{" without "}}", an empty or invalid path, or a tab
	// or line break inside a tag.
	ErrMalformedTag = itemplate.ErrMalformedTag
	// ErrUnbalancedBlock: a block opened and not closed, closed and not
	// opened, or closed by the wrong kind.
	ErrUnbalancedBlock = itemplate.ErrUnbalancedBlock
	// ErrMisplacedBlock: a block tag that is neither alone in its paragraph
	// nor at a table-row edge.
	ErrMisplacedBlock = itemplate.ErrMisplacedBlock
	// ErrSplitAcrossBoundary: a tag split across a field, bookmark,
	// hyperlink edge, content control or tracked change. Retype it in one
	// piece.
	ErrSplitAcrossBoundary = itemplate.ErrSplitAcrossBoundary
	// ErrUnsupportedValue: a value that cannot be written as text, such as
	// a list or object in a value tag.
	ErrUnsupportedValue = itemplate.ErrUnsupportedValue
	// ErrSectionBreakInBlock: a block range contains a section break.
	ErrSectionBreakInBlock = itemplate.ErrSectionBreakInBlock
	// ErrUnsupportedLocation: a tag where it cannot be filled, such as a
	// block tag inside a content control.
	ErrUnsupportedLocation = itemplate.ErrUnsupportedLocation
)

// Mode decides what a value tag with a missing key does.
type Mode int

const (
	// Strict (the default) fails and lists every missing key.
	Strict Mode = iota
	// Lenient writes empty text for a missing key and returns a warning.
	Lenient
)

// Options configures Execute.
type Options struct {
	Mode Mode
}

// Severity says whether an Issue stopped execution.
type Severity int

const (
	// SeverityError stopped execution; the document was left unchanged.
	SeverityError Severity = iota
	// SeverityWarning did not stop execution.
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Location says where in the package a tag sits.
type Location struct {
	// Part is the part's path inside the .docx, e.g. "word/header1.xml".
	Part string
	// Path is the element path inside the part, e.g. "body/tbl[0]/tr[2]/tc[1]/p[0]":
	// zero-based indexes among siblings of the same kind.
	Path string
}

func (l Location) String() string {
	if l.Path == "" {
		return l.Part
	}
	return l.Part + " › " + l.Path
}

// Issue is one problem found in a template or its data.
type Issue struct {
	Severity Severity
	Location Location
	// Tag is the tag as written, e.g. "{{client.name}}".
	Tag string
	// Err wraps one of the sentinel errors, except in warnings that report
	// a change Execute made (a table left without rows was removed, comments
	// kept on the first copy of repeated content), not a problem.
	Err error
}

func (i Issue) String() string {
	s := i.Severity.String() + ": " + i.Location.String()
	if i.Tag != "" {
		s += ": " + i.Tag
	}
	return s + ": " + i.Err.Error()
}

// Error is returned when execution stops. It lists every error found, not
// just the first, so a template or its data can be fixed in one pass.
type Error struct {
	Issues []Issue
}

func (e *Error) Error() string {
	switch len(e.Issues) {
	case 0:
		return "template: execution failed"
	case 1:
		return "template: " + e.Issues[0].String()
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "template: %d issues:", len(e.Issues))
	for _, i := range e.Issues {
		sb.WriteString("\n  ")
		sb.WriteString(i.String())
	}
	return sb.String()
}

// Is reports whether any issue wraps target.
func (e *Error) Is(target error) bool {
	for _, i := range e.Issues {
		if errors.Is(i.Err, target) {
			return true
		}
	}
	return false
}

// Placeholder is one tag found in a template.
type Placeholder struct {
	// Kind is "value", "each", "if", "unless", "end" or "image".
	Kind string
	// Path is the data path, e.g. "client.name" or ".severity"; empty for "end".
	Path string
	// Tag is the tag as written.
	Tag      string
	Location Location
}

// Execute fills the template doc from data, in the body, every header and
// every footer.
//
// Data is JSON-shaped: map[string]any whose values are strings, numbers,
// booleans, nil, nested map[string]any, or lists ([]any, []map[string]any,
// []string), which is what encoding/json produces.
//
// On success it returns any warnings. On failure it returns an *Error listing
// every problem, and doc is unchanged.
func Execute(doc *docx.Document, data map[string]any, opts Options) ([]Issue, error) {
	warnings, err := itemplate.Execute(bridge.Stories(doc), data, itemplate.Options{Mode: itemplate.Mode(opts.Mode)})
	var ie *itemplate.Error
	if errors.As(err, &ie) {
		return convertIssues(warnings), &Error{Issues: convertIssues(ie.Issues)}
	}
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	return convertIssues(warnings), nil
}

// Placeholders lists every tag in doc in document order, without filling
// anything. Use it to check data against a template before rendering. A
// non-nil error is an *Error listing the malformed tags; the well-formed
// ones are still returned.
func Placeholders(doc *docx.Document) ([]Placeholder, error) {
	ps, issues, err := itemplate.Placeholders(bridge.Stories(doc))
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	out := make([]Placeholder, len(ps))
	for i, p := range ps {
		out[i] = Placeholder{Kind: p.Kind, Path: p.Path, Tag: p.Tag, Location: Location(p.Location)}
	}
	if len(issues) > 0 {
		return out, &Error{Issues: convertIssues(issues)}
	}
	return out, nil
}

func convertIssues(in []itemplate.Issue) []Issue {
	if len(in) == 0 {
		return nil
	}
	out := make([]Issue, len(in))
	for i, is := range in {
		out[i] = Issue{Severity: Severity(is.Severity), Location: Location(is.Location), Tag: is.Tag, Err: is.Err}
	}
	return out
}
