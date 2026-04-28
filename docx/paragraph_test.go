package docx_test

import (
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// ---------------------------------------------------------------------------
// LineSpacing — typed setters / getters
// ---------------------------------------------------------------------------

func TestParagraphFormat_LineSpacing_Auto(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	f.SetLineSpacingAuto(docx.OneAndAHalfLine()) // 360 = 1.5×

	got := f.LineSpacingAuto()
	if got == nil || *got != docx.OneAndAHalfLine() {
		t.Errorf("LineSpacingAuto() = %v, want 360", got)
	}
	if f.LineSpacingExact() != nil {
		t.Error("LineSpacingExact() should be nil when rule is auto")
	}
	if f.LineSpacingAtLeast() != nil {
		t.Error("LineSpacingAtLeast() should be nil when rule is auto")
	}
}

func TestParagraphFormat_LineSpacing_Exact(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	f.SetLineSpacingExact(docx.Pt(12)) // 240 twips

	got := f.LineSpacingExact()
	if got == nil || *got != docx.Pt(12) {
		t.Errorf("LineSpacingExact() = %v, want %d", got, docx.Pt(12))
	}
	if f.LineSpacingAuto() != nil {
		t.Error("LineSpacingAuto() should be nil when rule is exact")
	}
}

func TestParagraphFormat_LineSpacing_AtLeast(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	f.SetLineSpacingAtLeast(docx.Pt(10))

	got := f.LineSpacingAtLeast()
	if got == nil || *got != docx.Pt(10) {
		t.Errorf("LineSpacingAtLeast() = %v, want %d", got, docx.Pt(10))
	}
	if f.LineSpacingAuto() != nil {
		t.Error("LineSpacingAuto() should be nil when rule is atLeast")
	}
}

func TestParagraphFormat_ClearLineSpacing_NoPPr(t *testing.T) {
	// Clearing on a paragraph that has no PPr must be a no-op — no PPr created.
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	f.ClearLineSpacing() // must not panic or allocate PPr

	// Verify no PPr leaked: Style() returns "" only when PPr is nil or has no styleId.
	if f.LineSpacingAuto() != nil || f.LineSpacingExact() != nil || f.LineSpacingAtLeast() != nil {
		t.Error("ClearLineSpacing() on empty paragraph must not allocate PPr")
	}
}

func TestParagraphFormat_ClearLineSpacing_WithExistingPPr(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	f.SetLineSpacingAuto(docx.DoubleLine())
	f.ClearLineSpacing()

	if f.LineSpacingAuto() != nil {
		t.Error("LineSpacingAuto() should be nil after ClearLineSpacing()")
	}
}

func TestParagraphFormat_LineSpacing_Helpers(t *testing.T) {
	if docx.SingleLine() != 240 {
		t.Errorf("SingleLine() = %d, want 240", docx.SingleLine())
	}
	if docx.OneAndAHalfLine() != 360 {
		t.Errorf("OneAndAHalfLine() = %d, want 360", docx.OneAndAHalfLine())
	}
	if docx.DoubleLine() != 480 {
		t.Errorf("DoubleLine() = %d, want 480", docx.DoubleLine())
	}
	if got := docx.MultipleLine(1.15); got != 276 {
		t.Errorf("MultipleLine(1.15) = %d, want 276", got)
	}
}

// ---------------------------------------------------------------------------
// Tri-state paragraph properties
// ---------------------------------------------------------------------------

func TestParagraphFormat_WidowControl_TriState(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	if f.WidowControl() != nil {
		t.Error("WidowControl() should be nil before any setter call")
	}

	f.SetWidowControl(docx.On())
	if b := f.WidowControl(); b == nil || !*b {
		t.Errorf("WidowControl() = %v, want true", b)
	}

	f.SetWidowControl(docx.Off())
	if b := f.WidowControl(); b == nil || *b {
		t.Errorf("WidowControl() = %v, want false", b)
	}

	f.SetWidowControl(nil)
	if f.WidowControl() != nil {
		t.Error("WidowControl() should be nil after SetWidowControl(nil)")
	}
}

func TestParagraphFormat_WidowControl_NilClear_NoPPr(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()
	// SetXxx(nil) on a paragraph with no PPr must not allocate PPr.
	f.SetWidowControl(nil)
	if f.Style() != "" {
		t.Error("SetWidowControl(nil) must not allocate PPr")
	}
}

func TestParagraphFormat_SuppressLineNumbers_TriState(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	if f.SuppressLineNumbers() != nil {
		t.Error("SuppressLineNumbers() should be nil initially")
	}

	f.SetSuppressLineNumbers(docx.On())
	if b := f.SuppressLineNumbers(); b == nil || !*b {
		t.Errorf("SuppressLineNumbers() = %v, want true", b)
	}

	f.SetSuppressLineNumbers(nil)
	if f.SuppressLineNumbers() != nil {
		t.Error("SuppressLineNumbers() should be nil after clear")
	}
}

func TestParagraphFormat_MirrorIndents_TriState(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	f := p.Format()

	if f.MirrorIndents() != nil {
		t.Error("MirrorIndents() should be nil initially")
	}

	f.SetMirrorIndents(docx.On())
	if b := f.MirrorIndents(); b == nil || !*b {
		t.Errorf("MirrorIndents() = %v, want true", b)
	}

	f.SetMirrorIndents(nil)
	if f.MirrorIndents() != nil {
		t.Error("MirrorIndents() should be nil after clear")
	}
}

// ---------------------------------------------------------------------------
// Run.AddBreak / Run.AddTab
// ---------------------------------------------------------------------------

func TestRun_AddBreak_TextRendering(t *testing.T) {
	tests := []struct {
		name     string
		kind     docx.BreakType
		wantRun  string
		wantPara string
	}{
		{"line break", docx.BreakLine, "a\n", "a\nb"},
		{"page break", docx.BreakPage, "a\n", "a\nb"},
		{"column break", docx.BreakColumn, "a\n", "a\nb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := docx.New().Body().AddParagraph()
			r := p.AddRun("a")
			r.AddBreak(tc.kind)
			p.AddRun("b")

			if got := r.Text(); got != tc.wantRun {
				t.Errorf("run.Text() = %q, want %q", got, tc.wantRun)
			}
			if got := p.Text(); got != tc.wantPara {
				t.Errorf("para.Text() = %q, want %q", got, tc.wantPara)
			}
		})
	}
}

func TestRun_AddTab_TextRendering(t *testing.T) {
	p := docx.New().Body().AddParagraph()
	r := p.AddRun("col1")
	r.AddTab()
	p.AddRun("col2")

	if got := r.Text(); got != "col1\t" {
		t.Errorf("run.Text() = %q, want %q", got, "col1\t")
	}
	if got := p.Text(); got != "col1\tcol2" {
		t.Errorf("para.Text() = %q, want %q", got, "col1\tcol2")
	}
}
