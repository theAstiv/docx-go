package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Block tags come in two placements (CLAUDE.md, 2026-10-17):
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

// pair matches open and close marks. It returns the outermost ranges (open
// index → close index) and false, after reporting, if the level is not
// balanced. With report false it only computes the ranges.
func (en *engine) pair(marks []mark, report bool) (map[int]int, bool) {
	ranges := map[int]int{}
	ok := true
	fail := func(m mark, detail string) {
		ok = false
		if report {
			en.fail(m.loc, m.t, ErrUnbalancedBlock, detail)
		}
	}
	var stack []mark
	for _, m := range marks {
		if m.t.kind == tagOpen {
			stack = append(stack, m)
			continue
		}
		if len(stack) == 0 {
			fail(m, "no matching {{#"+m.t.block+"}} before it in the same paragraph list or table")
			continue
		}
		open := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if open.t.block != m.t.block {
			fail(m, fmt.Sprintf("closes %s, opened at %s", open.t.text, open.loc))
			continue
		}
		if len(stack) == 0 {
			ranges[open.idx] = m.idx
		}
	}
	for _, m := range stack {
		fail(m, "no matching {{/"+m.t.block+"}} after it in the same paragraph list or table")
	}
	return ranges, ok
}

// marker returns the block tag that p consists of, if p's whole text is one
// open or close tag.
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
	if t.start != 0 || t.end != len(text) || (t.kind != tagOpen && t.kind != tagClose) {
		return tag{}, false
	}
	return t, true
}

// blockLocs returns the location of each block.
func blockLocs(blocks []wml.BlockElement, loc Location) []Location {
	locs := make([]Location, len(blocks))
	var np, nt, nsdt int
	for i, b := range blocks {
		switch b.(type) {
		case *wml.CT_P:
			locs[i] = loc.child("p", np)
			np++
		case *wml.CT_Tbl:
			locs[i] = loc.child("tbl", nt)
			nt++
		default:
			locs[i] = loc.child("other", nsdt)
			nsdt++
		}
	}
	return locs
}

// expandBlocks fills a story's or a cell's block list.
func (en *engine) expandBlocks(blocks []wml.BlockElement, sc *scope, loc Location) []wml.BlockElement {
	return en.expandRange(blocks, blockLocs(blocks, loc), sc)
}

// expandRange expands the block ranges and fills the other blocks.
func (en *engine) expandRange(blocks []wml.BlockElement, locs []Location, sc *scope) []wml.BlockElement {
	var marks []mark
	for i, b := range blocks {
		if p, ok := b.(*wml.CT_P); ok {
			if t, ok := marker(p); ok {
				marks = append(marks, mark{idx: i, t: t, loc: locs[i]})
			}
		}
	}
	ranges, ok := en.pair(marks, true)
	if !ok {
		// The structure is broken and Execute will fail; filling the rest
		// would only add noise (".field" outside its loop) to the report.
		// Misplaced block tags are still worth reporting: one is often why
		// the level does not pair.
		en.reportMisplaced(blocks, locs)
		return blocks
	}
	opens := map[int]tag{}
	for _, m := range marks {
		opens[m.idx] = m.t
	}
	out := make([]wml.BlockElement, 0, len(blocks))
	for i := 0; i < len(blocks); i++ {
		if j, isOpen := ranges[i]; isOpen {
			out = append(out, en.expandBlockRange(blocks, locs, i, j, opens[i], sc)...)
			i = j
			continue
		}
		switch el := blocks[i].(type) {
		case *wml.CT_P:
			en.fillParagraph(el, sc, locs[i])
			out = append(out, el)
		case *wml.CT_Tbl:
			if en.expandTable(el, sc, locs[i]) {
				out = append(out, el)
			}
		default:
			out = append(out, el)
		}
	}
	return out
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
		for _, txt := range paragraphTexts(p.Children) {
			tags, _ := lexTags(txt.Content)
			for _, t := range tags {
				if t.kind == tagOpen || t.kind == tagClose {
					en.fail(locs[i], t, ErrMisplacedBlock, misplacedDetail)
				}
			}
		}
	}
}

// misplacedDetail explains where block tags may go.
const misplacedDetail = "a block tag must be alone in its paragraph, or at the start of a row's first cell or the end of its last cell"

