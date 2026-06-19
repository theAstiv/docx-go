package wml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// ── CT_Numbering ─────────────────────────────────────────────────────────────

// CT_Numbering is the root element of numbering.xml (w:numbering).
// Schema order: numPicBullet* → abstractNum* → num* → numIdMacAtCleanup?
type CT_Numbering struct {
	Attrs        []xml.Attr        // xmlns:* and mc:Ignorable preserved for nsfix round-trip
	rawPre       []*raw.RawElement // numPicBullet + any extras before abstractNums
	AbstractNums []*CT_AbstractNum
	Nums         []*CT_Num
	rawPost      []*raw.RawElement // numIdMacAtCleanup + trailing extras
}

// MarshalXML encodes the numbering element.
func (n *CT_Numbering) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{Name: ns.Qn("w:numbering"), Attr: n.Attrs}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := rawMarshalChildren(e, n.rawPre); err != nil {
		return err
	}
	for _, a := range n.AbstractNums {
		if err := a.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, num := range n.Nums {
		if err := num.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := rawMarshalChildren(e, n.rawPost); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:numbering")})
}

// UnmarshalXML decodes a w:numbering element.
func (n *CT_Numbering) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// Preserve xmlns:prefix declarations so nsfix can reconstruct them on the
	// output root element. Drop {Space:"", Local:"xmlns"} (default-namespace
	// pseudo-attr) — the encoder auto-emits it from Name.Space and a duplicate
	// would cause a parse-time XML error.
	for _, a := range start.Attr {
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
			continue
		}
		n.Attrs = append(n.Attrs, a)
	}
	seenTyped := false
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("CT_Numbering: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:abstractNum"):
				seenTyped = true
				a := new(CT_AbstractNum)
				if err := a.UnmarshalXML(d, t); err != nil {
					return err
				}
				n.AbstractNums = append(n.AbstractNums, a)
			case ns.Qn("w:num"):
				seenTyped = true
				num := new(CT_Num)
				if err := num.UnmarshalXML(d, t); err != nil {
					return err
				}
				n.Nums = append(n.Nums, num)
			default:
				r := new(raw.RawElement) //nolint:exhaustruct // zero-value decode target; UnmarshalXML populates all fields
				if err := r.UnmarshalXML(d, t); err != nil {
					return err
				}
				if !seenTyped {
					n.rawPre = append(n.rawPre, r)
				} else {
					n.rawPost = append(n.rawPost, r)
				}
			}
		}
	}
}

// AbstractNumByID returns the abstractNum with the given abstractNumId, or nil.
func (n *CT_Numbering) AbstractNumByID(id int64) *CT_AbstractNum {
	for _, a := range n.AbstractNums {
		if a.AbstractNumID == id {
			return a
		}
	}
	return nil
}

// NumByID returns the num with the given numId, or nil.
func (n *CT_Numbering) NumByID(id int64) *CT_Num {
	for _, num := range n.Nums {
		if num.NumID == id {
			return num
		}
	}
	return nil
}

// NextAbstractNumID returns the next unused abstractNumId.
func (n *CT_Numbering) NextAbstractNumID() int64 {
	var max int64
	for _, a := range n.AbstractNums {
		if a.AbstractNumID >= max {
			max = a.AbstractNumID + 1
		}
	}
	return max
}

// NextNumID returns the next unused numId (must be ≥ 1).
func (n *CT_Numbering) NextNumID() int64 {
	var max int64 = 1
	for _, num := range n.Nums {
		if num.NumID >= max {
			max = num.NumID + 1
		}
	}
	return max
}

// ── CT_AbstractNum ────────────────────────────────────────────────────────────

// CT_AbstractNum represents a w:abstractNum element.
type CT_AbstractNum struct {
	AbstractNumID int64
	rawMeta       []*raw.RawElement // nsid, multiLevelType, tmpl, name, styleLink, numStyleLink
	Lvls          []*CT_Lvl         // up to 9 levels (indexed by ilvl)
	rawPost       []*raw.RawElement // unknown elements appearing after lvl children (mc:AlternateContent, etc.)
}

