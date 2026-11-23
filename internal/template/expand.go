package template

import (
	"errors"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/textmerge"
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
	// check makes the engine a data-free structure check (checkStructure):
	// every branch is visited once and no value is looked up.
	check bool
	// fmts resolves formatter names (Options.Funcs, then built-ins).
	fmts formatterSet
	// removedBookmarks are the bookmarks inline ranges removed, checked
	// against cross-references once every story is filled.
	removedBookmarks []removedBookmark
	// copiesLeft is how many more elements {{#each}} may copy; -1 for no
	// limit. Once spent, loops make no more copies (execution fails).
	copiesLeft int
	overLimit  bool
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

// fillParagraph expands the inline ranges of a normalized paragraph, where
// every well-formed tag sits inside one w:t, and replaces its value tags.
func (en *engine) fillParagraph(p *wml.CT_P, sc *scope, loc Location) {
	if !en.inRaw {
		p.Children = en.expandInline(p.Children, sc, loc)
	}
	p.Children = en.fillInlines(p.Children, sc, loc, false, &fieldState{stack: nil})
}

// fillInlines fills an inline sequence and returns it: a run holding a
// rich-text value becomes several runs and hyperlinks.
func (en *engine) fillInlines(children []wml.InlineElement, sc *scope, loc Location, inLink bool, fs *fieldState) []wml.InlineElement {
	out := children[:0:0]
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			out = append(out, en.fillRun(el, sc, loc, inLink, fs)...)
			continue
		case *wml.CT_Hyperlink:
			el.Children = en.fillInlines(el.Children, sc, loc, true, fs)
		case *wml.RawInline:
			en.fillRaw(el.E, sc, loc)
		}
		out = append(out, c)
	}
	return out
}

func (en *engine) fillRun(r *wml.CT_R, sc *scope, loc Location, inLink bool, fs *fieldState) []wml.InlineElement {
	sp := &runSplitter{en: en, orig: r, inLink: inLink, fields: fs}
	changed := false
	for _, rc := range r.Children {
		switch el := rc.(type) {
		case *wml.RawRunContent:
			en.fillRaw(el.E, sc, loc)
		case *wml.CT_Drawing:
			en.fillDrawing(el, sc, loc)
		case *wml.CT_FldChar:
			fs.char(el.FldCharType)
		case *wml.CT_InstrText:
			fs.instr(el.Content)
		}
		txt, ok := rc.(*wml.CT_Text)
		if !ok || en.skip[txt] || !strings.Contains(txt.Content, "{{") {
			sp.add(rc)
			continue
		}
		// Malformed tags were reported by normalize; they stay as text.
		tags, _ := lexTags(txt.Content)
		if len(tags) == 0 {
			sp.add(rc)
			continue
		}
		changed = true
		en.fillText(txt, tags, sc, loc, sp)
	}
	if !changed {
		return []wml.InlineElement{r}
	}
	return sp.done()
}

// fillText renders one w:t with its tags replaced into sp. A value
// containing line breaks or tabs becomes w:t / w:br / w:tab elements; a
// rich-text value splits the run.
func (en *engine) fillText(txt *wml.CT_Text, tags []tag, sc *scope, loc Location, sp *runSplitter) {
	var w textWriter
	w.space = txt.Space
	pos := 0
	for _, t := range tags {
		w.text(txt.Content[pos:t.start])
		pos = t.end
		switch t.kind {
		case tagValue:
			v, ok := en.resolve(t, sc, loc)
			if !ok {
				continue
			}
			if rv, isRich := richOf(v); isRich && !en.inRaw {
				if rv.plain() == "" {
					continue // nothing to insert: leave the run whole
				}
				if els, ok := en.richInlines(rv, sp.orig, sp.inLink || sp.fields.inLink(), t, loc); ok {
					sp.add(w.take()...)
					sp.insert(els)
				}
				continue
			}
			w.value(en.text(v, t, loc))
		case tagImage:
			if d, ok := en.image(t, sc, loc); ok {
				w.flush()
				w.out = append(w.out, d)
			} else if en.inRaw {
				w.text(t.text)
			}
		case tagOpen, tagClose, tagElse:
			// Outside raw content, expandInline has already reported the
			// block tags that are left: they are not part of a range.
			if en.inRaw {
				en.fail(loc, t, ErrUnsupportedLocation, "block tags are not supported "+rawLocation)
			}
			w.text(t.text)
		}
	}
	w.text(txt.Content[pos:])
	sp.add(w.done()...)
}

