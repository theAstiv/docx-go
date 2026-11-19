package docx

import (
	"strings"
	"unicode/utf8"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/textmerge"
)

// ReplaceOptions controls Document.Replace and Paragraph.Replace. The zero
// value replaces every case-sensitive match in the body, headers and
// footers.
type ReplaceOptions struct {
	// IgnoreCase matches without regard to case (Unicode simple folding).
	IgnoreCase bool
	// SkipHeaders and SkipFooters leave headers and footers alone
	// (Document.Replace only).
	SkipHeaders, SkipFooters bool
	// Limit stops after this many replacements; 0 means no limit.
	Limit int
}

// ReplaceResult reports what a Replace call did.
type ReplaceResult struct {
	// Count is the number of matches replaced.
	Count int
	// Skipped lists paragraphs whose visible text has matches that were
	// not replaced (see Document.Replace).
	Skipped []ReplaceSkip
}

// ReplaceSkip describes matches that could not be replaced.
type ReplaceSkip struct {
	// Part is the package part holding the text ("word/document.xml",
	// "word/header1.xml", …).
	Part string
	// Text is the visible text, before replacing, of the paragraph (or
	// content control) the matches are in.
	Text string
	// Count is the number of matches not replaced there.
	Count int
}

// Replace replaces old with new throughout the document: body paragraphs
// and tables, then headers and footers (unless opts skips them).
//
// Text that Word has split into several runs (spell-check marks,
// formatting or revision changes) is found; the replacement takes the
// formatting of the run holding the match's first character. "\n" and "\t"
// in new become a line break and a tab. old must not contain tabs or line
// breaks: such a call replaces nothing.
//
// A match is not replaced when it is split by something other than plain
// runs (a field, a bookmark, a hyperlink edge), when it lies in content the
// library does not model (content controls, tracked insertions, smart
// tags), or when it lies in a table of contents, which Word rebuilds when
// fields update. Those matches are reported in ReplaceResult.Skipped.
// Field results are replaced; field instructions are not searched.
func (d *Document) Replace(old, new string, opts ReplaceOptions) ReplaceResult {
	var r replacer
	if !r.init(old, new, opts) {
		return ReplaceResult{}
	}
	if d.pkg.DocPart != nil {
		if body := d.pkg.DocPart.Body(); body != nil {
			r.story(body.Children, docStory(d.pkg).src)
		}
	}
	if !opts.SkipHeaders {
		for _, h := range d.pkg.UniqueHeaders() {
			r.story(h.Part.CT().Children, h.Path())
		}
	}
	if !opts.SkipFooters {
		for _, f := range d.pkg.UniqueFooters() {
			r.story(f.Part.CT().Children, f.Path())
		}
	}
	return r.res
}

// Replace replaces old with new in this paragraph, including its
// hyperlinks, with the rules of Document.Replace. SkipHeaders and
// SkipFooters are ignored.
func (p *Paragraph) Replace(old, new string, opts ReplaceOptions) ReplaceResult {
	var r replacer
	if !r.init(old, new, opts) {
		return ReplaceResult{}
	}
	part := ""
	if p.st != nil {
		part = p.st.src
	}
	// A table of contents spans paragraphs, so its text is found by
	// scanning the paragraph's whole container.
	var skip textmerge.SkipSet
	if list, _, ok := p.st.locateBlock(p.p, p.parent); ok {
		skip = textmerge.TOCSkips(*list)
	}
	r.paragraph(p.p, part, skip)
	return r.res
}

type replacer struct {
	old, new string
	fold     bool
	limit    int
	res      ReplaceResult
}

func (r *replacer) init(old, new string, opts ReplaceOptions) bool {
	if old == "" || strings.ContainsAny(old, "\t\n\r\x00\x01") {
		return false
	}
	r.old = old
	r.new = strings.ReplaceAll(strings.ReplaceAll(new, "\r\n", "\n"), "\r", "\n")
	r.fold = opts.IgnoreCase
	r.limit = opts.Limit
	return true
}

func (r *replacer) done() bool { return r.limit > 0 && r.res.Count >= r.limit }

func (r *replacer) story(blocks []wml.BlockElement, part string) {
	r.blocks(blocks, part, textmerge.TOCSkips(blocks))
}

func (r *replacer) blocks(blocks []wml.BlockElement, part string, skip textmerge.SkipSet) {
	for _, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			r.paragraph(el, part, skip)
		case *wml.CT_Tbl:
			for _, row := range el.Children {
				tr, ok := row.(*wml.CT_Tr)
				if !ok {
					r.skipRaw(row, part)
					continue
				}
				for _, c := range tr.Children {
					if tc, ok := c.(*wml.CT_Tc); ok {
						r.blocks(tc.Children, part, skip)
					} else {
						r.skipRaw(c, part)
					}
				}
			}
		case *wml.RawBlock:
			r.skipRaw(el, part)
		}
	}
}