// MarshalXML encodes w:abstractNum.
func (a *CT_AbstractNum) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{
		Name: ns.Qn("w:abstractNum"),
		Attr: []xml.Attr{{Name: ns.Qn("w:abstractNumId"), Value: strconv.FormatInt(a.AbstractNumID, 10)}},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := rawMarshalChildren(e, a.rawMeta); err != nil {
		return err
	}
	for _, lvl := range a.Lvls {
		if err := lvl.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := rawMarshalChildren(e, a.rawPost); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:abstractNum")})
}

// UnmarshalXML decodes a w:abstractNum element.
func (a *CT_AbstractNum) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	a.AbstractNumID = 0
	for _, attr := range start.Attr {
		if attr.Name.Local == "abstractNumId" {
			id, err := strconv.ParseInt(attr.Value, 10, 64)
			if err == nil {
				a.AbstractNumID = id
			}
		}
	}
	seenLvl := false
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("CT_AbstractNum: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			if t.Name == ns.Qn("w:lvl") {
				seenLvl = true
				lvl := new(CT_Lvl)
				if err := lvl.UnmarshalXML(d, t); err != nil {
					return err
				}
				a.Lvls = append(a.Lvls, lvl)
			} else {
				r := new(raw.RawElement) //nolint:exhaustruct // zero-value decode target; UnmarshalXML populates all fields
				if err := r.UnmarshalXML(d, t); err != nil {
					return err
				}
				if seenLvl {
					a.rawPost = append(a.rawPost, r)
				} else {
					a.rawMeta = append(a.rawMeta, r)
				}
			}
		}
	}
}

// LvlByIlvl returns the level entry for the given ilvl (0-based), or nil.
func (a *CT_AbstractNum) LvlByIlvl(ilvl int64) *CT_Lvl {
	for _, lvl := range a.Lvls {
		if lvl.ILvl == ilvl {
			return lvl
		}
	}
	return nil
}

// ── CT_Lvl ────────────────────────────────────────────────────────────────────

// CT_Lvl represents a w:lvl element (one list level definition).
// All child elements are stored as raw for round-trip fidelity; accessors
// read and write the key properties needed for authoring.
type CT_Lvl struct {
	ILvl    int64
	rawBody []*raw.RawElement // start, numFmt, lvlRestart, pStyle, isLgl, suff, lvlText, lvlJc, pPr, rPr, …
	// Non-body attrs (tplc, tentative) stored alongside ILvl for round-trip.
	extraAttrs []xml.Attr
}

// MarshalXML encodes a w:lvl element.
func (l *CT_Lvl) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	attrs := []xml.Attr{{Name: ns.Qn("w:ilvl"), Value: strconv.FormatInt(l.ILvl, 10)}}
	attrs = append(attrs, l.extraAttrs...)
	start := xml.StartElement{Name: ns.Qn("w:lvl"), Attr: attrs}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := rawMarshalChildren(e, l.rawBody); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:lvl")})
}

// UnmarshalXML decodes a w:lvl element.
func (l *CT_Lvl) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	l.ILvl = 0
	l.extraAttrs = l.extraAttrs[:0]
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "ilvl":
			id, err := strconv.ParseInt(attr.Value, 10, 64)
			if err == nil {
				l.ILvl = id
			}
		default:
			l.extraAttrs = append(l.extraAttrs, attr)
		}
	}
	children, err := rawUnmarshalChildren(d)
	if err != nil {
		return fmt.Errorf("CT_Lvl ilvl=%d: %w", l.ILvl, err)
	}
	l.rawBody = children
	return nil
}

// Start returns the w:start value for this level, or nil if not set.
func (l *CT_Lvl) Start() *int64 {
	return rawGetInt64Attr(l.rawBody, ns.Qn("w:start"), "val")
}

// NumFmt returns the w:numFmt value (e.g. "decimal", "bullet"), or "".
func (l *CT_Lvl) NumFmt() string {
	return rawGetAttrOnElement(l.rawBody, ns.Qn("w:numFmt"), "val")
}

// LvlText returns the w:lvlText value (e.g. "%1."), or "".
func (l *CT_Lvl) LvlText() string {
	return rawGetAttrOnElement(l.rawBody, ns.Qn("w:lvlText"), "val")
}

