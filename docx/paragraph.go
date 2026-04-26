package docx

import (
	"iter"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Paragraph is a proxy for the w:p element.
type Paragraph struct {
	p *wml.CT_P
}

// Format returns a ParagraphFormat proxy for reading and writing paragraph
// properties. Each call returns a new proxy backed by the same CT_P.
// Property elements (CT_PPr) are created lazily only when a setter is called.
func (p *Paragraph) Format() *ParagraphFormat {
	return &ParagraphFormat{p: p.p}
}

// Runs returns an iterator over the runs (w:r elements) in this paragraph.
// Hyperlinks and other inline containers are not visited; use for simple text access.
func (p *Paragraph) Runs() iter.Seq[*Run] {
	return func(yield func(*Run) bool) {
		for _, child := range p.p.Children {
			r, ok := child.(*wml.CT_R)
			if !ok {
				continue
			}
			if !yield(&Run{r: r}) {
				return
			}
		}
	}
}

// AddRun appends a new run containing text and returns its proxy.
// If text is non-empty, a w:t element with xml:space="preserve" is added.
func (p *Paragraph) AddRun(text string) *Run {
	r := new(wml.CT_R)
	if text != "" {
		t := new(wml.CT_Text)
		t.Content = text
		t.Space = "preserve"
		r.Children = append(r.Children, t)
	}
	p.p.Children = append(p.p.Children, r)
	return &Run{r: r}
}

// Text returns the visible text of the paragraph by walking all w:t descendants
// in document order.
func (p *Paragraph) Text() string {
	return p.p.Text()
}

// ---------------------------------------------------------------------------
// ParagraphFormat
// ---------------------------------------------------------------------------

// ParagraphFormat is a proxy for w:pPr, providing typed access to paragraph
// formatting properties. Mutations write through to the underlying CT_PPr.
type ParagraphFormat struct {
	p *wml.CT_P
}

func (f *ParagraphFormat) ppr() *wml.CT_PPr {
	if f.p.PPr == nil {
		f.p.PPr = new(wml.CT_PPr)
	}
	return f.p.PPr
}

// twipPtr converts *Twip to *int64 for the oxml layer. nil in → nil out.
func twipPtr(t *Twip) *int64 {
	if t == nil {
		return nil
	}
	v := int64(*t)
	return &v
}

// twipFromInt64 converts *int64 from the oxml layer to *Twip. nil in → nil out.
func twipFromInt64(v *int64) *Twip {
	if v == nil {
		return nil
	}
	t := Twip(*v)
	return &t
}

// Style returns the paragraph style ID, or "".
func (f *ParagraphFormat) Style() string {
	if f.p.PPr == nil {
		return ""
	}
	return f.p.PPr.StyleID()
}

// SetStyle sets the paragraph style ID. An empty string removes it.
func (f *ParagraphFormat) SetStyle(s string) { f.ppr().SetStyleID(s) }

// Alignment returns the paragraph alignment, or "".
func (f *ParagraphFormat) Alignment() AlignmentValue {
	if f.p.PPr == nil {
		return ""
	}
	return AlignmentValue(f.p.PPr.Alignment())
}

// SetAlignment sets the paragraph alignment.
func (f *ParagraphFormat) SetAlignment(a AlignmentValue) { f.ppr().SetAlignment(string(a)) }

// OutlineLevel returns the outline level (0-8), or nil if not set.
func (f *ParagraphFormat) OutlineLevel() *int64 {
	if f.p.PPr == nil {
		return nil
	}
	return f.p.PPr.OutlineLevel()
}

// SetOutlineLevel sets the outline level. nil removes it.
func (f *ParagraphFormat) SetOutlineLevel(lvl *int64) { f.ppr().SetOutlineLevel(lvl) }

// SpaceBefore returns spacing before in twips, or nil.
func (f *ParagraphFormat) SpaceBefore() *Twip {
	if f.p.PPr == nil {
		return nil
	}
	return twipFromInt64(f.p.PPr.SpacingBefore())
}

// SetSpaceBefore sets spacing before in twips. nil removes it.
func (f *ParagraphFormat) SetSpaceBefore(t *Twip) { f.ppr().SetSpacingBefore(twipPtr(t)) }

// SpaceAfter returns spacing after in twips, or nil.
func (f *ParagraphFormat) SpaceAfter() *Twip {
	if f.p.PPr == nil {
		return nil
	}
	return twipFromInt64(f.p.PPr.SpacingAfter())
}

// SetSpaceAfter sets spacing after in twips. nil removes it.
func (f *ParagraphFormat) SetSpaceAfter(t *Twip) { f.ppr().SetSpacingAfter(twipPtr(t)) }

// LeftIndent returns the left (start) indent in twips, or nil.
func (f *ParagraphFormat) LeftIndent() *Twip {
	if f.p.PPr == nil {
		return nil
	}
	return twipFromInt64(f.p.PPr.IndStart())
}

// SetLeftIndent sets the left (start) indent in twips. nil removes it.
func (f *ParagraphFormat) SetLeftIndent(t *Twip) { f.ppr().SetIndStart(twipPtr(t)) }

// RightIndent returns the right (end) indent in twips, or nil.
func (f *ParagraphFormat) RightIndent() *Twip {
	if f.p.PPr == nil {
		return nil
	}
	return twipFromInt64(f.p.PPr.IndEnd())
}

// SetRightIndent sets the right (end) indent in twips. nil removes it.
func (f *ParagraphFormat) SetRightIndent(t *Twip) { f.ppr().SetIndEnd(twipPtr(t)) }

// FirstLineIndent returns the first-line indent in twips, or nil.
func (f *ParagraphFormat) FirstLineIndent() *Twip {
	if f.p.PPr == nil {
		return nil
	}
	return twipFromInt64(f.p.PPr.IndFirstLine())
}

// SetFirstLineIndent sets the first-line indent in twips. nil removes it.
func (f *ParagraphFormat) SetFirstLineIndent(t *Twip) { f.ppr().SetIndFirstLine(twipPtr(t)) }

// HangingIndent returns the hanging indent in twips, or nil.
func (f *ParagraphFormat) HangingIndent() *Twip {
	if f.p.PPr == nil {
		return nil
	}
	return twipFromInt64(f.p.PPr.IndHanging())
}

// SetHangingIndent sets the hanging indent in twips. nil removes it.
func (f *ParagraphFormat) SetHangingIndent(t *Twip) { f.ppr().SetIndHanging(twipPtr(t)) }

// KeepNext returns the KeepNext tri-state, or nil (inherit).
func (f *ParagraphFormat) KeepNext() *bool {
	if f.p.PPr == nil {
		return nil
	}
	return f.p.PPr.KeepNext()
}

// SetKeepNext sets KeepNext. nil removes the element (inherit from style).
func (f *ParagraphFormat) SetKeepNext(b *bool) { f.ppr().SetKeepNext(b) }

// KeepLines returns the KeepLines tri-state, or nil.
func (f *ParagraphFormat) KeepLines() *bool {
	if f.p.PPr == nil {
		return nil
	}
	return f.p.PPr.KeepLines()
}

// SetKeepLines sets KeepLines.
func (f *ParagraphFormat) SetKeepLines(b *bool) { f.ppr().SetKeepLines(b) }

// PageBreakBefore returns the PageBreakBefore tri-state, or nil.
func (f *ParagraphFormat) PageBreakBefore() *bool {
	if f.p.PPr == nil {
		return nil
	}
	return f.p.PPr.PageBreakBefore()
}

// SetPageBreakBefore sets PageBreakBefore.
func (f *ParagraphFormat) SetPageBreakBefore(b *bool) { f.ppr().SetPageBreakBefore(b) }

// ContextualSpacing returns the ContextualSpacing tri-state, or nil.
func (f *ParagraphFormat) ContextualSpacing() *bool {
	if f.p.PPr == nil {
		return nil
	}
	return f.p.PPr.ContextualSpacing()
}

// SetContextualSpacing sets ContextualSpacing.
func (f *ParagraphFormat) SetContextualSpacing(b *bool) { f.ppr().SetContextualSpacing(b) }