// skipRaw reports the matches inside content the library keeps raw.
func (r *replacer) skipRaw(node any, part string) {
	var text string
	switch el := node.(type) {
	case *wml.RawBlock:
		text = wml.BlocksText([]wml.BlockElement{el})
	case *wml.RawTableRow, *wml.RawTableCell:
		tbl := new(wml.CT_Tbl)
		if row, ok := el.(*wml.RawTableRow); ok {
			tbl.Children = []wml.TableRowElement{row}
		} else {
			tr := new(wml.CT_Tr)
			tr.Children = []wml.TableCellElement{el.(*wml.RawTableCell)}
			tbl.Children = []wml.TableRowElement{tr}
		}
		text = wml.BlocksText([]wml.BlockElement{tbl})
	}
	text = strings.TrimRight(text, "\n")
	if n := r.count(text); n > 0 {
		r.res.Skipped = append(r.res.Skipped, ReplaceSkip{Part: part, Text: text, Count: n})
	}
}

// paragraph replaces in p and reports the matches of its visible text that
// were not replaced.
func (r *replacer) paragraph(p *wml.CT_P, part string, skip textmerge.SkipSet) {
	text := p.Text()
	want := r.count(text)
	if want == 0 {
		return
	}
	before := r.res.Count
	r.inlines(&p.Children, skip)
	if missed := want - (r.res.Count - before); missed > 0 && !r.done() {
		r.res.Skipped = append(r.res.Skipped, ReplaceSkip{Part: part, Text: text, Count: missed})
	}
}

// inlines replaces in one inline container, then in its hyperlinks; each
// hyperlink is searched on its own, as a match cannot cross its edge.
func (r *replacer) inlines(children *[]wml.InlineElement, skip textmerge.SkipSet) {
	pos := 0
	for !r.done() {
		flat, pieces := textmerge.Flatten(*children, skip)
		start, end, ok := r.find(flat, pos)
		if !ok {
			break
		}
		merged, ok := textmerge.Collapse(*children, pieces, flat, start, end)
		if !ok {
			pos = end // unreachable: a match holds no barrier
			continue
		}
		*children = merged
		_, pieces = textmerge.Flatten(*children, skip) // piece indexes changed
		if !r.replaceAt(*children, pieces, start, end) {
			pos = end
			continue
		}
		r.res.Count++
		pos = start + len(r.new) // "\n" and "\t" flatten to one byte each
	}
	for _, c := range *children {
		if h, ok := c.(*wml.CT_Hyperlink); ok && !r.done() {
			r.inlines(&h.Children, skip)
		}
	}
}

// replaceAt rewrites the w:t holding flat[start:end] (one w:t, after
// Collapse) with the replacement.
func (r *replacer) replaceAt(children []wml.InlineElement, pieces []textmerge.Piece, start, end int) bool {
	for _, pc := range pieces {
		if pc.Kind != textmerge.PieceText || pc.Start > start || pc.End < end {
			continue
		}
		run, ok := children[pc.Inline].(*wml.CT_R)
		if !ok {
			return false
		}
		t, ok := run.Children[pc.Content].(*wml.CT_Text)
		if !ok {
			return false
		}
		pre, post := t.Content[:start-pc.Start], t.Content[end-pc.Start:]
		seq := r.content(pre, post, t.Space)
		run.Children = append(run.Children[:pc.Content], append(seq, run.Children[pc.Content+1:]...)...)
		return true
	}
	return false
}

// content builds the run content replacing one w:t: the text before the
// match, the replacement with its breaks and tabs, and the text after.
func (r *replacer) content(pre, post, space string) []wml.RunContentElement {
	var out []wml.RunContentElement
	var sb strings.Builder
	flush := func() {
		if sb.Len() == 0 {
			return
		}
		t := new(wml.CT_Text)
		t.Content = sb.String()
		t.Space = space
		textmerge.PreserveSpace(t)
		out = append(out, t)
		sb.Reset()
	}
	sb.WriteString(pre)
	for _, ch := range r.new {
		switch ch {
		case '\n':
			flush()
			out = append(out, new(wml.CT_Br))
		case '\t':
			flush()
			out = append(out, new(wml.CT_Tab))
		default:
			sb.WriteRune(ch)
		}
	}
	sb.WriteString(post)
	flush()
	return out
}

// count returns the number of non-overlapping matches of old in s.
func (r *replacer) count(s string) int {
	n, pos := 0, 0
	for {
		_, end, ok := r.find(s, pos)
		if !ok {
			return n
		}
		n++
		pos = end
	}
}

// find returns the byte range of the first match of old in s at or after
// from.
func (r *replacer) find(s string, from int) (start, end int, ok bool) {
	if from > len(s) {
		return 0, 0, false
	}
	if !r.fold {
		i := strings.Index(s[from:], r.old)
		if i < 0 {
			return 0, 0, false
		}
		return from + i, from + i + len(r.old), true
	}
	for i := from; i < len(s); {
		if n, ok := foldPrefix(s[i:], r.old); ok {
			return i, i + n, true
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return 0, 0, false
}

// foldPrefix reports whether s starts with sub under simple case folding,
// and the byte length of the matching prefix of s.
func foldPrefix(s, sub string) (int, bool) {
	n := 0
	for _, want := range sub {
		if n >= len(s) {
			return 0, false
		}
		got, size := utf8.DecodeRuneInString(s[n:])
		if got != want && !strings.EqualFold(string(got), string(want)) {
			return 0, false
		}
		n += size
	}
	return n, true
}
