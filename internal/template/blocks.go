package template

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Block tags come in two placements:
//
//   - Paragraph range: the open and close tags are each alone in a paragraph
//     of the same block list (body, cell, header). The marker paragraphs go;
//     the blocks between them are kept, dropped or repeated.
//   - Row range: the open tag starts a row's first cell and the close tag
//     ends the last cell of the same or a later row of the same table. The
//     tags are stripped; the rows from open to close are kept, dropped or
//     repeated.
//
// Each level pairs its own tags with a stack and handles only its outermost
// ranges; nested ranges are handled when the range body is expanded.

// mark is a block tag found at one level, with the index of the block or row
// that holds it.
type mark struct {
	idx int
	t   tag
	loc Location
}

// span is one outermost range found by pair: the indexes of its {{else}}
// (-1 when it has none) and of its close tag.
type span struct {
	els, close int
}

// hasElse reports whether the range has an {{else}} branch.
func (s span) hasElse() bool { return s.els >= 0 }

// pair matches open, else and close marks. It returns the outermost ranges
// (open index → span), or with all the ranges at every nesting level, and
// false, after reporting, if the level is not balanced. With report false it
// only computes the ranges.
func (en *engine) pair(marks []mark, report, all bool) (map[int]span, bool) {
	ranges := map[int]span{}
	ok := true
	fail := func(m mark, sentinel error, detail string) {
		ok = false
		if report {
			en.fail(m.loc, m.t, sentinel, detail)
		}
	}
	type openMark struct {
		m   mark
		els int
	}
	var stack []openMark
	for _, m := range marks {
		switch m.t.kind {
		case tagOpen:
			stack = append(stack, openMark{m: m, els: -1})
			continue
		case tagElse:
			if len(stack) == 0 {
				fail(m, ErrMisplacedBlock, "no {{#if}}, {{#unless}} or {{#each}} open before it in the same paragraph list or table")
				continue
			}
			top := &stack[len(stack)-1]
			if top.els >= 0 {
				fail(m, ErrUnbalancedBlock, "a second {{else}} for "+top.m.t.text+", opened at "+top.m.loc.String())
				continue
			}
			top.els = m.idx
			continue
		}
		if len(stack) == 0 {
			fail(m, ErrUnbalancedBlock, "no matching {{#"+m.t.block+"}} before it in the same paragraph list or table")
			continue
		}
		open := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if open.m.t.block != m.t.block {
			fail(m, ErrUnbalancedBlock, fmt.Sprintf("closes %s, opened at %s", open.m.t.text, open.m.loc))
			continue
		}
		if all || len(stack) == 0 {
			ranges[open.m.idx] = span{els: open.els, close: m.idx}
		}
	}
	for _, o := range stack {
		fail(o.m, ErrUnbalancedBlock, "no matching {{/"+o.m.t.block+"}} after it in the same paragraph list or table")
	}
	return ranges, ok
}

// isBlockTag reports whether t is an open, else or close tag.
func isBlockTag(t tag) bool {
	return t.kind == tagOpen || t.kind == tagClose || t.kind == tagElse
}

// marker returns the block tag that p consists of, if p's whole text is one
// open, else or close tag.
func marker(p *wml.CT_P) (tag, bool) {
	text := strings.TrimSpace(p.Text())
	if !strings.HasPrefix(text, "{{") {
		return tag{}, false
	}
	tags, bad := lexTags(text)
	if len(bad) > 0 || len(tags) != 1 {
		return tag{}, false
	}
	t := tags[0]
	if t.start != 0 || t.end != len(text) || !isBlockTag(t) {
		return tag{}, false
	}
	return t, true
}

// blockLocs returns the location of each block.
func blockLocs(blocks []wml.BlockElement, loc Location) []Location {
	locs := make([]Location, len(blocks))
	var np, nt int
	nraw := map[string]int{}
	for i, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			locs[i] = loc.child("p", np)
			np++
		case *wml.CT_Tbl:
			locs[i] = loc.child("tbl", nt)
			nt++
		default:
			// Name raw blocks by element: "sdt[0]", "customXml[1]".
			kind := "block"
			if rb, ok := el.(*wml.RawBlock); ok && rb.E != nil {
				kind = rb.E.QName.Local
			}
			locs[i] = loc.child(kind, nraw[kind])
			nraw[kind]++
		}
	}
	return locs
}

// maxNesting bounds how deeply block ranges nest along one path: paragraph
// ranges and row ranges counted together, through cells. Real templates
// nest a few levels. Without a bound, the template alone decides how deep
// expansion recurses and how often the row ranges' edge rows are examined.
const maxNesting = 100

// nestingDetail explains the nesting limit.
const nestingDetail = "block tags nest more than 100 deep"

