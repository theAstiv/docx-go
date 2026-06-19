package docx_test

import (
	"bytes"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestCreateList_Basic(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{})
	if list == nil {
		t.Fatal("CreateList() returned nil")
	}
	if list.NumID() < 1 {
		t.Errorf("NumID() = %d, want ≥ 1", list.NumID())
	}
}

func TestParagraph_SetList(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{})

	p := d.Body().AddParagraph()
	p.AddRun("item one")
	p.SetList(list, 0)

	// Round-trip: save + reopen.
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	var found bool
	for p2 := range d2.Body().Paragraphs() {
		if p2.Text() == "item one" {
			found = true
			pf := p2.Format()
			if got := pf.NumID(); got == nil || *got != list.NumID() {
				t.Errorf("after round-trip NumID() = %v, want %d", got, list.NumID())
			}
			if got := pf.NumILvl(); got == nil || *got != 0 {
				t.Errorf("after round-trip NumILvl() = %v, want 0", got)
			}
			break
		}
	}
	if !found {
		t.Error("paragraph 'item one' not found after round-trip")
	}
}

func TestParagraph_SetList_MultiLevel(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{Style: docx.ListDecimal})

	texts := []string{"top", "sub", "subsub"}
	levels := []int{0, 1, 2}
	for i, text := range texts {
		p := d.Body().AddParagraph()
		p.AddRun(text)
		p.SetList(list, levels[i])
	}

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	i := 0
	for p2 := range d2.Body().Paragraphs() {
		if i >= len(texts) {
			break
		}
		if got := p2.Text(); got != texts[i] {
			t.Errorf("para[%d].Text() = %q, want %q", i, got, texts[i])
		}
		if got := p2.Format().NumILvl(); got == nil || int(*got) != levels[i] {
			t.Errorf("para[%d].NumILvl() = %v, want %d", i, got, levels[i])
		}
		i++
	}
}

func TestParagraph_ClearList(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{})
	p := d.Body().AddParagraph()
	p.AddRun("item")
	p.SetList(list, 0)
	p.ClearList()

	if got := p.Format().NumID(); got != nil {
		t.Errorf("NumID() after ClearList() = %v, want nil", got)
	}
}

func TestList_RestartAt(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{Style: docx.ListDecimal})

	p1 := d.Body().AddParagraph()
	p1.AddRun("first list item 1")
	p1.SetList(list, 0)

	p2 := d.Body().AddParagraph()
	p2.AddRun("first list item 2")
	p2.SetList(list, 0)

	// Restart at 1.
	list2 := list.RestartAt(0, 1)
	if list2.NumID() == list.NumID() {
		t.Error("RestartAt() should return a new List with a different numId")
	}

	p3 := d.Body().AddParagraph()
	p3.AddRun("second list item 1")
	p3.SetList(list2, 0)

	// Verify round-trip preserves both numIds.
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	paras := make([]*docx.Paragraph, 0, 3)
	for p := range d2.Body().Paragraphs() {
		paras = append(paras, p)
	}
	if len(paras) < 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(paras))
	}
	// p1 and p3 should have different numIds.
	id1 := paras[0].Format().NumID()
	id3 := paras[2].Format().NumID()
	if id1 == nil || id3 == nil {
		t.Fatal("numId not found on paragraphs after round-trip")
	}
	if *id1 == *id3 {
		t.Errorf("p1.NumID() = p3.NumID() = %d, expected different IDs for restart", *id1)
	}
}

func TestCreateList_BulletStyle(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{Style: docx.ListBullet})

	p := d.Body().AddParagraph()
	p.AddRun("bullet item")
	p.SetList(list, 0)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	// Verify it opens without error.
	if _, err := docx.OpenBytes(buf.Bytes()); err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
}

func TestList_FixtureRoundTrip_Numbering(t *testing.T) {
	// Open an existing fixture that contains numbering and verify round-trip.
	d := docx.New()
	list := d.CreateList(docx.ListOptions{Style: docx.ListDecimal})

	for range 3 {
		p := d.Body().AddParagraph()
		p.AddRun("item")
		p.SetList(list, 0)
	}
	// Middle plain paragraph (not a list item).
	plain := d.Body().AddParagraph()
	plain.AddRun("plain")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	var numItems, nonItems int
	for p := range d2.Body().Paragraphs() {
		if p.Format().NumID() != nil {
			numItems++
		} else {
			nonItems++
		}
	}
	if numItems != 3 {
		t.Errorf("numbered items after round-trip = %d, want 3", numItems)
	}
	if nonItems != 1 {
		t.Errorf("non-list paragraphs after round-trip = %d, want 1", nonItems)
	}
}
