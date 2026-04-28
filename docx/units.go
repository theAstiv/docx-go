package docx

// Twip is a twentieth of a typographical point (1440 per inch).
// Used for indents, paragraph spacing, and table widths.
type Twip int64

// EMU is an English Metric Unit (914400 per inch).
// Used for DrawingML positions and sizes.
type EMU int64

// HalfPoint is half of a typographical point.
// Used for font sizes (e.g. 24 HalfPoints = 12pt).
type HalfPoint int32

// EighthPoint is one eighth of a typographical point.
// Used for border widths.
type EighthPoint int32

// FiftiethPercent is one fiftieth of a percent (i.e. 5000 = 100%).
// Used for table widths expressed as percentages.
type FiftiethPercent int32

// LineSpacing240 is one two-hundred-fortieth of a line.
// Used for w:spacing/@w:line when lineRule="auto".
type LineSpacing240 int32

// ---- Constructors -----------------------------------------------------------

const (
	twipsPerInch = 1440
	twipsPerPt   = 20
	twipsPerPx   = 15 // at 96 dpi: 1px = 0.75pt = 15 twips
	emuPerInch   = 914400
	emuPerTwip   = emuPerInch / twipsPerInch // 635
)

// Inches returns a Twip value for the given number of inches.
func Inches(n float64) Twip { return Twip(n * twipsPerInch) }

// Cm returns a Twip value for the given number of centimetres.
// Computed from twipsPerInch / 2.54 to stay consistent with Inches.
func Cm(n float64) Twip { return Twip(n * twipsPerInch / 2.54) }

// Mm returns a Twip value for the given number of millimetres.
// Computed from twipsPerInch / 25.4 to stay consistent with Inches.
func Mm(n float64) Twip { return Twip(n * twipsPerInch / 25.4) }

// Pt returns a Twip value for the given number of typographical points.
func Pt(n float64) Twip { return Twip(n * twipsPerPt) }

// Px returns a Twip value for the given number of CSS pixels (at 96 dpi).
func Px(n float64) Twip { return Twip(n * twipsPerPx) }

// Percent returns a FiftiethPercent value for the given percentage (0–100).
func Percent(pct float64) FiftiethPercent { return FiftiethPercent(pct * 50) }

// EMUInches returns an EMU value for the given number of inches.
func EMUInches(n float64) EMU { return EMU(n * emuPerInch) }

// ---- LineSpacing240 helpers -------------------------------------------------

// SingleLine returns 240 — single-line spacing for use with SetLineSpacingAuto.
func SingleLine() LineSpacing240 { return 240 }

// OneAndAHalfLine returns 360 — 1.5-line spacing for use with SetLineSpacingAuto.
func OneAndAHalfLine() LineSpacing240 { return 360 }

// DoubleLine returns 480 — double-line spacing for use with SetLineSpacingAuto.
func DoubleLine() LineSpacing240 { return 480 }

// MultipleLine converts a multiplier to LineSpacing240 (e.g. MultipleLine(1.15) → 276).
func MultipleLine(n float64) LineSpacing240 { return LineSpacing240(n * 240) }

// ---- Conversions ------------------------------------------------------------

// EMU converts a Twip to EMU.
func (t Twip) EMU() EMU { return EMU(t) * emuPerTwip }

// Twip converts an EMU to Twip.
func (e EMU) Twip() Twip { return Twip(e / emuPerTwip) }

// Pt converts a HalfPoint to floating-point typographical points.
func (h HalfPoint) Pt() float64 { return float64(h) / 2 }

// HalfPoints converts a floating-point point size to HalfPoint.
func HalfPoints(pt float64) HalfPoint { return HalfPoint(pt * 2) }

// Pt converts an EighthPoint to floating-point points.
func (ep EighthPoint) Pt() float64 { return float64(ep) / 8 }
