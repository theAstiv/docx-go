package template

import (
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Mode decides what a value tag with a missing key does.
type Mode int

const (
	// Strict fails execution and lists every missing key.
	Strict Mode = iota
	// Lenient writes empty text for a missing key and reports a warning.
	Lenient
)

// Options configures Execute.
type Options struct {
	Mode Mode
}

// Story is one part's block content: the document body, a header or a
// footer.
type Story struct {
	// Part is the part's OPC path, used in issue locations.
	Part string
	// Blocks is the part's block list. Execute replaces it on success.
	Blocks *[]wml.BlockElement
}

// Execute fills the stories from data.
//
// It works on copies and writes them back only when no error-severity issue
// was found, so on failure every story is exactly as it was. The returned
// issues are warnings; errors come back together in an *Error. A non-nil
// error that is not an *Error means the element tree could not be copied.
func Execute(stories []Story, data map[string]any, opts Options) ([]Issue, error) {
	if data == nil {
		data = map[string]any{}
	}
	en := &engine{opts: opts}
	work := make([][]wml.BlockElement, len(stories))
	for i, s := range stories {
		blocks, err := cloneBlocks(*s.Blocks)
		if err != nil {
			return nil, fmt.Errorf("template: copy %s: %w", s.Part, err)
		}
		en.issues = append(en.issues, normalizeBlocks(blocks, Location{Part: s.Part})...)
		work[i] = blocks
	}
	// Drawings copied by {{#each}} need wp:docPr ids unique across every
	// story; scan once, allocate many. Known gap: footnotes and endnotes are
	// unparsed, so a drawing there is not seen and a copy's new id could, in
	// principle, equal its id.
	scan := new(wml.IDScan)
	for _, blocks := range work {
		wml.ScanBlocks(blocks, scan)
	}
	en.nextDocPr = scan.DocPrAllocator()

	root := &scope{root: data}
	for i, s := range stories {
		work[i] = en.expandBlocks(work[i], root, Location{Part: s.Part})
	}
	if en.err != nil {
		return nil, fmt.Errorf("template: %w", en.err)
	}

	warnings, errs := split(en.issues)
	if len(errs) > 0 {
		return warnings, &Error{Issues: errs}
	}
	for i, s := range stories {
		*s.Blocks = work[i]
	}
	return warnings, nil
}

func cloneBlocks(blocks []wml.BlockElement) ([]wml.BlockElement, error) {
	out := make([]wml.BlockElement, len(blocks))
	for i, b := range blocks {
		c, err := wml.Clone(b)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// split separates warnings from errors, keeping their order.
func split(issues []Issue) (warnings, errs []Issue) {
	for _, is := range issues {
		if is.Severity == SeverityWarning {
			warnings = append(warnings, is)
		} else {
			errs = append(errs, is)
		}
	}
	return warnings, errs
}
