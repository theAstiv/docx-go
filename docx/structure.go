package docx

import (
	"errors"
	"slices"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/parts"
)

// Errors returned by the Remove methods. Test with errors.Is.
var (
	// ErrNotInDocument reports an element that is no longer in the document,
	// because it (or its container) was already removed.
	ErrNotInDocument = errors.New("docx: element is not in the document")
	// ErrSectionBreak reports a paragraph that carries a section break
	// (Paragraph.SectionBreak is non-nil). Removing it would merge two
	// sections and change page layout; remove the content around it instead.
	ErrSectionBreak = errors.New("docx: paragraph carries a section break")
	// ErrUnbalancedField reports content holding only part of a complex
	// field (for example the paragraph where a table of contents begins).
	// Removing it would leave the rest of the field broken.
	ErrUnbalancedField = errors.New("docx: content holds part of a field")
	// ErrLastRow reports the only row of a table. A table needs at least one
	// row; remove the table instead.
	ErrLastRow = errors.New("docx: last row of the table")
)

// ── Removal ───────────────────────────────────────────────────────────────

// Remove deletes the paragraph from its container (the body, a table cell,
// a header or a footer).
//
// It returns ErrNotInDocument when the paragraph was already removed,
// ErrSectionBreak when it carries a section break, and ErrUnbalancedField
// when it holds only part of a complex field. A bookmark, comment,
// move or permission range that started or ended in the paragraph loses
// its other end too. A footnote, endnote or comment whose only reference
// was in the paragraph is deleted, and a deleted comment takes its range.
// A cell, header or footer left without a final paragraph gets an empty
// one, as Word requires.
func (p *Paragraph) Remove() error {
	if p.p.PPr != nil && p.p.PPr.SectPr != nil {
		if _, _, ok := p.st.locateBlock(p.p, p.hint()); ok {
			return ErrSectionBreak
		}
	}
	return p.st.removeBlock(p.p, p.hint())
}

// Remove deletes the table from its container, with the same rules as
// Paragraph.Remove (except section breaks, which a table cannot hold).
func (t *Table) Remove() error {
	return t.st.removeBlock(t.tbl, t.hint())
}

// removeBlock removes el from the block list holding it.
func (s *story) removeBlock(el wml.BlockElement, hint *[]wml.BlockElement) error {
	list, i, ok := s.locateBlock(el, hint)
	if !ok {
		return ErrNotInDocument
	}
	marks := wml.MarksIn(el)
	if !wml.FieldsBalanced(marks) {
		return ErrUnbalancedField
	}
	*list = slices.Delete(*list, i, i+1)
	if !s.isBody(list) {
		ensureTrailingParagraph(list)
	}
	s.detach()
	s.afterRemoval(marks)
	return nil
}

// afterRemoval deletes the anchors the removed content leaves without a
// partner and the notes and comments it leaves without a reference, and
// drops the id cache: Document.Bookmarks must not list a removed bookmark.
func (s *story) afterRemoval(marks []wml.Mark) {
	if orphans := wml.OrphanedPartners(marks); len(orphans) > 0 {
		for _, root := range s.roots() {
			wml.RemoveAnchors(root, orphans)
		}
	}
	if s != nil && s.pkg != nil {
		s.invalidateIDs()
		s.removeNotes(wml.Refs(marks))
	}
}

// noteKinds maps a reference kind to the part its note is in.
var noteKinds = map[wml.MarkKind]parts.NoteKind{
	wml.MarkFootnoteRef: parts.Footnotes,
	wml.MarkEndnoteRef:  parts.Endnotes,
	wml.MarkCommentRef:  parts.Comments,
}

