package docx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// Removing what an iterator yields, inside the loop, must visit and remove
// every element. The iterators used to range over the
// live slice, so each removal shifted the next element into the slot just
// visited and every other one survived. Earlier tests always collected the
// iterator first.
func TestIterators_RemoveWhileIterating(t *testing.T) {
	t.Run("body paragraphs", func(t *testing.T) {
		d := docx.New()
		for i := range 6 {
			d.Body().AddParagraph().AddRun(fmt.Sprint(i))
		}
		for p := range d.Body().Paragraphs() {
			if err := p.Remove(); err != nil {
				t.Fatal(err)
			}
		}
		if n := countSeq(d.Body().Paragraphs()); n != 0 {
			t.Errorf("%d paragraphs left, want 0", n)
		}
	})
	t.Run("body tables", func(t *testing.T) {
		d := docx.New()
		for range 4 {
			d.Body().AddTable(1, 1, docx.TableOptions{})
		}
		for tb := range d.Body().Tables() {
			if err := tb.Remove(); err != nil {
				t.Fatal(err)
			}
		}
		if n := countSeq(d.Body().Tables()); n != 0 {
			t.Errorf("%d tables left, want 0", n)
		}
	})
	t.Run("runs", func(t *testing.T) {
		d := docx.New()
		p := d.Body().AddParagraph()
		for i := range 5 {
			p.AddRun(fmt.Sprint(i))
		}
		for r := range p.Runs() {
			if err := r.Remove(); err != nil {
				t.Fatal(err)
			}
		}
		if p.Text() != "" {
			t.Errorf("text left: %q", p.Text())
		}
	})
	t.Run("rows", func(t *testing.T) {
		d := docx.New()
		tb := d.Body().AddTable(5, 1, docx.TableOptions{})
		for r := range tb.Rows() {
			// The last row cannot go; every other must.
			if err := r.Remove(); err != nil && !errors.Is(err, docx.ErrLastRow) {
				t.Fatal(err)
			}
		}
		if n := countSeq(tb.Rows()); n != 1 {
			t.Errorf("%d rows left, want 1", n)
		}
	})
	t.Run("header paragraphs", func(t *testing.T) {
		d := docx.New()
		h := d.LastSection().SetHeader(docx.HeaderDefault)
		for i := range 4 {
			h.AddParagraph().AddRun(fmt.Sprint(i))
		}
		for p := range h.Paragraphs() {
			if err := p.Remove(); err != nil {
				t.Fatal(err)
			}
		}
		// A header always keeps one (empty) paragraph; Header.Text ends every
		// paragraph with "\n".
		if n, txt := countSeq(h.Paragraphs()), h.Text(); n != 1 || txt != "\n" {
			t.Errorf("%d paragraphs with text %q left, want one empty", n, txt)
		}
	})
	t.Run("cell paragraphs", func(t *testing.T) {
		d := docx.New()
		c := firstCell(d.Body().AddTable(1, 1, docx.TableOptions{}))
		for i := range 4 {
			c.AddParagraph().AddRun(fmt.Sprint(i))
		}
		for p := range c.Paragraphs() {
			if err := p.Remove(); err != nil {
				t.Fatal(err)
			}
		}
		if n, txt := countSeq(c.Paragraphs()), c.Text(); n != 1 || txt != "" {
			t.Errorf("%d paragraphs with text %q left, want one empty", n, txt)
		}
	})
}

// Proxies inside a removed table found themselves in their (now detached)
// cell and acted on it: Remove returned nil instead of ErrNotInDocument and
// inserts attached to the detached cell.
func TestStaleProxy_AfterTableRemove(t *testing.T) {
	d := docx.New()
	tb := d.Body().AddTable(2, 1, docx.TableOptions{})
	cell := firstCell(tb)
	para := cell.AddParagraph()
	para.AddRun("inner")
	var rows []*docx.Row
	for r := range tb.Rows() {
		rows = append(rows, r)
	}
	if err := tb.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := para.Remove(); !errors.Is(err, docx.ErrNotInDocument) {
		t.Errorf("cell paragraph Remove after Table.Remove = %v, want ErrNotInDocument", err)
	}
	if err := rows[0].Remove(); !errors.Is(err, docx.ErrNotInDocument) {
		t.Errorf("row Remove after Table.Remove = %v, want ErrNotInDocument", err)
	}
	para.InsertParagraphAfter("ghost")
	if got := d.Text(); got != "" {
		t.Errorf("Text() = %q, want nothing from the removed table", got)
	}
}

// A proxy held across a removal still works when its element is still in
// the document: a stale hint falls back to the walk, it is not a refusal.
func TestStaleProxy_StillAttached(t *testing.T) {
	d := docx.New()
	a := d.Body().AddParagraph()
	a.AddRun("a")
	b := d.Body().AddParagraph()
	b.AddRun("b")
	if err := a.Remove(); err != nil {
		t.Fatal(err)
	}
	b.InsertParagraphBefore("before")
	if err := b.Remove(); err != nil {
		t.Fatalf("Remove of a still-attached paragraph: %v", err)
	}
	if got := d.Text(); got != "before\n" {
		t.Errorf("Text() = %q, want %q", got, "before\n")
	}
}