// blockPlan is the structure of one block list, found once: its marker
// paragraphs, paired at every nesting level. Nested ranges and {{#each}}
// copies (clones, which hold the same blocks at the same indexes) are
// expanded from it by index, so no level lexes, pairs or scans its body
// again. Rescanning made N nested ranges cost N² (v0.5.0 review H5).
type blockPlan struct {
	locs []Location
	// opens holds the open tag at each range's first index.
	opens map[int]tag
	// spans holds every range, at every level, by its open index.
	spans map[int]span
	// sects[k] counts the section-break paragraphs among blocks[:k].
	sects []int
	// fieldsOK holds, in the structure check, whether each range's branches
	// hold whole fields.
	fieldsOK map[int]bool
}

// expandBlocks fills a story's or a cell's block list.
func (en *engine) expandBlocks(blocks []wml.BlockElement, sc *scope, loc Location) []wml.BlockElement {
	pl, ok := en.planBlocks(blocks, blockLocs(blocks, loc))
	if !ok {
		return blocks
	}
	return en.expandPlanned(blocks, pl, 0, sc, make([]wml.BlockElement, 0, len(blocks)))
}

// planBlocks finds and pairs the marker paragraphs of a block list. It
// returns false, after reporting, when they do not pair or nest too deeply.
func (en *engine) planBlocks(blocks []wml.BlockElement, locs []Location) (*blockPlan, bool) {
	pl := &blockPlan{locs: locs, opens: map[int]tag{}, spans: nil, sects: make([]int, len(blocks)+1), fieldsOK: nil}
	var marks []mark
	for i, b := range blocks {
		pl.sects[i+1] = pl.sects[i]
		p, ok := b.(*wml.CT_P)
		if !ok {
			continue
		}
		if p.PPr != nil && p.PPr.SectPr != nil {
			pl.sects[i+1]++
		}
		if t, ok := marker(p); ok {
			marks = append(marks, mark{idx: i, t: t, loc: locs[i]})
			if t.kind == tagOpen {
				pl.opens[i] = t
			}
		}
	}
	// A balanced list is balanced inside every range too, so pairing the
	// whole list once covers every level.
	spans, ok := en.pair(marks, true, true)
	if !ok {
		// The structure is broken and Execute will fail; filling the rest
		// would only add noise (".field" outside its loop) to the report.
		// Misplaced block tags are still worth reporting: one is often why
		// the level does not pair.
		en.reportMisplaced(blocks, locs)
		return nil, false
	}
	if !en.nestingFits(marks) {
		return nil, false
	}
	pl.spans = spans
	if en.check && len(spans) > 0 {
		pl.fieldsOK = fieldsOKOf(blocks, spans)
	}
	return pl, true
}

// nestingFits reports, after reporting the first range too deep, whether the
// balanced marks nest within maxNesting from the current depth.
func (en *engine) nestingFits(marks []mark) bool {
	depth := en.depth
	for _, m := range marks {
		switch m.t.kind {
		case tagOpen:
			depth++
			if depth > maxNesting {
				if !en.tooDeep {
					en.tooDeep = true
					en.fail(m.loc, m.t, ErrOutputLimit, nestingDetail)
				}
				return false
			}
		case tagClose:
			depth--
		}
	}
	return true
}

// enter counts one more level of nesting. It reports false, once per
// Execute, with an ErrOutputLimit issue, when the levels would exceed
// maxNesting; the range is then left unexpanded.
func (en *engine) enter(loc Location, open tag) bool {
	if en.depth >= maxNesting {
		if !en.tooDeep {
			en.tooDeep = true
			en.fail(loc, open, ErrOutputLimit, nestingDetail)
		}
		return false
	}
	en.depth++
	return true
}

func (en *engine) leave() { en.depth-- }

// expandPlanned appends the expansion of blocks to out. blocks[0] is block
// base of the list pl was planned from, or its clone in a loop copy.
func (en *engine) expandPlanned(blocks []wml.BlockElement, pl *blockPlan, base int, sc *scope, out []wml.BlockElement) []wml.BlockElement {
	for i := 0; i < len(blocks); i++ {
		if sp, isOpen := pl.spans[base+i]; isOpen {
			out = en.expandBlockRange(blocks, pl, base, i, sp, sc, out)
			i = sp.close - base
			continue
		}
		loc := pl.locs[base+i]
		switch el := blocks[i].(type) {
		case *wml.CT_P:
			en.fillParagraph(el, sc, loc)
			out = append(out, el)
		case *wml.CT_Tbl:
			if en.expandTable(el, sc, loc) {
				out = append(out, el)
			}
		case *wml.RawBlock:
			en.fillRaw(el.E, sc, loc)
			out = append(out, el)
		default:
			out = append(out, el)
		}
	}
	return out
}

// fieldSum summarises the field characters of some content for
// wml.FieldsBalanced, so that the summaries of consecutive pieces combine:
// net is the change in field depth, low the lowest depth reached relative
// to the start (a separate counts as one below its depth, since it needs an
// open field).
type fieldSum struct{ net, low int }

