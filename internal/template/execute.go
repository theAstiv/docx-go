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
	// Funcs adds formatters, or replaces built-in ones other than "default".
	Funcs map[string]Formatter
	// Adapt, when set, maps every looked-up value before it is used, so a
	// caller's own value types (an image, rich text) reach the engine as
	// ImageValue and the like.
	Adapt func(any) any
	// Hooks do what needs the package, such as storing images.
	Hooks Hooks
}

// Story is one part's block content: the document body, a header or a
// footer.
type Story struct {
	// Part is the part's OPC path, used in issue locations.
	Part string
	// Blocks is the part's block list. Execute replaces it on success.
	Blocks *[]wml.BlockElement
	// FinalParagraph is set for parts that must end with a w:p (headers
	// and footers): if ranges remove their last paragraph, an empty one is
	// added.
	FinalParagraph bool
}

// Execute fills the stories from data.
//
// It works on copies and writes them back only when no error-severity issue
// was found, so on failure every story is exactly as it was. The returned
// issues are warnings; errors come back together in an *Error. A non-nil
// error that is not an *Error means the element tree could not be copied.
func Execute(stories []Story, data map[string]any, opts Options) ([]Issue, error) {
	_, warnings, err := ExecuteAll(stories, nil, data, opts)
	return warnings, err
}

// ExecuteAll is Execute that also fills the parts data-bound content
// controls read from (see xmlparts.go). It returns the parts that changed;
// the caller writes them back, and only on success, so the run stays
// atomic.
func ExecuteAll(stories []Story, boundParts map[string][]byte, data map[string]any, opts Options) (map[string][]byte, []Issue, error) {
	if data == nil {
		data = map[string]any{}
	}
	if _, ok := opts.Funcs[defaultName]; ok {
		return nil, nil, errDefaultOverride
	}
	en := &engine{opts: opts, skip: skipSet{}, fmts: formatterSet{funcs: opts.Funcs}}
	work := make([][]wml.BlockElement, len(stories))
	for i, s := range stories {
		blocks, err := cloneBlocks(*s.Blocks)
		if err != nil {
			return nil, nil, fmt.Errorf("template: copy %s: %w", s.Part, err)
		}
		skip, issues := normalizeStory(blocks, Location{Part: s.Part})
		for t := range skip {
			en.skip[t] = true
		}
		en.issues = append(en.issues, issues...)
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

	structural, err := checkStructure(stories, work, opts)
	if err != nil {
		return nil, nil, err
	}
	before := make([]storyAnchors, len(work))
	for i, blocks := range work {
		before[i] = anchorsOf(blocks)
	}
	root := &scope{root: data}
	for i, s := range stories {
		loc := Location{Part: s.Part}
		work[i] = en.expandBlocks(work[i], root, loc)
		en.cleanStoryAnchors(&work[i], before[i], loc)
		if s.FinalParagraph && !endsWithParagraph(work[i]) {
			work[i] = append(work[i], emptyParagraph())
		}
	}
	en.warnRemovedReferences(work)
	filled := en.fillXMLParts(boundParts, root)
	if en.err != nil {
		return nil, nil, fmt.Errorf("template: %w", en.err)
	}

	warnings, errs := split(dedupe(append(en.issues, structural...)))
	if len(errs) > 0 {
		return nil, warnings, &Error{Issues: errs}
	}
	for i, s := range stories {
		*s.Blocks = work[i]
	}
	return filled, warnings, nil
}

// checkStructure reports the template errors that do not depend on the
// data: unbalanced and misplaced block tags, section breaks in blocks,
// malformed tags in raw content. Execution only looks at the branches the
// data selects, so without this a broken branch (most often the side of an
// {{else}} the test data never takes) would pass and fail later in
// production. It runs a data-free engine over a copy of the normalized
// stories that visits every branch once and looks up no values, and returns
// its error-severity issues; its warnings depend on that visit, not on the
// document, and are dropped.
func checkStructure(stories []Story, work [][]wml.BlockElement, opts Options) ([]Issue, error) {
	ck := &engine{opts: opts, skip: skipSet{}, check: true, nextDocPr: func() int64 { return 0 },
		fmts: formatterSet{funcs: opts.Funcs}}
	for i, s := range stories {
		blocks, err := cloneBlocks(work[i])
		if err != nil {
			return nil, fmt.Errorf("template: copy %s: %w", s.Part, err)
		}
		// The skip set holds text elements by identity, so the copy needs
		// its own.
		for t := range tocSkips(blocks) {
			ck.skip[t] = true
		}
		ck.expandBlocks(blocks, &scope{root: map[string]any{}}, Location{Part: s.Part})
	}
	if ck.err != nil {
		return nil, fmt.Errorf("template: %w", ck.err)
	}
	_, errs := split(ck.issues)
	return errs, nil
}

// dedupe drops repeated issues, keeping the first. The structure check
// repeats what execution finds in the branches it renders, and execution
// repeats a template error once per {{#each}} copy.
func dedupe(issues []Issue) []Issue {
	type key struct {
		sev           Severity
		loc, tag, msg string
	}
	seen := make(map[key]bool, len(issues))
	out := issues[:0:0]
	for _, is := range issues {
		k := key{is.Severity, is.Location.String(), is.Tag, is.Err.Error()}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, is)
	}
	return out
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
