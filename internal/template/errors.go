package template

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors. Every Issue wraps one of them, so callers can test with
// errors.Is on an Issue's Err or on the *Error that Execute returns.
var (
	// ErrMissingKey: a value tag names a key the data does not contain.
	ErrMissingKey = errors.New("missing key")
	// ErrMalformedTag: {{ without a closing }}, an empty or invalid path, or
	// a tab or line break inside a tag.
	ErrMalformedTag = errors.New("malformed tag")
	// ErrUnbalancedBlock: an opening block tag without its closing tag, or
	// the reverse, or mismatched kinds ({{#if}} closed by {{/each}}).
	ErrUnbalancedBlock = errors.New("unbalanced block")
	// ErrMisplacedBlock: a block tag that is neither alone in its paragraph
	// nor at a table-row edge.
	ErrMisplacedBlock = errors.New("misplaced block tag")
	// ErrSplitAcrossBoundary: a tag split across a field, bookmark,
	// hyperlink edge, content control or tracked change.
	ErrSplitAcrossBoundary = errors.New("tag split across a boundary")
	// ErrUnsupportedValue: a value of a type that cannot be written as text,
	// or a list or map where a single value is expected.
	ErrUnsupportedValue = errors.New("unsupported value")
	// ErrSectionBreakInBlock: an {{#if}} or {{#each}} range contains a
	// section break, which removing or repeating would corrupt.
	ErrSectionBreakInBlock = errors.New("section break inside a block")
	// ErrUnsupportedLocation: a tag where the engine cannot act on it, such
	// as a block tag inside a content control, or a tag in a footnote.
	ErrUnsupportedLocation = errors.New("tag in an unsupported location")
)

// Severity says whether an Issue stops execution.
type Severity int

const (
	// SeverityError stops execution; the document is left unchanged.
	SeverityError Severity = iota
	// SeverityWarning is reported but does not stop execution.
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Location identifies where in the package a tag sits.
type Location struct {
	// Part is the OPC path of the part, e.g. "word/document.xml".
	Part string
	// Path is the element path inside the part, e.g. "body/tbl[0]/tr[2]/tc[1]/p[0]".
	Path string
}

func (l Location) String() string {
	if l.Path == "" {
		return l.Part
	}
	return l.Part + " › " + l.Path
}

// child returns the location of the index-th child of the given kind.
func (l Location) child(kind string, index int) Location {
	p := kind + "[" + fmt.Sprint(index) + "]"
	if l.Path != "" {
		p = l.Path + "/" + p
	}
	return Location{Part: l.Part, Path: p}
}

// Issue is one problem found in a template or its data.
type Issue struct {
	Severity Severity
	Location Location
	// Tag is the tag text as written, e.g. "{{client.name}}"; empty when the
	// issue is not about one tag.
	Tag string
	// Err wraps one of the sentinel errors.
	Err error
}

func (i Issue) String() string {
	var sb strings.Builder
	sb.WriteString(i.Severity.String())
	sb.WriteString(": ")
	sb.WriteString(i.Location.String())
	if i.Tag != "" {
		sb.WriteString(": ")
		sb.WriteString(i.Tag)
	}
	sb.WriteString(": ")
	sb.WriteString(i.Err.Error())
	return sb.String()
}

// Error is returned when execution stops. It lists every error-severity
// issue found, not just the first.
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

// Is reports whether any issue wraps target, so errors.Is(err, ErrMissingKey)
// works on the error Execute returns.
func (e *Error) Is(target error) bool {
	for _, i := range e.Issues {
		if errors.Is(i.Err, target) {
			return true
		}
	}
	return false
}

// issueErr builds an error-severity issue whose Err wraps sentinel with detail.
func issueErr(loc Location, tag string, sentinel error, detail string) Issue {
	err := sentinel
	if detail != "" {
		err = fmt.Errorf("%w: %s", sentinel, detail)
	}
	return Issue{Severity: SeverityError, Location: loc, Tag: tag, Err: err}
}
