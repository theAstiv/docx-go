package wml

import (
	"encoding/xml"
	"sort"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// Property containers (w:pPr, w:rPr, w:sectPr) keep their children as raw
// elements, and most typed setters append. Word and the OOXML schema want the
// children in xs:sequence order, so these containers put them in order when
// they are marshalled. The sort is stable and only reorders children that are
// out of order, so a document that was already valid is written back unchanged.

// rPrChildSeqPos is the xs:sequence order of w:rPr children. The change-
// tracking children that CT_ParaRPr allows first (ins, del, move*) sort first.
var rPrChildSeqPos = map[xml.Name]int{
	ns.Qn("w:ins"):             1,
	ns.Qn("w:del"):             2,
	ns.Qn("w:moveFrom"):        3,
	ns.Qn("w:moveTo"):          4,
	ns.Qn("w:rStyle"):          5,
	ns.Qn("w:rFonts"):          6,
	ns.Qn("w:b"):               7,
	ns.Qn("w:bCs"):             8,
	ns.Qn("w:i"):               9,
	ns.Qn("w:iCs"):             10,
	ns.Qn("w:caps"):            11,
	ns.Qn("w:smallCaps"):       12,
	ns.Qn("w:strike"):          13,
	ns.Qn("w:dstrike"):         14,
	ns.Qn("w:outline"):         15,
	ns.Qn("w:shadow"):          16,
	ns.Qn("w:emboss"):          17,
	ns.Qn("w:imprint"):         18,
	ns.Qn("w:noProof"):         19,
	ns.Qn("w:snapToGrid"):      20,
	ns.Qn("w:vanish"):          21,
	ns.Qn("w:webHidden"):       22,
	ns.Qn("w:color"):           23,
	ns.Qn("w:spacing"):         24,
	ns.Qn("w:w"):               25,
	ns.Qn("w:kern"):            26,
	ns.Qn("w:position"):        27,
	ns.Qn("w:sz"):              28,
	ns.Qn("w:szCs"):            29,
	ns.Qn("w:highlight"):       30,
	ns.Qn("w:u"):               31,
	ns.Qn("w:effect"):          32,
	ns.Qn("w:bdr"):             33,
	ns.Qn("w:shd"):             34,
	ns.Qn("w:fitText"):         35,
	ns.Qn("w:vertAlign"):       36,
	ns.Qn("w:rtl"):             37,
	ns.Qn("w:cs"):              38,
	ns.Qn("w:em"):              39,
	ns.Qn("w:lang"):            40,
	ns.Qn("w:eastAsianLayout"): 41,
	ns.Qn("w:specVanish"):      42,
	ns.Qn("w:oMath"):           43,
	ns.Qn("w:rPrChange"):       44,
}

// sectPrChildSeqPos is the xs:sequence order of w:sectPr children. Header and
// footer references share a position, so they keep their relative order.
var sectPrChildSeqPos = map[xml.Name]int{
	ns.Qn("w:headerReference"): 1,
	ns.Qn("w:footerReference"): 1,
	ns.Qn("w:footnotePr"):      2,
	ns.Qn("w:endnotePr"):       3,
	ns.Qn("w:type"):            4,
	ns.Qn("w:pgSz"):            5,
	ns.Qn("w:pgMar"):           6,
	ns.Qn("w:paperSrc"):        7,
	ns.Qn("w:pgBorders"):       8,
	ns.Qn("w:lnNumType"):       9,
	ns.Qn("w:pgNumType"):       10,
	ns.Qn("w:cols"):            11,
	ns.Qn("w:formProt"):        12,
	ns.Qn("w:vAlign"):          13,
	ns.Qn("w:noEndnote"):       14,
	ns.Qn("w:titlePg"):         15,
	ns.Qn("w:textDirection"):   16,
	ns.Qn("w:bidi"):            17,
	ns.Qn("w:rtlGutter"):       18,
	ns.Qn("w:docGrid"):         19,
	ns.Qn("w:printerSettings"): 20,
	ns.Qn("w:sectPrChange"):    21,
}

// orderBySeq returns children in xs:sequence order according to seq. Elements
// not named in seq (extensions) take the position of the element before them
// so they stay attached to it. The input is returned as is, with no copy, when
// it is already in order; otherwise a sorted copy is returned and the input is
// left untouched.
func orderBySeq(children []*raw.RawElement, seq map[xml.Name]int) []*raw.RawElement {
	keys := make([]int, len(children))
	prev := 0
	sorted := true
	for i, c := range children {
		k, ok := seq[c.QName]
		if !ok {
			k = prev
		}
		keys[i] = k
		if k < prev {
			sorted = false
		}
		prev = k
	}
	if sorted {
		return children
	}
	idx := make([]int, len(children))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return keys[idx[a]] < keys[idx[b]] })
	out := make([]*raw.RawElement, len(children))
	for i, j := range idx {
		out[i] = children[j]
	}
	return out
}
