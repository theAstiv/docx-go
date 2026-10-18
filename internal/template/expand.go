package template

import (
	"errors"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// engine carries the state of one Execute call.
type engine struct {
	opts   Options
	issues []Issue
	// nextDocPr hands out wp:docPr ids for drawings in {{#each}} copies.
	nextDocPr func() int64
	// err is an internal failure (an element that could not be copied);
	// it aborts Execute.
	err error
	// skip holds text that is not template text (tables of contents).
	skip skipSet
	// inRaw is set while filling raw content (content controls, text
	// boxes), where block tags are not supported.
	inRaw bool
}

// missing reports a value tag whose key is absent: an error in Strict mode,
// a warning (and empty text) in Lenient mode.
func (en *engine) missing(loc Location, t tag, detail string) {
	is := issueErr(loc, t.text, ErrMissingKey, detail)
	if en.opts.Mode == Lenient {
		is.Severity = SeverityWarning
	}
	en.issues = append(en.issues, is)
}

func (en *engine) fail(loc Location, t tag, sentinel error, detail string) {
	en.issues = append(en.issues, issueErr(loc, t.text, sentinel, detail))
}

// warn records a warning about a change Execute made rather than a problem
// (a table left without rows was removed, say), so it wraps no sentinel.
func (en *engine) warn(loc Location, t tag, msg string) {
	en.issues = append(en.issues, Issue{Severity: SeverityWarning, Location: loc, Tag: t.text, Err: errors.New(msg)})
}

// fillParagraph replaces the value tags of a normalized paragraph, where
// every well-formed tag sits inside one w:t.
func (en *engine) fillParagraph(p *wml.CT_P, sc *scope, loc Location) {
	en.fillInlines(p.Children, sc, loc)
}

func (en *engine) fillInlines(children []wml.InlineElement, sc *scope, loc Location) {
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			en.fillRun(el, sc, loc)
		case *wml.CT_Hyperlink:
			en.fillInlines(el.Children, sc, loc)
		case *wml.RawInline:
			en.fillRaw(el.E, sc, loc)
		}
	}
}

func (en *engine) fillRun(r *wml.CT_R, sc *scope, loc Location) {
	var out []wml.RunContentElement
	changed := false
	for _, rc := range r.Children {
		switch el := rc.(type) {
		case *wml.RawRunContent:
			en.fillRaw(el.E, sc, loc)
		case *wml.CT_Drawing:
			en.fillDrawing(el, sc, loc)
		}
		txt, ok := rc.(*wml.CT_Text)
		if !ok || en.skip[txt] || !strings.Contains(txt.Content, "{{") {
			out = append(out, rc)
			continue
		}
		// Malformed tags were reported by normalize; they stay as text.
		tags, _ := lexTags(txt.Content)
		if len(tags) == 0 {
			out = append(out, rc)
			continue
		}
		changed = true
		out = append(out, en.fillText(txt, tags, sc, loc)...)
	}
	if changed {
		r.Children = out
	}
}

// fillText renders one w:t with its tags replaced. A value containing line
// breaks or tabs becomes w:t / w:br / w:tab elements.
func (en *engine) fillText(txt *wml.CT_Text, tags []tag, sc *scope, loc Location) []wml.RunContentElement {
	var w textWriter
	w.space = txt.Space
	pos := 0
	for _, t := range tags {
		w.text(txt.Content[pos:t.start])
		pos = t.end
		switch t.kind {
		case tagValue:
			w.value(en.value(t, sc, loc))
		case tagImage:
			en.fail(loc, t, ErrUnsupportedLocation, "image placeholders are not supported yet")
			w.text(t.text)
		case tagOpen, tagClose:
			if en.inRaw {
				en.fail(loc, t, ErrUnsupportedLocation, "block tags are not supported "+rawLocation)
			} else {
				en.fail(loc, t, ErrMisplacedBlock, misplacedDetail)
			}
			w.text(t.text)
		}
	}
	w.text(txt.Content[pos:])
	return w.done()
}

// value resolves a value tag to its text.
func (en *engine) value(t tag, sc *scope, loc Location) string {
	r := sc.lookup(t.path)
	if !r.found {
		en.missing(loc, t, r.detail)
		return ""
	}
	s, ok := format(r.value)
	if !ok {
		en.fail(loc, t, ErrUnsupportedValue, "a list or object cannot be written as text; use {{#each}} or a dotted path")
		return ""
	}
	return s
}

// textWriter builds run content from text, turning line breaks and tabs in
// values into w:br and w:tab.
type textWriter struct {
	space string
	sb    strings.Builder
	out   []wml.RunContentElement
}

func (w *textWriter) text(s string) { w.sb.WriteString(s) }

func (w *textWriter) value(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	for _, c := range s {
		switch c {
		case '\n':
			w.flush()
			w.out = append(w.out, &wml.CT_Br{Type: "", Clear: ""})
		case '\t':
			w.flush()
			w.out = append(w.out, &wml.CT_Tab{})
		default:
			w.sb.WriteRune(c)
		}
	}
}

func (w *textWriter) flush() {
	if w.sb.Len() == 0 {
		return
	}
	t := &wml.CT_Text{Space: w.space, Content: w.sb.String()}
	preserveSpace(t)
	w.out = append(w.out, t)
	w.sb.Reset()
}

func (w *textWriter) done() []wml.RunContentElement {
	w.flush()
	return w.out
}
