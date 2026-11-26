package docx_test

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// Footnotes, endnotes and comments are kept as bytes. They still count for
// document-wide ids (review M1), and removing the last reference to a note
// or comment removes it (C3, review L1).

const (
	notesNS = pathsWNS +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"`
	relFootnotes = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes`
	relComments  = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments`

	footnotesXML = `<w:footnotes ` + notesNS + `>` +
		`<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>` +
		`<w:footnote w:type="continuationSeparator" w:id="0"><w:p><w:r><w:continuationSeparator/></w:r></w:p></w:footnote>` +
		`<w:footnote w:id="1"><w:p><w:bookmarkStart w:id="7" w:name="Intro"/><w:r><w:t>note one</w:t></w:r><w:bookmarkEnd w:id="7"/>` +
		`<w:r><w:drawing><wp:inline><wp:extent cx="1" cy="1"/><wp:docPr id="40" name="p"/></wp:inline></w:drawing></w:r></w:p></w:footnote>` +
		`<w:footnote w:id="2"><w:p><w:r><w:t>note two</w:t></w:r></w:p></w:footnote>` +
		`</w:footnotes>`
	footnoteOne = `<w:footnote w:id="1">`

	commentsXML = `<w:comments ` + notesNS + `>` +
		`<w:comment w:id="5" w:author="A"><w:p><w:r><w:t>comment five</w:t></w:r></w:p></w:comment>` +
		`<w:comment w:id="6" w:author="A"><w:p><w:r><w:t>comment six</w:t></w:r></w:p></w:comment>` +
		`</w:comments>`
)