// removeNotes deletes the footnotes, endnotes and comments whose references
// were removed (refs, by reference kind) and that nothing else in the
// document still references. A removed comment also takes its range
// anchors, which would otherwise mark a range for a comment that is gone.
func (s *story) removeNotes(refs map[wml.MarkKind]map[int64]bool) {
	if len(refs) == 0 {
		return
	}
	sc := s.freshScan()
	for kind, ids := range refs {
		gone := map[int64]bool{}
		for id := range ids {
			if !sc.Referenced(kind, id) {
				gone[id] = true
			}
		}
		if len(gone) == 0 {
			continue
		}
		s.pkg.RemoveNotes(noteKinds[kind], gone)
		if kind == wml.MarkCommentRef {
			anchors := map[wml.MarkKind]map[int64]bool{wml.MarkCommentStart: gone, wml.MarkCommentEnd: gone}
			for _, root := range s.roots() {
				wml.RemoveAnchors(root, anchors)
			}
		}
	}
	// The removed notes may have held bookmarks and drawings.
	s.invalidateIDs()
}

// ensureTrailingParagraph appends an empty paragraph when list is empty or
// ends with something else: a cell, header or footer must end with w:p.
func ensureTrailingParagraph(list *[]wml.BlockElement) {
	if n := len(*list); n > 0 {
		if _, ok := (*list)[n-1].(*wml.CT_P); ok {
			return
		}
	}
	*list = append(*list, new(wml.CT_P))
}

// Remove deletes the run from its paragraph or hyperlink. It returns
// ErrNotInDocument when the run was already removed and ErrUnbalancedField
// when the run holds part of a complex field (a field character without
// its partners).
func (r *Run) Remove() error {
	// Removing a run detaches no list a hint can name, so it does not bump
	// the structure generation.
	list, i, ok := r.st.locateInline(r.r, r.hint())
	if !ok {
		return ErrNotInDocument
	}
	marks := wml.MarksIn(r.r)
	if !wml.FieldsBalanced(marks) {
		return ErrUnbalancedField
	}
	*list = slices.Delete(*list, i, i+1)
	r.st.afterRemoval(marks)
	return nil
}

// Remove deletes the row from its table. It returns ErrNotInDocument when
// the row was already removed, ErrLastRow when it is the table's only row,
// and ErrUnbalancedField when it holds part of a complex field.
//
// A cell that starts a vertical merge passes the start to the cell below
// it, so the rest of the merged block survives.
func (r *Row) Remove() error {
	if r.tbl == nil {
		return ErrNotInDocument
	}
	// The row is in r.tbl, but r.tbl may have been removed since the proxy
	// was made (Table.Remove, or the row of an enclosing table).
	if r.gen != r.st.stamp() {
		if _, _, ok := r.st.locateBlock(r.tbl, nil); !ok {
			return ErrNotInDocument
		}
	}
	idx := slices.IndexFunc(r.tbl.Children, func(e wml.TableRowElement) bool { return e == r.tr })
	if idx < 0 {
		return ErrNotInDocument
	}
	if len(typedRows(r.tbl)) == 1 {
		return ErrLastRow
	}
	marks := wml.MarksIn(r.tr)
	if !wml.FieldsBalanced(marks) {
		return ErrUnbalancedField
	}
	if next := nextTypedRow(r.tbl, idx); next != nil {
		passMergeStart(r.tr, next)
	}
	r.tbl.Children = slices.Delete(r.tbl.Children, idx, idx+1)
	r.st.detach()
	r.st.afterRemoval(marks)
	return nil
}

// typedRows returns the positions of the w:tr children of t.
func typedRows(t *wml.CT_Tbl) []int {
	var out []int
	for i, c := range t.Children {
		if _, ok := c.(*wml.CT_Tr); ok {
			out = append(out, i)
		}
	}
	return out
}

func nextTypedRow(t *wml.CT_Tbl, after int) *wml.CT_Tr {
	for _, c := range t.Children[after+1:] {
		if tr, ok := c.(*wml.CT_Tr); ok {
			return tr
		}
	}
	return nil
}