// then returns the summary of s followed by b.
func (s fieldSum) then(b fieldSum) fieldSum {
	return fieldSum{net: s.net + b.net, low: min(s.low, s.net+b.low)}
}

// whole reports whether the content holds whole fields.
func (s fieldSum) whole() bool { return s.net == 0 && s.low >= 0 }

func fieldSumOf(node any) fieldSum {
	var s fieldSum
	for _, mk := range wml.MarksIn(node) {
		if mk.Kind != wml.MarkFldChar {
			continue
		}
		switch mk.FldCharType {
		case wml.FldCharBegin:
			s.net++
		case wml.FldCharSeparate:
			s.low = min(s.low, s.net-1)
		case wml.FldCharEnd:
			s.net--
			s.low = min(s.low, s.net)
		}
	}
	return s
}

// fieldsOKOf reports, for each range of a paired block list, whether its
// branches hold whole fields. Each block is summarised once and each
// range's summary is built from its parts, so the cost is linear however
// deep the ranges nest.
func fieldsOKOf(blocks []wml.BlockElement, spans map[int]span) map[int]bool {
	ok := make(map[int]bool, len(spans))
	var sum func(lo, hi int) fieldSum
	sum = func(lo, hi int) fieldSum {
		var s fieldSum
		for i := lo; i < hi; i++ {
			sp, isOpen := spans[i]
			if !isOpen {
				s = s.then(fieldSumOf(blocks[i]))
				continue
			}
			thenEnd := sp.close
			if sp.hasElse() {
				thenEnd = sp.els
			}
			body := sum(i+1, thenEnd)
			r := fieldSumOf(blocks[i]).then(body)
			whole := body.whole()
			if sp.hasElse() {
				els := sum(sp.els+1, sp.close)
				whole = whole && els.whole()
				r = r.then(fieldSumOf(blocks[sp.els])).then(els)
			}
			ok[i] = whole
			s = s.then(r.then(fieldSumOf(blocks[sp.close])))
			i = sp.close
		}
		return s
	}
	sum(0, len(blocks))
	return ok
}

// reportMisplaced reports the block tags in paragraphs that are not marker
// paragraphs.
func (en *engine) reportMisplaced(blocks []wml.BlockElement, locs []Location) {
	for i, b := range blocks {
		p, ok := b.(*wml.CT_P)
		if !ok {
			continue
		}
		if _, isMarker := marker(p); isMarker {
			continue
		}
		en.reportInlineLeftovers(paragraphBlockTags(p), locs[i])
	}
}

// reportInlineLeftovers reports the block tags of a paragraph that are not
// part of an inline range, and inline {{#each}} ranges.
func (en *engine) reportInlineLeftovers(tags []tag, loc Location) {
	roots, left := inlinePairs(tags)
	for _, l := range left {
		en.fail(loc, l.t, l.sentinel, l.detail)
	}
	var walk func([]*inlinePair)
	walk = func(ps []*inlinePair) {
		for _, p := range ps {
			if tags[p.open].block == "each" {
				en.fail(loc, tags[p.open], ErrMisplacedBlock, inlineEachDetail)
				continue
			}
			walk(p.children)
		}
	}
	walk(roots)
}

// reportMisplacedRows reports block tags in rows that are neither row-edge
// tags nor in marker paragraphs.
func (en *engine) reportMisplacedRows(rows []wml.TableRowElement, rlocs []Location) {
	for k, r := range rows {
		row, ok := r.(*wml.CT_Tr)
		if !ok {
			continue
		}
		// The paragraphs holding this row's edge tags, and which end to skip.
		var openPara, closePara *wml.CT_P
		e := en.edges(row)
		if e.hasOpen || e.hasElse {
			openPara = edgeParagraph(row, true)
		}
		if e.hasClose {
			closePara = edgeParagraph(row, false)
		}
		nc := 0
		for _, c := range row.Children {
			cell, ok := c.(*wml.CT_Tc)
			if !ok {
				continue
			}
			locs := blockLocs(cell.Children, rlocs[k].child("tc", nc))
			for i, b := range cell.Children {
				p, ok := b.(*wml.CT_P)
				if !ok {
					continue
				}
				if _, isMarker := marker(p); isMarker {
					continue
				}
				tags := paragraphBlockTags(p)
				if p == openPara && len(tags) > 0 {
					tags = tags[1:] // the row's open or else tag
				}
				if p == closePara && len(tags) > 0 {
					tags = tags[:len(tags)-1] // the row's close tag
				}
				en.reportInlineLeftovers(tags, locs[i])
				en.reportRawBlockTags(p, locs[i])
			}
			nc++
		}
	}
}

