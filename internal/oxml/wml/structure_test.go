package wml_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestMarksIn_TypedAndRaw(t *testing.T) {
	b := bodyFrom(t,
		`<w:p><w:bookmarkStart w:id="1" w:name="a"/><w:r><w:fldChar w:fldCharType="begin"/></w:r>`+
			`<w:ins w:id="9" w:author="x"><w:r><w:fldChar w:fldCharType="end"/></w:r><w:commentRangeStart w:id="4"/></w:ins>`+
			`<w:hyperlink w:anchor="z"><w:bookmarkEnd w:id="1"/></w:hyperlink></w:p>`)
	marks := wml.MarksIn(b.Children[0])
	want := []wml.Mark{
		{Kind: wml.MarkBookmarkStart, ID: 1},
		{Kind: wml.MarkFldChar, FldCharType: "begin"},
		{Kind: wml.MarkFldChar, FldCharType: "end"},
		{Kind: wml.MarkCommentStart, ID: 4},
		{Kind: wml.MarkBookmarkEnd, ID: 1},
	}
	if len(marks) != len(want) {
		t.Fatalf("marks = %+v", marks)
	}
	for i := range want {
		if marks[i] != want[i] {
			t.Errorf("mark %d = %+v, want %+v", i, marks[i], want[i])
		}
	}
	if !wml.FieldsBalanced(marks) {
		t.Error("begin/end across typed and raw runs should balance")
	}
	orph := wml.OrphanedPartners(marks)
	if len(orph) != 1 || !orph[wml.MarkCommentEnd][4] {
		t.Errorf("orphans = %+v, want comment end 4 only", orph)
	}
}

func TestFieldsBalanced(t *testing.T) {
	f := func(types ...string) []wml.Mark {
		var m []wml.Mark
		for _, ty := range types {
			m = append(m, wml.Mark{Kind: wml.MarkFldChar, FldCharType: ty})
		}
		return m
	}
	for _, c := range []struct {
		marks []wml.Mark
		want  bool
	}{
		{nil, true},
		{f("begin", "separate", "end"), true},
		{f("begin", "begin", "end", "separate", "end"), true},
		{f("begin", "separate"), false},
		{f("separate", "end"), false},
		{f("end"), false},
		{f("separate"), false},
	} {
		if got := wml.FieldsBalanced(c.marks); got != c.want {
			t.Errorf("%+v: %v, want %v", c.marks, got, c.want)
		}
	}
}

func TestRemoveAnchors_Everywhere(t *testing.T) {
	b := bodyFrom(t,
		`<w:bookmarkEnd w:id="1"/>`+ // block level (raw)
			`<w:p><w:bookmarkEnd w:id="2"/><w:hyperlink w:anchor="z"><w:bookmarkEnd w:id="3"/><w:r><w:t>x</w:t></w:r></w:hyperlink>`+
			`<w:smartTag w:uri="u" w:element="e"><w:bookmarkEnd w:id="4"/><w:commentRangeEnd w:id="4"/></w:smartTag></w:p>`+
			`<w:tbl><w:bookmarkEnd w:id="5"/><w:tr><w:bookmarkEnd w:id="6"/><w:tc><w:p><w:bookmarkEnd w:id="7"/></w:p></w:tc></w:tr></w:tbl>`+
			`<w:sdt><w:sdtContent><w:p><w:bookmarkEnd w:id="8"/><w:bookmarkEnd w:id="99"/></w:p></w:sdtContent></w:sdt>`)
	ids := map[wml.MarkKind]map[int64]bool{
		wml.MarkBookmarkEnd: {1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true},
	}
	wml.RemoveAnchors(&b.Children, ids)
	var buf bytes.Buffer
	if err := xml.NewEncoder(&buf).Encode(b); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if n := strings.Count(out, "<bookmarkEnd"); n != 1 || !strings.Contains(out, `id="99"`) {
		t.Errorf("want only bookmarkEnd 99 left, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "commentRangeEnd") {
		t.Error("comment anchor with the same id as a bookmark was removed")
	}
	if !strings.Contains(out, ">x<") {
		t.Error("text was removed")
	}
}