// notesDoc builds a package with the given body content, a footnotes part
// and a comments part.
func notesDoc(t *testing.T, body string) *docx.Document {
	t.Helper()
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels":         rels(rel("rId1", relDoc, "word/document.xml")),
		"word/_rels/document.xml.rels": rels(
			rel("rId1", relFootnotes, "footnotes.xml"),
			rel("rId2", relComments, "comments.xml")),
		"word/document.xml":  `<w:document ` + notesNS + `><w:body>` + body + `<w:sectPr/></w:body></w:document>`,
		"word/footnotes.xml": footnotesXML,
		"word/comments.xml":  commentsXML,
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func footnoteRef(id string) string {
	return `<w:r><w:footnoteReference w:id="` + id + `"/></w:r>`
}

// savedParts saves d and returns its entries; the saved package must open.
func savedParts(t *testing.T, d *docx.Document) map[string][]byte {
	t.Helper()
	out := saveBytes(t, d)
	if _, err := docx.OpenBytes(out); err != nil {
		t.Fatalf("saved package does not reopen: %v", err)
	}
	return readZip(t, out)
}

// hasElem reports whether xml holds an element with the given local name
// (any prefix, or none) and, when id is not "", that w:id (or id). Saved
// parts declare WordprocessingML as the default namespace, so a "w:"
// substring check would pass vacuously.
func hasElem(xml, local, id string) bool {
	pat := `<(?:\w+:)?` + regexp.QuoteMeta(local) + `[\s/>]`
	if id != "" {
		pat = `<(?:\w+:)?` + regexp.QuoteMeta(local) + `\s[^>]*?\b(?:\w+:)?id="` + regexp.QuoteMeta(id) + `"`
	}
	return regexp.MustCompile(pat).MatchString(xml)
}

func nthParagraph(d *docx.Document, n int) *docx.Paragraph {
	i := 0
	for p := range d.Body().Paragraphs() {
		if i == n {
			return p
		}
		i++
	}
	return nil
}

func TestNotes_BookmarkNamesAreDocumentWide(t *testing.T) {
	d := notesDoc(t, `<w:p><w:r><w:t>body</w:t></w:r>`+footnoteRef("1")+`</w:p>`)
	p := nthParagraph(d, 0)
	if _, err := p.AddBookmark("Intro"); !errors.Is(err, docx.ErrDuplicateBookmark) {
		t.Errorf("AddBookmark(name used in a footnote) = %v, want ErrDuplicateBookmark", err)
	}
	if d.Bookmark("Intro") == nil {
		t.Error("Document.Bookmark does not find the footnote's bookmark")
	}
	if _, err := p.AddBookmark("Fresh"); err != nil {
		t.Fatal(err)
	}
	doc := string(savedParts(t, d)["word/document.xml"])
	if !hasElem(doc, "bookmarkStart", "") {
		t.Fatalf("no bookmark saved:\n%s", doc)
	}
	if hasElem(doc, "bookmarkStart", "7") {
		t.Errorf("new bookmark reuses the footnote bookmark's id 7:\n%s", doc)
	}
}

func TestNotes_DocPrIDsAreDocumentWide(t *testing.T) {
	d := notesDoc(t, `<w:p><w:r><w:t>body</w:t></w:r>`+footnoteRef("1")+`</w:p>`)
	run := nthParagraph(d, 0).AddRun("")
	if _, err := run.AddImage(bytes.NewReader(pngBytes(t, 4, 4, 10)), docx.ImageOptions{}); err != nil {
		t.Fatal(err)
	}
	doc := string(savedParts(t, d)["word/document.xml"])
	if !hasElem(doc, "docPr", "41") {
		t.Errorf("new drawing id is not above the footnote drawing's id 40:\n%s", doc)
	}
}

func TestNotes_RemovingReferenceRemovesNote(t *testing.T) {
	d := notesDoc(t, `<w:p><w:r><w:t>a</w:t></w:r>`+footnoteRef("1")+`</w:p>`+
		`<w:p><w:r><w:t>b</w:t></w:r>`+footnoteRef("2")+`</w:p>`)
	if err := nthParagraph(d, 0).Remove(); err != nil {
		t.Fatal(err)
	}
	parts := savedParts(t, d)
	got := string(parts["word/footnotes.xml"])
	i := strings.Index(footnotesXML, footnoteOne)
	j := i + strings.Index(footnotesXML[i:], `</w:footnote>`) + len(`</w:footnote>`)
	if want := footnotesXML[:i] + footnotesXML[j:]; got != want {
		t.Errorf("footnotes.xml after removal:\n got %s\nwant %s", got, want)
	}
	// Its bookmark went with it, so the name is free again.
	if _, err := nthParagraph(d, 0).AddBookmark("Intro"); err != nil {
		t.Errorf("AddBookmark(name of a removed footnote's bookmark) = %v", err)
	}
}

func TestNotes_NoteStillReferencedStays(t *testing.T) {
	d := notesDoc(t, `<w:p>`+footnoteRef("1")+`</w:p><w:p>`+footnoteRef("1")+`</w:p>`)
	if err := nthParagraph(d, 0).Remove(); err != nil {
		t.Fatal(err)
	}
	if got := string(savedParts(t, d)["word/footnotes.xml"]); got != footnotesXML {
		t.Errorf("footnotes.xml changed although the note is still referenced:\n%s", got)
	}
}

func TestNotes_RemovingCommentReference(t *testing.T) {
	const body = `<w:p><w:commentRangeStart w:id="5"/><w:r><w:t>text</w:t></w:r><w:commentRangeEnd w:id="5"/>` +
		`<w:r><w:commentReference w:id="5"/></w:r></w:p>` +
		`<w:p><w:commentRangeStart w:id="6"/><w:r><w:t>keep</w:t></w:r><w:commentRangeEnd w:id="6"/>` +
		`<w:r><w:commentReference w:id="6"/></w:r></w:p>`
	t.Run("whole paragraph", func(t *testing.T) {
		d := notesDoc(t, body)
		if err := nthParagraph(d, 0).Remove(); err != nil {
			t.Fatal(err)
		}
		c := string(savedParts(t, d)["word/comments.xml"])
		if strings.Contains(c, `comment five`) || !strings.Contains(c, `comment six`) {
			t.Errorf("comments.xml after removing comment 5's paragraph:\n%s", c)
		}
	})
	t.Run("reference run only", func(t *testing.T) {
		d := notesDoc(t, body)
		var last *docx.Run
		for r := range nthParagraph(d, 0).Runs() {
			last = r
		}
		if err := last.Remove(); err != nil {
			t.Fatal(err)
		}
		parts := savedParts(t, d)
		if c := string(parts["word/comments.xml"]); strings.Contains(c, `comment five`) {
			t.Errorf("comment 5 kept after its reference was removed:\n%s", c)
		}
		doc := string(parts["word/document.xml"])
		if hasElem(doc, "commentRangeStart", "5") || hasElem(doc, "commentRangeEnd", "5") {
			t.Errorf("comment 5's range anchors kept:\n%s", doc)
		}
		if !hasElem(doc, "commentRangeStart", "6") || !hasElem(doc, "commentRangeEnd", "6") {
			t.Errorf("comment 6's range removed:\n%s", doc)
		}
	})
}

// Move and permission ranges pair like bookmarks; removing one end takes
// the other.
func TestNotes_MoveAndPermRangesPair(t *testing.T) {
	for _, tc := range []struct{ start, end string }{
		{`<w:moveFromRangeStart w:id="3" w:name="m"/>`, `<w:moveFromRangeEnd w:id="3"/>`},
		{`<w:moveToRangeStart w:id="3" w:name="m"/>`, `<w:moveToRangeEnd w:id="3"/>`},
		{`<w:permStart w:id="3" w:edGrp="everyone"/>`, `<w:permEnd w:id="3"/>`},
	} {
		d := notesDoc(t, `<w:p>`+tc.start+`<w:r><w:t>a</w:t></w:r></w:p><w:p><w:r><w:t>b</w:t></w:r>`+tc.end+`</w:p>`)
		if err := nthParagraph(d, 0).Remove(); err != nil {
			t.Fatal(err)
		}
		doc := string(savedParts(t, d)["word/document.xml"])
		name := strings.TrimPrefix(tc.end[1:strings.Index(tc.end, " ")], "w:")
		if hasElem(doc, name, "") {
			t.Errorf("%s kept after its start was removed:\n%s", name, doc)
		}
	}
}

// Anchors inside a text box in a typed w:drawing are seen when the
// paragraph holding the drawing is removed, and removed when their partner
// goes.
func TestNotes_AnchorsInTextBoxes(t *testing.T) {
	const box = `<w:p><w:r><w:drawing><wp:anchor><wp:docPr id="3" name="t"/><a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<wps:wsp><wps:txbx><w:txbxContent><w:p><w:bookmarkStart w:id="9" w:name="box"/><w:r><w:t>x</w:t></w:r></w:p></w:txbxContent></wps:txbx></wps:wsp>` +
		`</a:graphicData></a:graphic></wp:anchor></w:drawing></w:r></w:p>`
	const end = `<w:p><w:r><w:t>b</w:t></w:r><w:bookmarkEnd w:id="9"/></w:p>`
	t.Run("remove the box", func(t *testing.T) {
		d := notesDoc(t, box+end)
		if err := nthParagraph(d, 0).Remove(); err != nil {
			t.Fatal(err)
		}
		if doc := string(savedParts(t, d)["word/document.xml"]); hasElem(doc, "bookmarkEnd", "") {
			t.Errorf("bookmarkEnd kept after the text box holding its start was removed:\n%s", doc)
		}
	})
	t.Run("remove the end", func(t *testing.T) {
		d := notesDoc(t, box+end)
		if err := nthParagraph(d, 1).Remove(); err != nil {
			t.Fatal(err)
		}
		if doc := string(savedParts(t, d)["word/document.xml"]); hasElem(doc, "bookmarkStart", "") {
			t.Errorf("bookmarkStart in the text box kept after its end was removed:\n%s", doc)
		}
	})
}
