package template

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// RichValue is formatted text for a value tag: runs with their own bold,
// italic, underline, colour and link. Each run starts from the formatting
// of the run holding the tag; a nil switch keeps that run's setting.
// docx/template maps its public Rich to this through Options.Adapt.
type RichValue struct {
	Runs []RichRun
}

// RichRun is one run of a RichValue. Text may hold \n and \t.
type RichRun struct {
	Text                    string
	Bold, Italic, Underline *bool
	// Color is a hex colour, RRGGBB; "" keeps the tag run's colour.
	Color string
	// Link, when set, makes the run an external hyperlink to this URL.
	Link string
}

// plain is the text of all runs, for places that only take text.
func (rv RichValue) plain() string {
	var sb strings.Builder
	for _, r := range rv.Runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// richOf reports whether v is a rich-text value.
func richOf(v any) (RichValue, bool) {
	switch x := v.(type) {
	case RichValue:
		return x, true
	case *RichValue:
		if x != nil {
			return *x, true
		}
	}
	return RichValue{Runs: nil}, false
}

// richInlines builds the runs (and hyperlinks) for a rich value placed in
// tagRun. ok is false after reporting a problem.
func (en *engine) richInlines(rv RichValue, tagRun *wml.CT_R, inLink bool, t tag, loc Location) ([]wml.InlineElement, bool) {
	var out []wml.InlineElement
	for i, rr := range rv.Runs {
		if rr.Color != "" && !isHexColor(rr.Color) {
			en.fail(loc, t, ErrUnsupportedValue, fmt.Sprintf("run %d: colour %q is not RRGGBB hex", i, rr.Color))
			return nil, false
		}
		if rr.Text == "" {
			continue
		}
		r, err := en.richRun(tagRun, rr)
		if err != nil {
			en.abort(err)
			return nil, false
		}
		if rr.Link == "" {
			out = append(out, r)
			continue
		}
		if inLink {
			en.fail(loc, t, ErrUnsupportedLocation, "a rich-text value with a link cannot sit inside a hyperlink")
			return nil, false
		}
		if en.opts.Hooks.AddLink == nil {
			en.fail(loc, t, ErrUnsupportedLocation, "links can only be added to a document")
			return nil, false
		}
		relID, styleID, err := en.opts.Hooks.AddLink(loc.Part, rr.Link)
		switch {
		case err == nil:
		case errors.Is(err, ErrUnsupportedValue):
			en.issues = append(en.issues, Issue{Severity: SeverityError, Location: loc, Tag: t.text, Err: err})
			return nil, false
		default:
			en.abort(fmt.Errorf("add link for %s: %w", t.text, err))
			return nil, false
		}
		// A link looks like a link: the Hyperlink style's colour and
		// underline, not the tag run's, unless the RichRun sets its own.
		// A run has one character style, so the Hyperlink style replaces
		// any the tag run had.
		if rr.Color == "" {
			r.RPr.SetColorVal("")
		}
		if rr.Underline == nil {
			r.RPr.SetUnderlineStyle("")
		}
		if old := r.RPr.StyleID(); styleID != "" && old != "" && old != styleID {
			en.warn(loc, t, fmt.Sprintf("the linked text uses the %q style in place of the tag's %q character style", styleID, old))
		}
		if styleID != "" {
			r.RPr.SetStyleID(styleID)
		}
		h := new(wml.CT_Hyperlink)
		h.RID = relID
		h.Children = []wml.InlineElement{r}
		out = append(out, h)
	}
	return out, true
}

// richRun builds one run: a copy of the tag run's properties with the
// RichRun's switches applied, holding its text.
func (en *engine) richRun(tagRun *wml.CT_R, rr RichRun) (*wml.CT_R, error) {
	r := new(wml.CT_R)
	if tagRun.RPr != nil {
		rpr, err := wml.Clone(tagRun.RPr)
		if err != nil {
			return nil, fmt.Errorf("copy run properties: %w", err)
		}
		r.RPr = rpr
		// The tag run's tracked formatting change belongs to it alone; its
		// w:id must stay unique.
		r.RPr.RemoveRevision()
	} else {
		r.RPr = new(wml.CT_RPr)
	}
	if rr.Bold != nil {
		r.RPr.SetBold(rr.Bold)
	}
	if rr.Italic != nil {
		r.RPr.SetItalic(rr.Italic)
	}
	if rr.Underline != nil {
		if *rr.Underline {
			r.RPr.SetUnderlineStyle("single")
		} else {
			r.RPr.SetUnderlineStyle("none")
		}
	}
	if rr.Color != "" {
		r.RPr.SetColorVal(strings.ToUpper(rr.Color))
	}
	var w textWriter
	w.value(rr.Text)
	r.Children = w.done()
	return r, nil
}

// abort records an internal failure that stops Execute.
func (en *engine) abort(err error) {
	if en.err == nil {
		en.err = err
	}
}

func isHexColor(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// fieldState follows the complex fields of a paragraph as its runs are
// filled, so a tag in the result of a HYPERLINK field counts as being in a
// link, as one inside a w:hyperlink does.
type fieldState struct {
	stack []*fieldFrame
}

type fieldFrame struct {
	instr  strings.Builder
	result bool // past the separate character: in the field's result
}

func (fs *fieldState) char(fldCharType string) {
	switch fldCharType {
	case wml.FldCharBegin:
		fs.stack = append(fs.stack, &fieldFrame{instr: strings.Builder{}, result: false})
	case wml.FldCharSeparate:
		if n := len(fs.stack); n > 0 {
			fs.stack[n-1].result = true
		}
	case wml.FldCharEnd:
		if n := len(fs.stack); n > 0 {
			fs.stack = fs.stack[:n-1]
		}
	}
}

func (fs *fieldState) instr(s string) {
	if n := len(fs.stack); n > 0 && !fs.stack[n-1].result {
		fs.stack[n-1].instr.WriteString(s)
	}
}

// inLink reports whether the current position is in a HYPERLINK field's
// result.
func (fs *fieldState) inLink() bool {
	for _, f := range fs.stack {
		if f.result && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(f.instr.String())), "HYPERLINK") {
			return true
		}
	}
	return false
}