// reportRawBlockTags reports block tags held in p's inline content other than
// runs and hyperlinks (tracked insertions, content controls). They are not
// reported by the fill path when the rows are left unfilled because they do
// not pair, and a row-edge tag held there is the usual reason they do not.
func (en *engine) reportRawBlockTags(p *wml.CT_P, loc Location) {
	for _, c := range p.Children {
		switch c.(type) {
		case *wml.CT_R, *wml.CT_Hyperlink:
			continue
		}
		text := (&wml.CT_P{PPr: nil, Children: []wml.InlineElement{c}, Attrs: nil}).Text()
		tags, _ := lexTags(text)
		for _, t := range tags {
			if isBlockTag(t) {
				en.fail(loc, t, ErrUnsupportedLocation, "block and image tags are not supported "+rawLocation)
			}
		}
	}
}

// edgeParagraph returns the first paragraph of a row's first cell or the last
// paragraph of its last cell.
func edgeParagraph(row *wml.CT_Tr, first bool) *wml.CT_P {
	cell := edgeCell(row, first)
	if cell == nil || len(cell.Children) == 0 {
		return nil
	}
	idx := 0
	if !first {
		idx = len(cell.Children) - 1
	}
	p, _ := cell.Children[idx].(*wml.CT_P)
	return p
}

// paragraphBlockTags returns the open and close tags of a paragraph in order.
func paragraphBlockTags(p *wml.CT_P) []tag {
	var out []tag
	for _, txt := range paragraphTexts(p.Children) {
		tags, _ := lexTags(txt.Content)
		for _, t := range tags {
			if isBlockTag(t) {
				out = append(out, t)
			}
		}
	}
	return out
}

// misplacedDetail explains where block tags may go.
const misplacedDetail = "a block tag must be alone in its paragraph, or at the start of a row's first cell or the end of its last cell"

// expandBlockRange appends to out the expansion of the range whose open
// marker is blocks[i] (block base+i of pl's list). The range's first and last
// blocks, and the block at sp.els if any, are its marker paragraphs; sp
// holds indexes into pl's list.
func (en *engine) expandBlockRange(blocks []wml.BlockElement, pl *blockPlan, base, i int, sp span, sc *scope, out []wml.BlockElement) []wml.BlockElement {
	g := base + i
	open := pl.opens[g]
	loc := pl.locs[g]
	j := sp.close - base
	if pl.sects[sp.close+1] > pl.sects[g] {
		for k := i; k <= j; k++ {
			if p, ok := blocks[k].(*wml.CT_P); ok && p.PPr != nil && p.PPr.SectPr != nil {
				en.fail(pl.locs[base+k], open, ErrSectionBreakInBlock, "move the section break outside "+open.text+" … {{/"+open.block+"}}")
				return out
			}
		}
	}
	if !en.enter(loc, open) {
		return out
	}
	defer en.leave()
	// The then branch runs to the {{else}} marker; the else branch follows it.
	thenEnd := sp.close
	if sp.hasElse() {
		thenEnd = sp.els
	}
	body := blocks[i+1 : thenEnd-base]
	elseBranch := func(out []wml.BlockElement) []wml.BlockElement {
		if !sp.hasElse() {
			return out
		}
		return en.expandPlanned(blocks[sp.els-base+1:j], pl, sp.els+1, sc, out)
	}
	if en.check {
		markers := []*wml.CT_P{blockParagraph(blocks[i]), blockParagraph(blocks[j])}
		if sp.hasElse() {
			markers = append(markers, blockParagraph(blocks[sp.els-base]))
		}
		en.checkBlockDrop(pl.fieldsOK[g], markers, loc, open)
		// Visit both branches; a loop body once, in the loop's scope so
		// ".field" tags are not reported as outside a loop. The check's
		// output is not used.
		inner := sc
		if open.block == "each" {
			inner = sc.push(nil, open.path+"[]")
		}
		en.expandPlanned(body, pl, g+1, inner, nil)
		elseBranch(nil)
		return out
	}
	if open.block != "each" {
		if !en.condition(open, sc) {
			return elseBranch(out)
		}
		return en.expandPlanned(body, pl, g+1, sc, out)
	}
	list, ok := en.list(open, sc, loc, sp.hasElse())
	if !ok {
		return out
	}
	if len(list) == 0 {
		return elseBranch(out)
	}
	copies := copiesOf(en, body, len(list), open, loc)
	// copies is shorter than list when copying stopped (an error, or the
	// output limit); execution fails then, and the rest is not filled.
	for k, cp := range copies {
		out = en.expandPlanned(cp, pl, g+1, sc.push(list[k], itemName(sc, open.path, k)), out)
	}
	return out
}

// expandTable expands row ranges and fills every cell. It returns false when
// the table has no rows left and must be removed.
func (en *engine) expandTable(t *wml.CT_Tbl, sc *scope, loc Location) bool {
	rlocs := make([]Location, len(t.Children))
	hadRows := false
	nr := 0
	for k, r := range t.Children {
		rlocs[k] = loc
		if _, ok := r.(*wml.CT_Tr); ok {
			rlocs[k] = loc.child("tr", nr)
			nr++
			hadRows = true
		}
	}
	t.Children = en.expandRows(t.Children, rlocs, sc, make([]wml.TableRowElement, 0, len(t.Children)))
	if !hadRows {
		return true
	}
	for _, r := range t.Children {
		if _, ok := r.(*wml.CT_Tr); ok {
			return true
		}
	}
	en.warn(loc, tag{}, "every row was removed, so the table was removed")
	return false
}