// SetStart sets w:start w:val for this level.
func (l *CT_Lvl) SetStart(v int64) {
	rawSetInt64Attr(&l.rawBody, ns.Qn("w:start"), ns.Qn("w:val"), &v)
}

// SetNumFmt sets w:numFmt w:val (e.g. "decimal", "bullet").
func (l *CT_Lvl) SetNumFmt(s string) {
	rawSetStr(&l.rawBody, ns.Qn("w:numFmt"), s)
}

// SetLvlText sets w:lvlText w:val (e.g. "%1.").
func (l *CT_Lvl) SetLvlText(s string) {
	rawSetStr(&l.rawBody, ns.Qn("w:lvlText"), s)
}

// SetLvlJc sets w:lvlJc w:val (list item justification, e.g. "left").
func (l *CT_Lvl) SetLvlJc(s string) {
	rawSetStr(&l.rawBody, ns.Qn("w:lvlJc"), s)
}

// SetIndent sets w:ind w:left and w:hanging inside a w:pPr child of this level.
// left and hanging are in twips.
func (l *CT_Lvl) SetIndent(left, hanging int64) {
	// Find or create w:pPr in rawBody.
	var ppr *raw.RawElement
	for _, r := range l.rawBody {
		if r.QName == ns.Qn("w:pPr") {
			ppr = r
			break
		}
	}
	if ppr == nil {
		ppr = new(raw.RawElement) //nolint:exhaustruct // populated immediately below
		ppr.QName = ns.Qn("w:pPr")
		l.rawBody = append(l.rawBody, ppr)
	}
	// Find or create w:ind inside ppr.Children.
	var ind *raw.RawElement
	for _, tok := range ppr.Children {
		if r, ok := tok.(*raw.RawElement); ok && r.QName == ns.Qn("w:ind") {
			ind = r
			break
		}
	}
	if ind == nil {
		ind = new(raw.RawElement) //nolint:exhaustruct // populated immediately below
		ind.QName = ns.Qn("w:ind")
		ppr.Children = append(ppr.Children, ind)
	}
	setAttrInSlice(&ind.Attrs, ns.Qn("w:left"), strconv.FormatInt(left, 10))
	setAttrInSlice(&ind.Attrs, ns.Qn("w:hanging"), strconv.FormatInt(hanging, 10))
}

// ── CT_Num ────────────────────────────────────────────────────────────────────

// CT_Num represents a w:num element.
type CT_Num struct {
	NumID         int64
	AbstractNumID int64
	LvlOverrides  []*CT_LvlOverride
}

// MarshalXML encodes a w:num element.
func (n *CT_Num) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{
		Name: ns.Qn("w:num"),
		Attr: []xml.Attr{{Name: ns.Qn("w:numId"), Value: strconv.FormatInt(n.NumID, 10)}},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	// w:abstractNumId child
	absEl := xml.StartElement{
		Name: ns.Qn("w:abstractNumId"),
		Attr: []xml.Attr{{Name: ns.Qn("w:val"), Value: strconv.FormatInt(n.AbstractNumID, 10)}},
	}
	if err := e.EncodeToken(absEl); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: ns.Qn("w:abstractNumId")}); err != nil {
		return err
	}
	for _, ov := range n.LvlOverrides {
		if err := ov.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:num")})
}

// UnmarshalXML decodes a w:num element.
func (n *CT_Num) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.NumID = 0
	n.AbstractNumID = 0
	n.LvlOverrides = n.LvlOverrides[:0]
	for _, attr := range start.Attr {
		if attr.Name.Local == "numId" {
			id, err := strconv.ParseInt(attr.Value, 10, 64)
			if err == nil {
				n.NumID = id
			}
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("CT_Num: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:abstractNumId"):
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						id, err := strconv.ParseInt(a.Value, 10, 64)
						if err == nil {
							n.AbstractNumID = id
						}
					}
				}
				if err := d.Skip(); err != nil {
					return fmt.Errorf("CT_Num abstractNumId skip: %w", err)
				}
			case ns.Qn("w:lvlOverride"):
				ov := new(CT_LvlOverride)
				if err := ov.UnmarshalXML(d, t); err != nil {
					return err
				}
				n.LvlOverrides = append(n.LvlOverrides, ov)
			default:
				if err := d.Skip(); err != nil {
					return fmt.Errorf("CT_Num skip %v: %w", t.Name, err)
				}
			}
		}
	}
}