// gridCells maps each typed cell of a row to the grid column it starts in.
func gridCells(tr *wml.CT_Tr) map[int]*wml.CT_Tc {
	col := 0
	if tr.TrPr != nil {
		col = gridCount(tr.TrPr.GridBefore(), 0)
	}
	out := map[int]*wml.CT_Tc{}
	for _, c := range tr.Children {
		span := 1
		if tc, ok := c.(*wml.CT_Tc); ok {
			out[col] = tc
			if tc.TcPr != nil {
				span = gridCount(tc.TcPr.GridSpan(), 1)
			}
		}
		col += span
	}
	return out
}

// gridCount converts a stored column count (w:gridBefore, w:gridSpan) to
// an int, returning def when it is absent, below def, or outside the int32
// range (a value no real table has).
func gridCount(n *int64, def int) int {
	if n == nil {
		return def
	}
	v, ok := toInt32(*n)
	if !ok || int(v) < def {
		return def
	}
	return int(v)
}

func vmergeState(tc *wml.CT_Tc) VMergeState {
	return (&Cell{tc: tc, st: nil}).VMerge()
}

// passMergeStart makes each cell of next that continues a vertical merge
// started in removed the new start of that merge.
func passMergeStart(removed, next *wml.CT_Tr) {
	below := gridCells(next)
	for col, tc := range gridCells(removed) {
		if vmergeState(tc) != VMergeStart {
			continue
		}
		if nb := below[col]; nb != nil && vmergeState(nb) == VMergeContinue {
			nb.TcPr.SetVMerge(wml.VMergeRestart)
		}
	}
}

// ── Insertion ─────────────────────────────────────────────────────────────

// InsertParagraphBefore inserts a new paragraph holding text (no run when
// text is "") just before p, in the same container, and returns it. The new
// paragraph has no formatting of its own; copy p's style with
// Format().SetStyle if wanted. If p is no longer in the document, the new
// paragraph is not attached either.
func (p *Paragraph) InsertParagraphBefore(text string) *Paragraph {
	return insertParagraph(p.st, p.p, p.hint(), 0, text)
}

// InsertParagraphAfter inserts a new paragraph holding text just after p,
// like InsertParagraphBefore. Inserted after a paragraph that carries a
// section break, the new paragraph starts the next section.
func (p *Paragraph) InsertParagraphAfter(text string) *Paragraph {
	return insertParagraph(p.st, p.p, p.hint(), 1, text)
}

// InsertParagraphBefore inserts a new paragraph holding text just before
// the table, like Paragraph.InsertParagraphBefore.
func (t *Table) InsertParagraphBefore(text string) *Paragraph {
	return insertParagraph(t.st, t.tbl, t.hint(), 0, text)
}

// InsertParagraphAfter inserts a new paragraph holding text just after the
// table, like Paragraph.InsertParagraphAfter.
func (t *Table) InsertParagraphAfter(text string) *Paragraph {
	return insertParagraph(t.st, t.tbl, t.hint(), 1, text)
}

func insertParagraph(st *story, el wml.BlockElement, hint *[]wml.BlockElement, offset int, text string) *Paragraph {
	np := new(wml.CT_P)
	para := &Paragraph{p: np, st: st, parent: nil}
	if text != "" {
		para.AddRun(text)
	}
	list, i, ok := st.locateBlock(el, hint)
	if !ok {
		return para
	}
	*list = slices.Insert(*list, i+offset, wml.BlockElement(np))
	para.parent, para.gen = list, st.stamp()
	return para
}

// InsertTableAfter inserts a new table with the given dimensions just after
// p and returns it, like Body.AddTable. In a cell, header or footer, an
// empty paragraph follows the table when it would otherwise end the
// container. If p is no longer in the document, the table is not attached.
func (p *Paragraph) InsertTableAfter(rows, cols int, opts TableOptions) *Table {
	list, i, ok := p.st.locateBlock(p.p, p.hint())
	tbl := buildTable(max(rows, 1), max(cols, 1), opts, p.st.textWidthAt(list, i+1))
	t := &Table{tbl: tbl, st: p.st, parent: nil}
	if !ok {
		return t
	}
	*list = slices.Insert(*list, i+1, wml.BlockElement(tbl))
	if !p.st.isBody(list) {
		ensureTrailingParagraph(list)
	}
	t.parent, t.gen = list, p.st.stamp()
	return t
}

