package docx_test

import (
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestInches_RoundTrip(t *testing.T) {
	cases := []struct {
		inches float64
		twips  docx.Twip
	}{
		{0, 0},
		{1, 1440},
		{0.5, 720},
		{2.5, 3600},
	}
	for _, tc := range cases {
		got := docx.Inches(tc.inches)
		if got != tc.twips {
			t.Errorf("Inches(%v) = %v, want %v", tc.inches, got, tc.twips)
		}
	}
}

func TestCm(t *testing.T) {
	// 2.54 cm == 1 inch == 1440 twips
	got := docx.Cm(2.54)
	// Allow ±2 twips for float rounding
	diff := int64(got) - 1440
	if diff < -2 || diff > 2 {
		t.Errorf("Cm(2.54) = %v, want ~1440", got)
	}
}

func TestPt(t *testing.T) {
	if got := docx.Pt(12); got != 240 {
		t.Errorf("Pt(12) = %v, want 240", got)
	}
}

func TestPercent(t *testing.T) {
	if got := docx.Percent(100); got != 5000 {
		t.Errorf("Percent(100) = %v, want 5000", got)
	}
	if got := docx.Percent(50); got != 2500 {
		t.Errorf("Percent(50) = %v, want 2500", got)
	}
}

func TestEMU_TwipConversions(t *testing.T) {
	const inches = 1.0
	twip := docx.Inches(inches)
	emu := twip.EMU()
	// 1 inch = 914400 EMU
	if emu != 914400 {
		t.Errorf("1 inch EMU = %v, want 914400", emu)
	}
	back := emu.Twip()
	if back != twip {
		t.Errorf("EMU.Twip() = %v, want %v", back, twip)
	}
}

func TestHalfPoint(t *testing.T) {
	hp := docx.HalfPoints(12)
	if hp != 24 {
		t.Errorf("HalfPoints(12) = %v, want 24", hp)
	}
	if hp.Pt() != 12 {
		t.Errorf("HalfPoint(24).Pt() = %v, want 12", hp.Pt())
	}
}

func TestEMUInches(t *testing.T) {
	if got := docx.EMUInches(1); got != 914400 {
		t.Errorf("EMUInches(1) = %v, want 914400", got)
	}
}
