package docx

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/parts"
)

// ── ListStyle ─────────────────────────────────────────────────────────────────

// ListStyle controls the bullet/number format applied to new lists.
type ListStyle int

const (
	// ListDecimal is a numbered list (1. 2. 3. …).
	ListDecimal ListStyle = iota
	// ListBullet is an unordered bullet list.
	ListBullet
	// ListLowerLetter uses lowercase letters (a. b. c. …).
	ListLowerLetter
	// ListUpperLetter uses uppercase letters (A. B. C. …).
	ListUpperLetter
	// ListLowerRoman uses lowercase roman numerals (i. ii. iii. …).
	ListLowerRoman
	// ListUpperRoman uses uppercase roman numerals (I. II. III. …).
	ListUpperRoman
)

// numFmtStr maps a ListStyle to the OOXML numFmt string value.
func (s ListStyle) numFmtStr() string {
	switch s {
	case ListBullet:
		return "bullet"
	case ListLowerLetter:
		return "lowerLetter"
	case ListUpperLetter:
		return "upperLetter"
	case ListLowerRoman:
		return "lowerRoman"
	case ListUpperRoman:
		return "upperRoman"
	default: // ListDecimal
		return "decimal"
	}
}

// levelTextPattern returns the w:lvlText val for the given style and level (0-based).
func (s ListStyle) levelTextPattern(ilvl int) string {
	if s == ListBullet {
		switch ilvl % 3 {
		case 1:
			return "o"
		case 2:
			return "▪" // ▪
		default:
			return "•" // •
		}
	}
	// Numbered: "%1." for level 0, "%1.%2." for level 1, etc.
	var sb strings.Builder
	for i := 0; i <= ilvl; i++ {
		sb.WriteByte('%')
		sb.WriteByte(byte('1' + i))
		sb.WriteByte('.')
	}
	return sb.String()
}

// ── ListOptions ───────────────────────────────────────────────────────────────

// ListOptions configures a new list created by Document.CreateList.
type ListOptions struct {
	// Style is the list format. Defaults to ListDecimal.
	Style ListStyle
	// IndentPerLevel is the left-indent step per level in twips.
	// Defaults to 720 twips (0.5 inch).
	IndentPerLevel Twip
	// HangingIndent is the hanging indent (text outdent) in twips.
	// Defaults to 360 twips (0.25 inch).
	HangingIndent Twip
}

// ── List ──────────────────────────────────────────────────────────────────────

// List is a proxy for a numbered or bulleted list definition stored in
// numbering.xml.  It holds the numId used by paragraph w:numPr references.
type List struct {
	pkg   *parts.Package
	numID int64
}

// NumID returns the w:numId value that paragraphs reference.
func (l *List) NumID() int64 {
	return l.numID
}

// RestartAt returns a new List that restarts at start (usually 1) at the given
// level.  Callers pass the returned list to Paragraph.SetList to restart the
// count from that paragraph.
func (l *List) RestartAt(level int, start int) *List {
	np := l.pkg.EnsureNumberingPart()
	ct := np.CT()

	// Find the original CT_Num to get its abstractNumID.
	orig := ct.NumByID(l.numID)
	var absID int64
	if orig != nil {
		absID = orig.AbstractNumID
	}

	newNumID := ct.NextNumID()
	startVal := int64(start)
	override := &wml.CT_LvlOverride{
		ILvl:          int64(level),
		StartOverride: &startVal,
		Lvl:           nil,
	}
	num := &wml.CT_Num{
		NumID:         newNumID,
		AbstractNumID: absID,
		LvlOverrides:  []*wml.CT_LvlOverride{override},
	}
	ct.Nums = append(ct.Nums, num)
	return &List{pkg: l.pkg, numID: newNumID}
}

// ── Document.CreateList ───────────────────────────────────────────────────────

// CreateList creates a new list definition (abstractNum + num) in numbering.xml
// and returns a List proxy that paragraphs can reference via SetList.
func (d *Document) CreateList(opts ListOptions) *List {
	if opts.IndentPerLevel == 0 {
		opts.IndentPerLevel = Twip(720)
	}
	if opts.HangingIndent == 0 {
		opts.HangingIndent = Twip(360)
	}

	np := d.pkg.EnsureNumberingPart()
	ct := np.CT()

	absID := ct.NextAbstractNumID()
	abstractNum := buildAbstractNum(absID, opts)
	ct.AbstractNums = append(ct.AbstractNums, abstractNum)

	numID := ct.NextNumID()
	num := &wml.CT_Num{
		NumID:         numID,
		AbstractNumID: absID,
		LvlOverrides:  nil,
	}
	ct.Nums = append(ct.Nums, num)

	return &List{pkg: d.pkg, numID: numID}
}

// buildAbstractNum creates a CT_AbstractNum with 9 levels for the given options.
func buildAbstractNum(id int64, opts ListOptions) *wml.CT_AbstractNum {
	a := &wml.CT_AbstractNum{AbstractNumID: id, Lvls: nil}
	for i := range 9 {
		a.Lvls = append(a.Lvls, buildLvl(int64(i), opts))
	}
	return a
}

// buildLvl creates a CT_Lvl with raw children for the given level and options.
func buildLvl(ilvl int64, opts ListOptions) *wml.CT_Lvl {
	lvl := new(wml.CT_Lvl)
	lvl.ILvl = ilvl

	// The level body is stored as raw elements for round-trip fidelity.
	// We build them via XML round-trip to avoid constructing raw.RawElement trees by hand.
	startVal := int64(1)
	lvl.SetStart(startVal)
	lvl.SetNumFmt(opts.Style.numFmtStr())
	lvl.SetLvlText(opts.Style.levelTextPattern(int(ilvl)))
	lvl.SetLvlJc("left")
	lvl.SetIndent(int64((ilvl+1)*int64(opts.IndentPerLevel)), int64(opts.HangingIndent))
	return lvl
}