// runSplitter collects the content of a run being filled. Until a rich
// value splits it, the run keeps its identity; after, its content is spread
// over copies of the run around the inserted runs.
type runSplitter struct {
	en     *engine
	orig   *wml.CT_R
	inLink bool        // the run is inside a w:hyperlink
	fields *fieldState // the paragraph's complex fields
	cur    []wml.RunContentElement
	out    []wml.InlineElement
	split  bool
	used   bool // orig already holds a piece
}

func (sp *runSplitter) add(rc ...wml.RunContentElement) { sp.cur = append(sp.cur, rc...) }

// insert ends the current piece and places els after it.
func (sp *runSplitter) insert(els []wml.InlineElement) {
	sp.flush()
	sp.out = append(sp.out, els...)
	sp.split = true
}

// flush closes the current piece as a run: the original run for the first
// piece, a copy of its properties and attributes for later ones.
func (sp *runSplitter) flush() {
	if len(sp.cur) == 0 {
		return
	}
	r := sp.orig
	if sp.used {
		c, err := cloneRunShell(sp.orig)
		if err != nil {
			sp.en.abort(err)
			return
		}
		r = c
	}
	sp.used = true
	r.Children = sp.cur
	sp.out = append(sp.out, r)
	sp.cur = nil
}

// done returns what replaces the run.
func (sp *runSplitter) done() []wml.InlineElement {
	if !sp.split {
		sp.orig.Children = sp.cur
		return []wml.InlineElement{sp.orig}
	}
	sp.flush()
	return sp.out
}

// cloneRunShell copies a run's properties and attributes without its
// content.
func cloneRunShell(r *wml.CT_R) (*wml.CT_R, error) {
	c := &wml.CT_R{RPr: nil, Children: nil, Attrs: append([]xml.Attr(nil), r.Attrs...)}
	if r.RPr != nil {
		rpr, err := wml.Clone(r.RPr)
		if err != nil {
			return nil, fmt.Errorf("copy run properties: %w", err)
		}
		// The original run keeps the tracked change; a copy repeating its
		// w:id would make the ids non-unique.
		rpr.RemoveRevision()
		c.RPr = rpr
	}
	return c, nil
}
