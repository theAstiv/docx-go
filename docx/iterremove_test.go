package docx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// Removing what an iterator yields, inside the loop, must visit and remove
// every element (review 2026-11-19, P0 #10). The iterators ranged over the
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
// inserts attached to the detached cell (review 2026-11-19, P1).
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

func countSeq[T any](seq func(func(T) bool)) int {
	n := 0
	for range seq {
		n++
	}
	return n
}
