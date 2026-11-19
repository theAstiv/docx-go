package template

import (
	"errors"
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/internal/bridge"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
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
	// ErrUnknownFormatter: a tag pipes its value through a formatter that is
	// neither built in nor in Options.Funcs. It fails in both modes.
	ErrUnknownFormatter = itemplate.ErrUnknownFormatter
	// ErrFormat: a formatter rejected its value, such as a date that does
	// not parse. An error in Strict mode; a warning (and empty text) in
	// Lenient mode.
	ErrFormat = itemplate.ErrFormat
)

// Image is a data value for an {{image path}} tag: the picture is placed
// inline where the tag is, in the tag's run. Data is PNG, JPEG or GIF.
// Width and Height follow docx.ImageOptions: both zero shows the image at
// its pixel size at 96 DPI, and one zero follows from the aspect ratio.
// AltText is required; it is what screen readers read out.
//
// An Image (or *Image) can sit anywhere in the data, including in a list
// that {{#each}} repeats. Use it only with {{image}}: a value tag holding an
// image is ErrUnsupportedValue.
type Image struct {
	Data          []byte
	Width, Height docx.EMU
	AltText       string
	// Title is an optional tooltip-style title.
	Title string
}

// adapt maps the public value types to the engine's.
func adapt(v any) any {
	switch x := v.(type) {
	case Image:
		return imageValue(x)
	case *Image:
		if x == nil {
			return nil
		}
		return imageValue(*x)
	}
	return v
}

func imageValue(i Image) itemplate.ImageValue {
	return itemplate.ImageValue{Data: i.Data, Width: int64(i.Width), Height: int64(i.Height), AltText: i.AltText, Title: i.Title}
}

// Formatter transforms a value in a tag's pipe. {{total | money "EUR"}}
// calls the formatter registered as "money" with total's value and the
// arguments ["EUR"]. Arguments are strings or float64s. The result is
// written as text (as a value would be), or passed to the next formatter.
// A returned error is reported as ErrFormat.
type Formatter func(v any, args []any) (any, error)

// errTOCNeedsUpdate explains the warning for a table of contents that still
// shows template tags.
var errTOCNeedsUpdate = errors.New("the table of contents shows the headings' tags until Word updates fields; " +
	"call doc.SetUpdateFieldsOnOpen(true) so Word rebuilds it when the document opens")

// errNilDocument is returned for a nil *docx.Document: without it a nil
// document has no stories and would "succeed" with nothing filled.
var errNilDocument = errors.New("template: nil document")

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
	// Funcs adds formatters by name. An entry with the name of a built-in
	// (upper, lower, number, currency, date) replaces it, so number can be
	// given local digit grouping. "default" cannot be replaced.
	Funcs map[string]Formatter
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
	// Kind is "value", "each", "if", "unless", "else", "end" or "image".
	Kind string
	// Path is the data path, e.g. "client.name" or ".severity"; empty for
	// "else" and "end".
	Path string
	// Tag is the tag as written.
	Tag string
	// Formatters are the names of the formatters a value tag pipes through,
	// in order: ["number", "default"] for {{total | number 2 | default "—"}}.
	Formatters []string
	Location   Location
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
	if doc == nil {
		return nil, errNilDocument
	}
	var funcs map[string]itemplate.Formatter
	if len(opts.Funcs) > 0 {
		funcs = make(map[string]itemplate.Formatter, len(opts.Funcs))
		for name, f := range opts.Funcs {
			funcs[name] = itemplate.Formatter(f)
		}
	}
	// Adding an image changes the package itself (a media part, a
	// relationship, a content type), not just the story copies the engine
	// fills. Record the package before the first such change and put it back
	// if execution fails, so a failed Execute still leaves doc unchanged.
	var restore func()
	var snapErr error
	addImage := func(part string, img itemplate.ImageValue, docPrID int64) (*wml.CT_Drawing, error) {
		if restore == nil {
			if restore, snapErr = bridge.Snapshot(doc); snapErr != nil {
				return nil, fmt.Errorf("snapshot: %w", snapErr)
			}
		}
		return bridge.AddImage(doc, part, img, docPrID)
	}
	filled, warnings, err := itemplate.ExecuteAll(bridge.Stories(doc), bridge.BoundParts(doc), data,
		itemplate.Options{
			Mode:  itemplate.Mode(opts.Mode),
			Funcs: funcs,
			Adapt: adapt,
			Hooks: itemplate.Hooks{AddImage: addImage},
		})
	if err != nil && restore != nil {
		restore()
	}
	if snapErr != nil {
		return nil, fmt.Errorf("template: %w", snapErr)
	}
	warnings = append(warnings, itemplate.ScanUnparsed(bridge.UnparsedParts(doc))...)
	var ie *itemplate.Error
	if errors.As(err, &ie) {
		return convertIssues(warnings), &Error{Issues: convertIssues(ie.Issues)}
	}
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	// Document properties and custom XML that data-bound content controls
	// read from: written only now, so a failed run changes nothing.
	for path, data := range filled {
		bridge.SetPart(doc, path, data)
	}
	out := convertIssues(warnings)
	// A table of contents repeats heading text, tags included; Execute
	// leaves it to Word to rebuild, which happens only when fields update.
	if !doc.UpdateFieldsOnOpen() {
		for _, part := range itemplate.TOCTagParts(bridge.Stories(doc)) {
			out = append(out, Issue{
				Severity: SeverityWarning,
				Location: Location{Part: part, Path: ""},
				Tag:      "",
				Err:      errTOCNeedsUpdate,
			})
		}
	}
	return out, nil
}

// Placeholders lists every tag in doc in document order, without filling
// anything. Use it to check data against a template before rendering. A
// non-nil error is an *Error listing the malformed tags; the well-formed
// ones are still returned.
func Placeholders(doc *docx.Document) ([]Placeholder, error) {
	if doc == nil {
		return nil, errNilDocument
	}
	ps, issues, err := itemplate.PlaceholdersAll(bridge.Stories(doc), bridge.BoundParts(doc))
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}
	out := make([]Placeholder, len(ps))
	for i, p := range ps {
		out[i] = Placeholder{Kind: p.Kind, Path: p.Path, Tag: p.Tag, Formatters: p.Formatters, Location: Location(p.Location)}
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
