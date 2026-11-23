package textmerge

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// splitParagraph builds a paragraph whose text is cut into runs at random
// points, with spell-check marks, the _GoBack bookmark and a tab sprinkled
// between them, and returns it with the ranges of its tags.
func splitParagraph(t testing.TB, rng *rand.Rand, tags int) (string, []Range) {
	var text strings.Builder
	var ranges []Range
	for i := range tags {
		if rng.Intn(3) == 0 {
			text.WriteString("x ")
		}
		tag := fmt.Sprintf("{{t%d}}", i)
		ranges = append(ranges, Range{Start: text.Len(), End: text.Len() + len(tag)})
		text.WriteString(tag)
	}
	s := text.String()
	var b strings.Builder
	cut := 0
	goBack := false
	for cut < len(s) {
		n := 1 + rng.Intn(4)
		if cut+n > len(s) {
			n = len(s) - cut
		}
		b.WriteString(`<w:r><w:t xml:space="preserve">` + s[cut:cut+n] + `</w:t>`)
		if rng.Intn(4) == 0 {
			b.WriteString(`<w:t xml:space="preserve"></w:t>`)
		}
		b.WriteString(`</w:r>`)
		switch rng.Intn(6) {
		case 0:
			b.WriteString(`<w:proofErr w:type="spellStart"/>`)
		case 1:
			if !goBack {
				goBack = true
				b.WriteString(`<w:bookmarkStart w:id="0" w:name="_GoBack"/><w:bookmarkEnd w:id="0"/>`)
			}
		}
		cut += n
	}
	return b.String(), ranges
}

// CollapseAll gives the same paragraph as collapsing one range at a time.
func TestCollapseAll_MatchesSequential(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for iter := range 300 {
		inner, ranges := splitParagraph(t, rng, 1+rng.Intn(6))
		seq := parseP(t, inner)
		for _, r := range ranges {
			flat, pieces := Flatten(seq.Children, nil)
			var ok bool
			seq.Children, ok = Collapse(seq.Children, pieces, flat, r.Start, r.End)
			if !ok {
				t.Fatalf("iter %d: sequential Collapse refused %v", iter, r)
			}
		}
		all := parseP(t, inner)
		flat, pieces := Flatten(all.Children, nil)
		var oks []bool
		all.Children, oks = CollapseAll(all.Children, pieces, flat, ranges)
		for i, ok := range oks {
			if !ok {
				t.Fatalf("iter %d: CollapseAll refused %v", iter, ranges[i])
			}
		}
		if got, want := runTexts(all.Children), runTexts(seq.Children); got != want {
			t.Fatalf("iter %d:\n all: %s\n seq: %s\n in: %s", iter, got, want, inner)
		}
		gotFlat, _ := Flatten(all.Children, nil)
		if gotFlat != flat {
			t.Fatalf("iter %d: text changed: %q -> %q", iter, flat, gotFlat)
		}
	}
}

// A range holding a barrier is refused; the others still collapse.
func TestCollapseAll_BarrierRange(t *testing.T) {
	p := parseP(t, `<w:r><w:t>{{a</w:t></w:r><w:r><w:t>}}x</w:t></w:r><w:r><w:tab/></w:r>`+
		`<w:r><w:t>y</w:t></w:r><w:r><w:t>{{b</w:t></w:r><w:r><w:t>}}</w:t></w:r>`)
	flat, pieces := Flatten(p.Children, nil)
	out, oks := CollapseAll(p.Children, pieces, flat, []Range{{0, 5}, {5, 8}, {8, 13}})
	if !oks[0] || oks[1] || !oks[2] {
		t.Errorf("oks = %v, want [true false true]", oks)
	}
	if got := runTexts(out); got != "[{{a}}] [x] [<>] [y] [{{b}}]" {
		t.Errorf("runs = %s", got)
	}
}

// Many split tags in one paragraph collapse in linear time.
func TestCollapseAll_Scale(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	inner, ranges := splitParagraph(t, rng, 8000)
	p := parseP(t, inner)
	start := time.Now()
	flat, pieces := Flatten(p.Children, nil)
	CollapseAll(p.Children, pieces, flat, ranges)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("8000 split tags took %v", d)
	}
}