// rowEdges are the block tags at a row's edges: an open or else tag at the
// start of its first cell, a close tag at the end of its last.
type rowEdges struct {
	open, els, close           tag
	hasOpen, hasElse, hasClose bool
}

// edges returns a row's edge tags. Row ranges are found level by level (an
// edge tag shows once the range around it has stripped its own), so each
// level looks at every row of its body again; the result is kept until the
// row is stripped, so a row's text is lexed once per strip, not once per
// level.
func (en *engine) edges(row *wml.CT_Tr) rowEdges {
	if e, ok := en.rowEdgeMemo[row]; ok {
		return e
	}
	var e rowEdges
	e.open, e.hasOpen = en.rowOpen(row)
	if !e.hasOpen {
		e.els, e.hasElse = rowElse(row)
	}
	e.close, e.hasClose = en.rowClose(row)
	if en.rowEdgeMemo == nil {
		en.rowEdgeMemo = map[*wml.CT_Tr]rowEdges{}
	}
	en.rowEdgeMemo[row] = e
	return e
}

// expandRows appends to out the expansion of the row ranges among rows and
// the other rows, filled.
func (en *engine) expandRows(rows []wml.TableRowElement, rlocs []Location, sc *scope, out []wml.TableRowElement) []wml.TableRowElement {
	var marks []mark
	opens := map[int]tag{}
	for k, r := range rows {
		row, ok := r.(*wml.CT_Tr)
		if !ok {
			continue
		}
		e := en.edges(row)
		if e.hasOpen {
			marks = append(marks, mark{idx: k, t: e.open, loc: rlocs[k]})
			opens[k] = e.open
		} else if e.hasElse {
			marks = append(marks, mark{idx: k, t: e.els, loc: rlocs[k]})
		}
		if e.hasClose {
			marks = append(marks, mark{idx: k, t: e.close, loc: rlocs[k]})
		}
	}
	ranges, ok := en.pair(marks, true, false)
	if !ok {
		// As for paragraph ranges: report block tags that are not at a row
		// edge (a close tag in the last cell but not at its end, say), which
		// is often why the rows do not pair.
		en.reportMisplacedRows(rows, rlocs)
		return append(out, rows...)
	}
	for k := 0; k < len(rows); k++ {
		if sp, isOpen := ranges[k]; isOpen {
			out = en.expandRowRange(rows, rlocs, k, sp, opens[k], sc, out)
			k = sp.close
			continue
		}
		switch row := rows[k].(type) {
		case *wml.CT_Tr:
			en.fillRow(row, sc, rlocs[k])
		case *wml.RawTableRow:
			en.fillRaw(row.E, sc, rlocs[k])
		}
		out = append(out, rows[k])
	}
	return out
}

// rowEdgeSplitDetail explains a row-edge tag that cannot be stripped.
const rowEdgeSplitDetail = "a block tag at a row's edge must be one piece of text; " +
	"this one is split by a bookmark, comment mark or other element, so delete it and type it again"

// expandRowRange appends to out the expansion of rows[k..sp.close], whose
// edge cells hold open and its close tag. With an {{else}}, the row starting
// with it begins the else branch: rows k..sp.els-1 are the then branch,
// sp.els..sp.close the else branch.
func (en *engine) expandRowRange(rows []wml.TableRowElement, rlocs []Location, k int, sp span, open tag, sc *scope, out []wml.TableRowElement) []wml.TableRowElement {
	j := sp.close
	first, _ := rows[k].(*wml.CT_Tr)
	last, _ := rows[j].(*wml.CT_Tr)
	// The close tag is found before any tag is stripped: stripping the else
	// tag from a one-cell row could otherwise change what rowClose sees.
	ends := en.edges(last)
	stripped := en.stripRowEdge(first, open, true)
	if sp.hasElse() {
		elseRow, _ := rows[sp.els].(*wml.CT_Tr)
		e := en.edges(elseRow)
		stripped = stripped && e.hasElse && en.stripRowEdge(elseRow, e.els, true)
	}
	stripped = stripped && ends.hasClose && en.stripRowEdge(last, ends.close, false)
	if !stripped {
		// The tag was found in the row's edge text but is not whole in one
		// w:t. Expanding the rows would find the same range again, without
		// end (v0.5.0 review H1).
		en.fail(rlocs[k], open, ErrSplitAcrossBoundary, rowEdgeSplitDetail)
		return out
	}
	if !en.enter(rlocs[k], open) {
		return out
	}
	defer en.leave()
	thenEnd := j + 1
	if sp.hasElse() {
		thenEnd = sp.els
	}
	body, bodyLocs := rows[k:thenEnd], rlocs[k:thenEnd]
	elseBranch := func(out []wml.TableRowElement) []wml.TableRowElement {
		if !sp.hasElse() {
			return out
		}
		return en.expandRows(rows[sp.els:j+1], rlocs[sp.els:j+1], sc, out)
	}
	if en.check {
		var elseRows []wml.TableRowElement
		if sp.hasElse() {
			elseRows = rows[sp.els : j+1]
		}
		en.checkRowDrop([][]wml.TableRowElement{body, elseRows}, rlocs[k], open)
		inner := sc
		if open.block == "each" {
			inner = sc.push(nil, open.path+"[]")
		}
		en.expandRows(body, bodyLocs, inner, nil)
		elseBranch(nil)
		return out
	}
	if open.block != "each" {
		if !en.condition(open, sc) {
			return elseBranch(out)
		}
		return en.expandRows(body, bodyLocs, sc, out)
	}
	list, ok := en.list(open, sc, rlocs[k], sp.hasElse())
	if !ok {
		return out
	}
	if len(list) == 0 {
		return elseBranch(out)
	}
	copies := copiesOf(en, body, len(list), open, rlocs[k])
	for n, cp := range copies { // shorter than list after a failure, as for blocks
		out = en.expandRows(cp, bodyLocs, sc.push(list[n], itemName(sc, open.path, n)), out)
	}
	return out
}