// InsertRow inserts a new row so that it becomes row i (0-based; i is
// clamped to 0…number of rows) and returns it.
//
// The row copies the structure of its neighbour (the row now at i, or the
// last row when appending): the same cells with their spans, widths,
// borders and shading, and the first paragraph's formatting, each cell
// holding one empty paragraph. Row properties such as height are copied
// too; opts.Header sets or clears the repeating-header mark. A cell joins a
// vertical merge when the cell below it continues one, so inserting inside
// a merged block extends it.
func (t *Table) InsertRow(i int, opts RowOptions) *Row {
	rows := typedRows(t.tbl)
	i = max(0, min(i, len(rows)))
	var ref, below *wml.CT_Tr
	pos := len(t.tbl.Children)
	if i < len(rows) {
		pos = rows[i]
		below, _ = t.tbl.Children[pos].(*wml.CT_Tr)
		ref = below
	} else if len(rows) > 0 {
		pos = rows[len(rows)-1] + 1
		ref, _ = t.tbl.Children[rows[len(rows)-1]].(*wml.CT_Tr)
	}
	tr := newRowLike(ref, below, t.tbl, opts)
	t.tbl.Children = slices.Insert(t.tbl.Children, pos, wml.TableRowElement(tr))
	return &Row{tr: tr, st: t.st, tbl: t.tbl, gen: t.childGen()}
}

// newRowLike builds an empty row shaped like ref (or, without ref, one cell
// per grid column of tbl).
func newRowLike(ref, below *wml.CT_Tr, tbl *wml.CT_Tbl, opts RowOptions) *wml.CT_Tr {
	tr := new(wml.CT_Tr)
	if ref != nil && ref.TrPr != nil {
		if c, err := wml.Clone(ref.TrPr); err == nil {
			tr.TrPr = c
		}
	}
	if opts.Header {
		if tr.TrPr == nil {
			tr.TrPr = new(wml.CT_TrPr)
		}
		tr.TrPr.SetHeader(On())
	} else if tr.TrPr != nil {
		tr.TrPr.SetHeader(nil)
	}

	if ref == nil {
		n := 1
		if tbl.TblGrid != nil && len(tbl.TblGrid.Cols) > 0 {
			n = len(tbl.TblGrid.Cols)
		}
		for range n {
			tc := new(wml.CT_Tc)
			tc.Children = []wml.BlockElement{new(wml.CT_P)}
			tr.Children = append(tr.Children, tc)
		}
		return tr
	}

	var belowCells map[int]*wml.CT_Tc
	if below != nil {
		belowCells = gridCells(below)
	}
	refCells := gridCells(ref)
	cols := make([]int, 0, len(refCells))
	for col := range refCells {
		cols = append(cols, col)
	}
	slices.Sort(cols)
	for _, col := range cols {
		tr.Children = append(tr.Children, newCellLike(refCells[col], belowCells[col]))
	}
	return tr
}

// newCellLike builds an empty cell with ref's properties (without its
// vertical-merge mark) and its first paragraph's formatting.
func newCellLike(ref, below *wml.CT_Tc) *wml.CT_Tc {
	tc := new(wml.CT_Tc)
	if ref.TcPr != nil {
		if c, err := wml.Clone(ref.TcPr); err == nil {
			c.ClearVMerge()
			tc.TcPr = c
		}
	}
	if below != nil && vmergeState(below) == VMergeContinue {
		if tc.TcPr == nil {
			tc.TcPr = new(wml.CT_TcPr)
		}
		tc.TcPr.SetVMerge("")
	}
	p := new(wml.CT_P)
	for _, b := range ref.Children {
		if first, ok := b.(*wml.CT_P); ok {
			if first.PPr != nil {
				if c, err := wml.Clone(first.PPr); err == nil {
					c.SectPr = nil
					p.PPr = c
				}
			}
			break
		}
	}
	tc.Children = []wml.BlockElement{p}
	return tc
}