// value resolves a value tag to its text: lookup, then the tag's
// formatters in order, then the text form. Where only text can go (content
// controls, document properties) a rich-text value is written as its plain
// text, with a warning.
func (en *engine) value(t tag, sc *scope, loc Location) string {
	v, ok := en.resolve(t, sc, loc)
	if !ok {
		return ""
	}
	return en.text(v, t, loc)
}

// text renders a resolved value as text.
func (en *engine) text(v any, t tag, loc Location) string {
	if rv, isRich := richOf(v); isRich {
		en.warn(loc, t, "rich-text formatting and links are dropped here; only the text is written")
		return rv.plain()
	}
	s, ok := format(v)
	if !ok {
		switch v.(type) {
		case ImageValue, *ImageValue:
			en.fail(loc, t, ErrUnsupportedValue, "an image is placed with {{image "+t.path+"}}, not a value tag")
		default:
			en.fail(loc, t, ErrUnsupportedValue, "a list or object cannot be written as text; use {{#each}} or a dotted path")
		}
		return ""
	}
	return s
}

// resolve looks up a value tag and runs its formatters. ok is false when
// nothing is to be written: a problem was reported, the key is missing (in
// Lenient mode too), or this is the structure check.
func (en *engine) resolve(t tag, sc *scope, loc Location) (any, bool) {
	// Formatter names and built-in arguments are template errors: checked
	// on every tag, rendered or not (the structure check visits them all).
	fns := make([]Formatter, len(t.calls))
	bad := false
	for i, c := range t.calls {
		f, check, ok := en.fmts.lookup(c.name)
		if !ok {
			en.fail(loc, t, ErrUnknownFormatter, quote(c.name)+" (built in: currency, date, default, lower, number, upper)")
			bad = true
			continue
		}
		if check != nil {
			if detail := check(c.args); detail != "" {
				en.fail(loc, t, ErrMalformedTag, detail)
				bad = true
				continue
			}
		}
		fns[i] = f
	}
	if bad || en.check {
		return nil, false
	}
	r := sc.lookup(t.path)
	v, found := en.adapt(r.value), r.found
	for i, c := range t.calls {
		if !found {
			// Only default turns a missing value into a present one; the
			// other formatters never see it.
			if c.name == defaultName {
				v, found = c.args[0], true
			}
			continue
		}
		out, err := callFormatter(fns[i], v, c.args)
		if err != nil {
			en.formatFailed(loc, t, c.name+": "+err.Error())
			return nil, false
		}
		v = out
	}
	if !found {
		en.missing(loc, t, r.detail)
		return nil, false
	}
	return v, true
}

// formatFailed reports a formatter that rejected its value: an error in
// Strict mode, a warning (and empty text) in Lenient mode.
func (en *engine) formatFailed(loc Location, t tag, detail string) {
	is := issueErr(loc, t.text, ErrFormat, detail)
	if en.opts.Mode == Lenient {
		is.Severity = SeverityWarning
	}
	en.issues = append(en.issues, is)
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
	textmerge.PreserveSpace(t)
	w.out = append(w.out, t)
	w.sb.Reset()
}

func (w *textWriter) done() []wml.RunContentElement {
	w.flush()
	return w.out
}

// take returns the content written so far and starts afresh, for a run
// that is split at this point.
func (w *textWriter) take() []wml.RunContentElement {
	out := w.done()
	w.out = nil
	return out
}
