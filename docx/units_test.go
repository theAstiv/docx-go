package docx_test

import (
	"math"
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
	if got := docx.Cm(2.54); got != 1440 {
		t.Errorf("Cm(2.54) = %v, want 1440", got)
	}
}

// TestUnitsRound checks every float constructor rounds to the nearest unit
// and saturates instead of overflowing.
func TestUnitsRound(t *testing.T) {
	cases := []struct {
		name      string
		got, want int64
	}{
		{"Mm(1)", int64(docx.Mm(1)), 57},
		{"Mm(25.4)", int64(docx.Mm(25.4)), 1440},
		{"Cm(1)", int64(docx.Cm(1)), 567},
		{"Inches(0.7)", int64(docx.Inches(0.7)), 1008},
		{"Inches(-0.7)", int64(docx.Inches(-0.7)), -1008},
		{"Pt(10.33)", int64(docx.Pt(10.33)), 207},
		{"Px(1.1)", int64(docx.Px(1.1)), 17},
		{"Percent(33.33)", int64(docx.Percent(33.33)), 1667},
		{"EMUInches(0.1)", int64(docx.EMUInches(0.1)), 91440},
		{"MultipleLine(1.15)", int64(docx.MultipleLine(1.15)), 276},
		{"HalfPoints(10.75)", int64(docx.HalfPoints(10.75)), 22},
		{"Inches(1e30)", int64(docx.Inches(1e30)), math.MaxInt64},
		{"Inches(-1e30)", int64(docx.Inches(-1e30)), math.MinInt64},
		{"HalfPoints(1e30)", int64(docx.HalfPoints(1e30)), math.MaxInt32},
		{"Inches(NaN)", int64(docx.Inches(math.NaN())), 0},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
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
