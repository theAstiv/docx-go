package wml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestNewFieldRuns_Layout(t *testing.T) {
	runs := wml.NewFieldRuns(`PAGE \* MERGEFORMAT`, "7")
	if len(runs) != 5 {
		t.Fatalf("%d runs, want 5", len(runs))
	}
	types := []string{wml.FldCharBegin, "", wml.FldCharSeparate, "", wml.FldCharEnd}
	for i, r := range runs {
		if types[i] == "" {
			continue
		}
		fc, ok := r.Children[0].(*wml.CT_FldChar)
		if !ok || fc.FldCharType != types[i] {
			t.Errorf("run %d is not fldChar %q", i, types[i])
		}
	}
	it := runs[1].Children[0].(*wml.CT_InstrText)
	if it.Content != ` PAGE \* MERGEFORMAT ` || it.Space != "preserve" {
		t.Errorf("instrText = %q space %q", it.Content, it.Space)
	}
	if runs[3].Text() != "7" {
		t.Errorf("result run text = %q", runs[3].Text())
	}
	for i, r := range runs {
		roundtrip.Verify[*wml.CT_R](t, r, ns.Qn("w:r"))
		_ = i
	}
}

func TestNewFieldRuns_EmptyResultHasNoText(t *testing.T) {
	runs := wml.NewFieldRuns("TOC", "")
	if len(runs[3].Children) != 0 {
		t.Errorf("result run should be empty, has %d children", len(runs[3].Children))
	}
}

func TestCT_Settings_UpdateFields_Ordering(t *testing.T) {
	src := `<w:settings xmlns:w="` + ns.W + `">` +
		`<w:zoom w:percent="100"/><w:defaultTabStop w:val="720"/>` +
		`<w:compat><w:compatSetting w:name="x" w:uri="u" w:val="1"/></w:compat>` +
		`<w:rsids/></w:settings>`
	var s wml.CT_Settings
	if err := xml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatal(err)
	}
	if s.UpdateFields() != nil {
		t.Error("UpdateFields should be nil when absent")
	}
	b := true
	s.SetUpdateFields(&b)
	out, err := xml.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	tab := strings.Index(got, "defaultTabStop")
	upd := strings.Index(got, "updateFields")
	compat := strings.Index(got, "<compat")
	if tab >= upd || upd >= compat {
		t.Errorf("updateFields out of schema order (tab %d, upd %d, compat %d): %s", tab, upd, compat, got)
	}
	if v := s.UpdateFields(); v == nil || !*v {
		t.Errorf("UpdateFields = %v", v)
	}
	roundtrip.Verify[*wml.CT_Settings](t, &s, ns.Qn("w:settings"))

	// Setting again replaces rather than duplicates.
	f := false
	s.SetUpdateFields(&f)
	out, _ = xml.Marshal(&s)
	if strings.Count(string(out), "<updateFields") != 1 {
		t.Errorf("duplicate updateFields: %s", out)
	}
	s.SetUpdateFields(nil)
	if s.UpdateFields() != nil {
		t.Error("nil should remove updateFields")
	}
}

func TestCT_Settings_UpdateFields_AppendsWhenNothingFollows(t *testing.T) {
	var s wml.CT_Settings
	b := true
	s.SetUpdateFields(&b)
	if v := s.UpdateFields(); v == nil || !*v {
		t.Errorf("UpdateFields = %v", v)
	}
}