// Proxies made from a removed parent after the removal (review H4) took the
// story's current generation, so their hints looked current: Remove returned
// nil and inserts edited the detached tree. Each case makes the child only
// after its parent is gone.
func TestStaleProxy_ChildOfRemovedParent(t *testing.T) {
	notIn := func(t *testing.T, what string, err error) {
		t.Helper()
		if !errors.Is(err, docx.ErrNotInDocument) {
			t.Errorf("%s: Remove = %v, want ErrNotInDocument", what, err)
		}
	}
	removedTable := func(t *testing.T) (*docx.Document, *docx.Table) {
		t.Helper()
		d := docx.New()
		d.Body().AddParagraph().AddRun("keep")
		tb := d.Body().AddTable(3, 1, docx.TableOptions{})
		firstCell(tb).SetText("cell")
		if err := tb.Remove(); err != nil {
			t.Fatal(err)
		}
		return d, tb
	}
	t.Run("rows", func(t *testing.T) {
		_, tb := removedTable(t)
		for row := range tb.Rows() {
			notIn(t, "row", row.Remove())
			break
		}
		notIn(t, "inserted row", tb.InsertRow(0, docx.RowOptions{}).Remove())
		notIn(t, "added row", tb.AddRow().Remove())
	})
	t.Run("cell paragraphs", func(t *testing.T) {
		d, tb := removedTable(t)
		c := firstCell(tb)
		for p := range c.Paragraphs() {
			notIn(t, "cell paragraph", p.Remove())
			p.InsertParagraphAfter("ghost")
		}
		notIn(t, "added cell paragraph", c.AddParagraph().Remove())
		notIn(t, "SetText paragraph", c.SetText("x").Remove())
		if got := d.Text(); got != "keep\n" {
			t.Errorf("Text() = %q, want %q", got, "keep\n")
		}
	})
	t.Run("nested table", func(t *testing.T) {
		d := docx.New()
		outer := d.Body().AddTable(1, 1, docx.TableOptions{})
		firstCell(outer).AddParagraph().InsertTableAfter(1, 1, docx.TableOptions{})
		c := firstCell(outer)
		if err := outer.Remove(); err != nil {
			t.Fatal(err)
		}
		for inner := range c.Tables() {
			notIn(t, "nested table", inner.Remove())
			for row := range inner.Rows() {
				notIn(t, "nested row", row.Remove())
			}
		}
	})
	t.Run("row cells", func(t *testing.T) {
		d := docx.New()
		tb := d.Body().AddTable(2, 1, docx.TableOptions{})
		var row *docx.Row
		for r := range tb.Rows() {
			row = r
			break
		}
		if err := row.Remove(); err != nil {
			t.Fatal(err)
		}
		for c := range row.Cells() {
			for p := range c.Paragraphs() {
				notIn(t, "paragraph of removed row", p.Remove())
			}
		}
		notIn(t, "paragraph of added cell", row.AddCell().AddParagraph().Remove())
	})
	t.Run("runs", func(t *testing.T) {
		d := docx.New()
		p := d.Body().AddParagraph()
		p.AddRun("a")
		d.Body().AddParagraph().AddRun("keep")
		if err := p.Remove(); err != nil {
			t.Fatal(err)
		}
		for r := range p.Runs() {
			notIn(t, "run", r.Remove())
		}
		notIn(t, "added run", p.AddRun("b").Remove())
	})
	t.Run("hyperlink runs", func(t *testing.T) {
		d := docx.New()
		p := d.Body().AddParagraph()
		link, err := p.AddInternalLink("x", docx.HyperlinkOptions{})
		if err != nil {
			t.Fatal(err)
		}
		link.AddRun("a")
		d.Body().AddParagraph().AddRun("keep")
		if err := p.Remove(); err != nil {
			t.Fatal(err)
		}
		notIn(t, "run of held link", link.AddRun("b").Remove())
		for h := range p.Hyperlinks() {
			for r := range h.Runs() {
				notIn(t, "link run", r.Remove())
			}
		}
	})
}

// Children of a parent that is still in the document keep working after an
// unrelated removal made the parent's gen stale.
func TestStaleProxy_ChildOfAttachedParent(t *testing.T) {
	d := docx.New()
	gone := d.Body().AddParagraph()
	tb := d.Body().AddTable(3, 1, docx.TableOptions{})
	p := d.Body().AddParagraph()
	p.AddRun("a")
	link, err := p.AddInternalLink("x", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	link.AddRun("b")
	if err := gone.Remove(); err != nil {
		t.Fatal(err)
	}
	for row := range tb.Rows() {
		if err := row.Remove(); err != nil && !errors.Is(err, docx.ErrLastRow) {
			t.Fatalf("row Remove: %v", err)
		}
	}
	if n := countSeq(tb.Rows()); n != 1 {
		t.Errorf("%d rows left, want 1", n)
	}
	cp := firstCell(tb).AddParagraph()
	if err := cp.Remove(); err != nil {
		t.Errorf("cell paragraph Remove: %v", err)
	}
	for r := range p.Runs() {
		if err := r.Remove(); err != nil {
			t.Errorf("run Remove: %v", err)
		}
	}
	for r := range link.Runs() {
		if err := r.Remove(); err != nil {
			t.Errorf("link run Remove: %v", err)
		}
	}
	if got := p.Text(); got != "" {
		t.Errorf("paragraph text %q, want empty", got)
	}
}

func countSeq[T any](seq func(func(T) bool)) int {
	n := 0
	for range seq {
		n++
	}
	return n
}