// fillRow fills every cell of a row, keeping each cell schema-valid.
func (en *engine) fillRow(row *wml.CT_Tr, sc *scope, rloc Location) {
	nc := 0
	for _, c := range row.Children {
		if rc, isRaw := c.(*wml.RawTableCell); isRaw {
			en.fillRaw(rc.E, sc, rloc)
			continue
		}
		cell, ok := c.(*wml.CT_Tc)
		if !ok {
			continue
		}
		endedWithP := endsWithParagraph(cell.Children)
		cell.Children = en.expandBlocks(cell.Children, sc, rloc.child("tc", nc))
		// A cell needs a block, and Word wants it to end with a paragraph. If
		// a removed range took the cell's closing paragraph, add one back;
		// a cell that already ended with a table is left as it was.
		if len(cell.Children) == 0 || (endedWithP && !endsWithParagraph(cell.Children)) {
			cell.Children = append(cell.Children, emptyParagraph())
		}
		nc++
	}
}

// blockParagraph returns b as a paragraph, or nil.
func blockParagraph(b wml.BlockElement) *wml.CT_P {
	p, _ := b.(*wml.CT_P)
	return p
}

func endsWithParagraph(blocks []wml.BlockElement) bool {
	if len(blocks) == 0 {
		return false
	}
	_, ok := blocks[len(blocks)-1].(*wml.CT_P)
	return ok
}

func emptyParagraph() *wml.CT_P {
	return &wml.CT_P{PPr: nil, Children: nil, Attrs: nil}
}

// rowOpen returns the open tag at the start of a row's first cell. A tag
// alone in its paragraph that is closed inside the same cell is a paragraph
// range of that cell, not a row range.
func (en *engine) rowOpen(row *wml.CT_Tr) (tag, bool) {
	cell := edgeCell(row, true)
	if cell == nil || len(cell.Children) == 0 {
		return tag{}, false
	}
	p, ok := cell.Children[0].(*wml.CT_P)
	if !ok {
		return tag{}, false
	}
	text := strings.TrimLeft(edgeText(p, true), " \t")
	tags, _ := lexTags(text)
	if len(tags) == 0 || tags[0].start != 0 || tags[0].kind != tagOpen {
		return tag{}, false
	}
	if _, whole := marker(p); whole && en.pairedInCell(cell, 0) {
		return tag{}, false
	}
	if inlineAtEdge(row, p, true) {
		return tag{}, false
	}
	return tags[0], true
}

// rowClose returns the close tag at the end of a row's last cell, with the
// same exception as rowOpen.
func (en *engine) rowClose(row *wml.CT_Tr) (tag, bool) {
	cell := edgeCell(row, false)
	if cell == nil || len(cell.Children) == 0 {
		return tag{}, false
	}
	last := len(cell.Children) - 1
	p, ok := cell.Children[last].(*wml.CT_P)
	if !ok {
		return tag{}, false
	}
	text := strings.TrimRight(edgeText(p, false), " \t")
	tags, _ := lexTags(text)
	if len(tags) == 0 {
		return tag{}, false
	}
	t := tags[len(tags)-1]
	if t.end != len(text) || t.kind != tagClose {
		return tag{}, false
	}
	if _, whole := marker(p); whole && en.pairedInCell(cell, last) {
		return tag{}, false
	}
	if inlineAtEdge(row, p, false) {
		return tag{}, false
	}
	return t, true
}