// expandBlockRange handles blocks[i..j], whose first and last blocks are the
// marker paragraphs of open.
func (en *engine) expandBlockRange(blocks []wml.BlockElement, locs []Location, i, j int, open tag, sc *scope) []wml.BlockElement {
	for k := i; k <= j; k++ {
		if p, ok := blocks[k].(*wml.CT_P); ok && p.PPr != nil && p.PPr.SectPr != nil {
			en.fail(locs[k], open, ErrSectionBreakInBlock, "move the section break outside "+open.text+" … {{/"+open.block+"}}")
			return nil
		}
	}
	body, bodyLocs := blocks[i+1:j], locs[i+1:j]
	if open.block != "each" {
		if !en.condition(open, sc) {
			return nil
		}
		return en.expandRange(body, bodyLocs, sc)
	}
	list, ok := en.list(open, sc, locs[i])
	if !ok {
		return nil
	}
	copies := copiesOf(en, body, len(list), open, locs[i])
	var out []wml.BlockElement
	for k, item := range list {
		out = append(out, en.expandRange(copies[k], bodyLocs, sc.push(item, itemName(sc, open.path, k)))...)
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
	t.Children = en.expandRows(t.Children, rlocs, sc)
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

// expandRows expands the row ranges among rows and fills the other rows.
func (en *engine) expandRows(rows []wml.TableRowElement, rlocs []Location, sc *scope) []wml.TableRowElement {
	var marks []mark
	opens := map[int]tag{}
	for k, r := range rows {
		row, ok := r.(*wml.CT_Tr)
		if !ok {
			continue
		}
		if t, ok := en.rowOpen(row); ok {
			marks = append(marks, mark{idx: k, t: t, loc: rlocs[k]})
			opens[k] = t
		}
		if t, ok := en.rowClose(row); ok {
			marks = append(marks, mark{idx: k, t: t, loc: rlocs[k]})
		}
	}
	ranges, ok := en.pair(marks, true)
	if !ok {
		return rows
	}
	out := make([]wml.TableRowElement, 0, len(rows))
	for k := 0; k < len(rows); k++ {
		if j, isOpen := ranges[k]; isOpen {
			out = append(out, en.expandRowRange(rows, rlocs, k, j, opens[k], sc)...)
			k = j
			continue
		}
		if row, ok := rows[k].(*wml.CT_Tr); ok {
			en.fillRow(row, sc, rlocs[k])
		}
		out = append(out, rows[k])
	}
	return out
}

// expandRowRange handles rows[k..j], whose edge cells hold open and its
// close tag.
func (en *engine) expandRowRange(rows []wml.TableRowElement, rlocs []Location, k, j int, open tag, sc *scope) []wml.TableRowElement {
	first, _ := rows[k].(*wml.CT_Tr)
	last, _ := rows[j].(*wml.CT_Tr)
	stripRowEdge(first, open, true)
	if closeTag, ok := en.rowClose(last); ok {
		stripRowEdge(last, closeTag, false)
	}
	body, bodyLocs := rows[k:j+1], rlocs[k:j+1]
	if open.block != "each" {
		if !en.condition(open, sc) {
			return nil
		}
		return en.expandRows(body, bodyLocs, sc)
	}
	list, ok := en.list(open, sc, rlocs[k])
	if !ok {
		return nil
	}
	copies := copiesOf(en, body, len(list), open, rlocs[k])
	var out []wml.TableRowElement
	for n, item := range list {
		out = append(out, en.expandRows(copies[n], bodyLocs, sc.push(item, itemName(sc, open.path, n)))...)
	}
	return out
}

// fillRow fills every cell of a row, keeping each cell schema-valid.
func (en *engine) fillRow(row *wml.CT_Tr, sc *scope, rloc Location) {
	nc := 0
	for _, c := range row.Children {
		cell, ok := c.(*wml.CT_Tc)
		if !ok {
			continue
		}
		cell.Children = en.expandBlocks(cell.Children, sc, rloc.child("tc", nc))
		// A cell must end with a paragraph; a removed range may have taken it.
		if n := len(cell.Children); n == 0 {
			cell.Children = append(cell.Children, emptyParagraph())
		} else if _, isP := cell.Children[n-1].(*wml.CT_P); !isP {
			cell.Children = append(cell.Children, emptyParagraph())
		}
		nc++
	}
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
	text := strings.TrimLeft(p.Text(), " \t")
	tags, _ := lexTags(text)
	if len(tags) == 0 || tags[0].start != 0 || tags[0].kind != tagOpen {
		return tag{}, false
	}
	if _, whole := marker(p); whole && en.pairedInCell(cell, 0) {
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
	text := strings.TrimRight(p.Text(), " \t")
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
	ranges, _ := en.pair(marks, false)
	for open, closeIdx := range ranges {
		if open == idx || closeIdx == idx {
			return true
		}
	}
	return false
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
// is removed when the cell has other content.
func stripRowEdge(row *wml.CT_Tr, t tag, open bool) {
	cell := edgeCell(row, open)
	if cell == nil || len(cell.Children) == 0 {
		return
	}
	idx := 0
	if !open {
		idx = len(cell.Children) - 1
	}
	p, ok := cell.Children[idx].(*wml.CT_P)
	if !ok {
		return
	}
	texts := paragraphTexts(p.Children)
	if open {
		for _, tx := range texts {
			if i := strings.Index(tx.Content, t.text); i >= 0 {
				tx.Content = tx.Content[:i] + tx.Content[i+len(t.text):]
				break
			}
		}
	} else {
		for n := len(texts) - 1; n >= 0; n-- {
			if i := strings.LastIndex(texts[n].Content, t.text); i >= 0 {
				texts[n].Content = texts[n].Content[:i] + texts[n].Content[i+len(t.text):]
				break
			}
		}
	}
	if len(cell.Children) > 1 && blankParagraph(p) {
		cell.Children = append(cell.Children[:idx:idx], cell.Children[idx+1:]...)
	}
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
func (en *engine) list(open tag, sc *scope, loc Location) ([]any, bool) {
	r := sc.lookup(open.path)
	if !r.found {
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