// ── CT_LvlOverride ────────────────────────────────────────────────────────────

// CT_LvlOverride represents a w:lvlOverride element (CT_NumLvl in the schema).
type CT_LvlOverride struct {
	ILvl          int64
	StartOverride *int64
	Lvl           *CT_Lvl
}

// MarshalXML encodes a w:lvlOverride element.
func (o *CT_LvlOverride) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{
		Name: ns.Qn("w:lvlOverride"),
		Attr: []xml.Attr{{Name: ns.Qn("w:ilvl"), Value: strconv.FormatInt(o.ILvl, 10)}},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if o.StartOverride != nil {
		soEl := xml.StartElement{
			Name: ns.Qn("w:startOverride"),
			Attr: []xml.Attr{{Name: ns.Qn("w:val"), Value: strconv.FormatInt(*o.StartOverride, 10)}},
		}
		if err := e.EncodeToken(soEl); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: ns.Qn("w:startOverride")}); err != nil {
			return err
		}
	}
	if o.Lvl != nil {
		if err := o.Lvl.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:lvlOverride")})
}

// UnmarshalXML decodes a w:lvlOverride element.
func (o *CT_LvlOverride) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	o.ILvl = 0
	o.StartOverride = nil
	o.Lvl = nil
	for _, attr := range start.Attr {
		if attr.Name.Local == "ilvl" {
			id, err := strconv.ParseInt(attr.Value, 10, 64)
			if err == nil {
				o.ILvl = id
			}
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("CT_LvlOverride: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:startOverride"):
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						v, err := strconv.ParseInt(a.Value, 10, 64)
						if err == nil {
							o.StartOverride = &v
						}
					}
				}
				if err := d.Skip(); err != nil {
					return fmt.Errorf("CT_LvlOverride startOverride skip: %w", err)
				}
			case ns.Qn("w:lvl"):
				o.Lvl = new(CT_Lvl)
				if err := o.Lvl.UnmarshalXML(d, t); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return fmt.Errorf("CT_LvlOverride skip %v: %w", t.Name, err)
				}
			}
		}
	}
}

// ── SetNumPr / ClearNumPr on CT_PPr ──────────────────────────────────────────

// SetNumPr sets w:numPr with the given numId and ilvl on this paragraph property.
// It replaces any existing w:numPr. Use ClearNumPr to remove it entirely.
func (p *CT_PPr) SetNumPr(numID, ilvl int64) {
	ilvlEl := new(raw.RawElement)
	ilvlEl.QName = ns.Qn("w:ilvl")
	ilvlEl.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: strconv.FormatInt(ilvl, 10)}}

	numIDEl := new(raw.RawElement)
	numIDEl.QName = ns.Qn("w:numId")
	numIDEl.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: strconv.FormatInt(numID, 10)}}

	numPrEl := new(raw.RawElement)
	numPrEl.QName = ns.Qn("w:numPr")
	numPrEl.Children = []xml.Token{ilvlEl, numIDEl}

	rawRemoveChild(&p.raw, ns.Qn("w:numPr"))
	rawInsertOrdered(&p.raw, numPrEl, pPrChildSeqPos)
}

// ClearNumPr removes any w:numPr from this paragraph property.
func (p *CT_PPr) ClearNumPr() {
	rawRemoveChild(&p.raw, ns.Qn("w:numPr"))
}

// ── Registry ─────────────────────────────────────────────────────────────────

func init() {
	// CT_Numbering is the part root; it is not dispatched via body/paragraph,
	// so it does not need a registry entry — the part's Parse() calls UnmarshalXML
	// directly. CT_AbstractNum, CT_Num, CT_Lvl, CT_LvlOverride are also not
	// body-level block elements, so only CT_Numbering itself needs registration
	// if it appears as an embedded element (it never does in practice).
	// No registry entries are needed for numbering types.
}