// pairedInCell reports whether the marker paragraph at index idx of cell is
// one end of a balanced paragraph range inside the cell.
func (en *engine) pairedInCell(cell *wml.CT_Tc, idx int) bool {
	var marks []mark
	for i, b := range cell.Children {
		if p, ok := b.(*wml.CT_P); ok {
			if t, ok := marker(p); ok {
				marks = append(marks, mark{idx: i, t: t, loc: Location{}})
			}
		}
	}
	ranges, _ := en.pair(marks, false, false)
	for open, sp := range ranges {
		if open == idx || sp.close == idx || sp.els == idx {
			return true
		}
	}
	return false
}

// rowElse returns the {{else}} at the start of a row's first cell. An else
// there can never belong to a paragraph range of that cell: such a range
// would need its open tag before the else, in the same cell.
func rowElse(row *wml.CT_Tr) (tag, bool) {
	cell := edgeCell(row, true)
	if cell == nil || len(cell.Children) == 0 {
		return tag{}, false
	}
	p, ok := cell.Children[0].(*wml.CT_P)
	if !ok {
		return tag{}, false
	}
	text := strings.TrimLeft(edgeText(p, true), " \t")
	tags, _ := lexTags(text)
	if len(tags) == 0 || tags[0].start != 0 || tags[0].kind != tagElse {
		return tag{}, false
	}
	return tags[0], true
}

// edgeText returns the text at the start (first) or end of p that
// stripRowEdge can edit: the text of the runs and hyperlinks up to the first
// other element that shows text (a tracked insertion, a content control, a
// field result in raw content). Row-edge tags are looked for in this text
// only. Looking in p.Text() found tags that stripRowEdge could not remove, so
// every copy of the row re-opened the range and expansion never ended.
func edgeText(p *wml.CT_P, first bool) string {
	n := len(p.Children)
	parts := make([]string, 0, n)
	for k := range n {
		i := k
		if !first {
			i = n - 1 - k
		}
		c := p.Children[i]
		text := (&wml.CT_P{PPr: nil, Children: []wml.InlineElement{c}, Attrs: nil}).Text()
		switch c.(type) {
		case *wml.CT_R, *wml.CT_Hyperlink:
			parts = append(parts, text)
			continue
		}
		if text != "" {
			break
		}
	}
	if !first {
		slices.Reverse(parts)
	}
	return strings.Join(parts, "")
}

// edgeCell returns a row's first or last typed cell.
func edgeCell(row *wml.CT_Tr, first bool) *wml.CT_Tc {
	var found *wml.CT_Tc
	for _, c := range row.Children {
		if cell, ok := c.(*wml.CT_Tc); ok {
			if first {
				return cell
			}
			found = cell
		}
	}
	return found
}

// stripRowEdge removes a row-range tag from the start of the row's first
// cell (open) or the end of its last cell (close). A paragraph left blank
// is removed when the cell has other content. The tag must lie whole in the
// first (or last) w:t that holds more than spaces and tabs, which is where
// edgeText found it unless it is split by an element the run merge does not
// cross; stripRowEdge reports false, and changes nothing, otherwise.
func (en *engine) stripRowEdge(row *wml.CT_Tr, t tag, open bool) bool {
	cell := edgeCell(row, open)
	if cell == nil || len(cell.Children) == 0 {
		return false
	}
	idx := 0
	if !open {
		idx = len(cell.Children) - 1
	}
	p, ok := cell.Children[idx].(*wml.CT_P)
	if !ok {
		return false
	}
	texts := paragraphTexts(p.Children)
	if !open {
		slices.Reverse(texts)
	}
	var tx *wml.CT_Text
	for _, x := range texts {
		if strings.Trim(x.Content, " \t") != "" {
			tx = x
			break
		}
	}
	if tx == nil {
		return false
	}
	if open {
		rest := strings.TrimLeft(tx.Content, " \t")
		if !strings.HasPrefix(rest, t.text) {
			return false
		}
		i := len(tx.Content) - len(rest)
		tx.Content = tx.Content[:i] + tx.Content[i+len(t.text):]
	} else {
		rest := strings.TrimRight(tx.Content, " \t")
		if !strings.HasSuffix(rest, t.text) {
			return false
		}
		tx.Content = tx.Content[:len(rest)-len(t.text)] + tx.Content[len(rest):]
	}
	delete(en.rowEdgeMemo, row)
	if len(cell.Children) > 1 && blankParagraph(p) {
		cell.Children = append(cell.Children[:idx:idx], cell.Children[idx+1:]...)
	}
	return true
}

// blankParagraph reports whether p holds nothing but empty text: no other
// run content, no inline elements other than runs, no section break.
func blankParagraph(p *wml.CT_P) bool {
	if p.PPr != nil && p.PPr.SectPr != nil {
		return false
	}
	for _, c := range p.Children {
		r, ok := c.(*wml.CT_R)
		if !ok {
			return false
		}
		for _, rc := range r.Children {
			if t, ok := rc.(*wml.CT_Text); !ok || t.Content != "" {
				return false
			}
		}
	}
	return true
}

// condition evaluates an {{#if}} or {{#unless}}. A missing key is false, not
// an error: an absent optional section is normal data.
func (en *engine) condition(open tag, sc *scope) bool {
	v := truthy(sc.lookup(open.path))
	if open.block == "unless" {
		return !v
	}
	return v
}

// list resolves an {{#each}} path. A missing list is a missing key (a typo
// in a list name must not silently produce nothing); null is an empty list.
// With an {{else}} branch (missingOK), a missing list is empty instead, as
// a missing key is false in {{#if}}: the else branch is the template's
// answer to "no items".
func (en *engine) list(open tag, sc *scope, loc Location, missingOK bool) ([]any, bool) {
	r := sc.lookup(open.path)
	if !r.found {
		if missingOK {
			return nil, true
		}
		en.missing(loc, open, r.detail)
		return nil, false
	}
	l, ok := items(r.value)
	if !ok {
		en.fail(loc, open, ErrUnsupportedValue, open.path+" is not a list")
		return nil, false
	}
	return l, true
}

// copiesOf returns n copies of els for an {{#each}}: the first is els
// itself, the rest are clones fixed up so ids stay unique (CloneFixup).
// Clones are taken before any copy is filled.
func copiesOf[T wml.Cloneable](en *engine, els []T, n int, open tag, loc Location) [][]T {
	if n == 0 {
		return nil
	}
	if !en.spendCopies((n-1)*len(els), loc, open) {
		return [][]T{els}
	}
	fx := &wml.CloneFixup{
		StripParaIDs:       true,
		DropBookmarks:      true,
		DropCommentAnchors: true,
		NextDocPrID:        en.nextDocPr,
		DroppedComments:    0,
		NoteRefs:           0,
	}
	out := make([][]T, n)
	out[0] = els
	for k := 1; k < n; k++ {
		cp := make([]T, len(els))
		for i, el := range els {
			c, err := wml.CloneWith(el, fx)
			if err != nil {
				if en.err == nil {
					en.err = err
				}
				return out[:1]
			}
			cp[i] = c
		}
		out[k] = cp
	}
	if fx.DroppedComments > 0 {
		en.warn(loc, open, "comments on repeated content are kept on the first copy only")
	}
	if fx.NoteRefs > 0 {
		en.warn(loc, open, "footnote or endnote references were repeated; every copy shows the same note")
	}
	return out
}

// spendCopies takes n copies from the run's budget. It reports false, once
// with an ErrOutputLimit issue, when the budget does not cover them.
func (en *engine) spendCopies(n int, loc Location, open tag) bool {
	if en.copiesLeft < 0 {
		return true
	}
	if n > en.copiesLeft {
		if !en.overLimit {
			en.overLimit = true
			en.fail(loc, open, ErrOutputLimit, fmt.Sprintf(
				"the loops would copy more than %d paragraphs, tables and rows; raise Options.MaxCopies if the output is meant to be this large",
				copyBudget(en.opts.MaxCopies)))
		}
		en.copiesLeft = 0
		return false
	}
	en.copiesLeft -= n
	return true
}

// itemName names one loop item in messages: "threats[3]", or for a relative
// path, "threats[3].mitigations[0]".
func itemName(sc *scope, path string, k int) string {
	base := path
	switch {
	case path == ".":
		base = sc.itemPath
	case strings.HasPrefix(path, "."):
		base = sc.itemPath + path
	}
	return fmt.Sprintf("%s[%d]", base, k)
}

// push returns the scope for one {{#each}} item.
func (s *scope) push(item any, path string) *scope {
	return &scope{root: s.root, item: item, itemPath: path, inLoop: true}
}

// truthy decides {{#if}} and {{#unless}}: missing, nil, false, "", zero and
// empty lists or maps are false; everything else is true.
func truthy(r lookupResult) bool {
	if !r.found {
		return false
	}
	switch x := r.value.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, err := x.Float64()
		return err != nil || f != 0
	case []any:
		return len(x) > 0
	case []map[string]any:
		return len(x) > 0
	case []string:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	case map[string]string:
		return len(x) > 0
	}
	return !isZeroNumber(r.value)
}

// isZeroNumber reports whether v is a Go number equal to zero.
func isZeroNumber(v any) bool {
	switch x := v.(type) {
	case float64:
		return x == 0
	case float32:
		return x == 0
	case int:
		return x == 0
	case int8:
		return x == 0
	case int16:
		return x == 0
	case int32:
		return x == 0
	case int64:
		return x == 0
	case uint:
		return x == 0
	case uint8:
		return x == 0
	case uint16:
		return x == 0
	case uint32:
		return x == 0
	case uint64:
		return x == 0
	}
	return false
}

// items returns the list an {{#each}} iterates. ok is false when the value
// is not a list.
func items(v any) ([]any, bool) {
	switch x := v.(type) {
	case nil:
		return nil, true
	case []any:
		return x, true
	case []map[string]any:
		out := make([]any, len(x))
		for i, m := range x {
			out[i] = m
		}
		return out, true
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}
