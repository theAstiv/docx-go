// Command edit changes an existing document: it sets the core properties,
// replaces text that Word may have split across runs, removes one table row
// and inserts another, adds a paragraph after an existing one, and prints the
// document's text.
//
// It is written against the report that examples/report generates:
//
//	go run ./examples/report report.docx
//	go run ./examples/edit report.docx edited.docx
//
// On another document the text and table edits find nothing to change, and
// the program says so.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/theAstiv/docx-go/docx"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: edit <input.docx> <output.docx>")
	}
	data, err := os.ReadFile(os.Args[1]) // #nosec G304 -- path is the explicit CLI argument
	if err != nil {
		log.Fatal(err)
	}
	d, err := docx.OpenBytes(data)
	if err != nil {
		log.Fatal(err)
	}

	edit(d)

	f, err := os.Create(os.Args[2]) // #nosec G304 -- path is the explicit CLI argument
	if err != nil {
		log.Fatal(err)
	}
	if err := d.Save(f); err != nil {
		_ = f.Close()
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Written %s", os.Args[2])
}

func edit(d *docx.Document) {
	// Document properties, shown by Word under File › Info.
	props := d.CoreProperties()
	props.SetTitle("Security Assessment Report: Acme Health")
	props.SetAuthor("Security team")
	props.SetLastModifiedBy("docx-go examples/edit")
	props.SetModified(time.Now().UTC().Truncate(time.Second))

	// Find and replace, in the body, headers and footers. Matches that
	// cannot be replaced safely (split by a field, inside a content control
	// or tracked change) are reported, not silently skipped.
	res := d.Replace("Example Corp.", "Acme Health", docx.ReplaceOptions{})
	log.Printf("replaced %d occurrence(s)", res.Count)
	for _, s := range res.Skipped {
		log.Printf("skipped %d match(es) in %s: %q", s.Count, s.Part, s.Text)
	}

	editFindings(d)

	// Add an italic note after an existing paragraph. The new paragraph has
	// no formatting of its own.
	for p := range d.Body().Paragraphs() {
		if strings.HasPrefix(p.Text(), "This report lists") {
			note := p.InsertParagraphAfter("")
			note.AddRun("F-04 was fixed before this review ended and is no longer listed.").Font().SetItalic(docx.On())
			break
		}
	}

	fmt.Print(d.Text())
}

// editFindings removes the row of finding F-04 and inserts F-05 after F-01
// in the first table.
func editFindings(d *docx.Document) {
	var tbl *docx.Table
	for t := range d.Body().Tables() {
		tbl = t
		break
	}
	if tbl == nil {
		log.Print("no table to edit")
		return
	}

	var rows []*docx.Row
	for r := range tbl.Rows() {
		rows = append(rows, r)
	}
	for _, r := range rows {
		if cells := cellsOf(r); len(cells) > 0 && cells[0].Text() == "F-04" {
			if err := r.Remove(); err != nil {
				log.Printf("remove F-04: %v", err)
			}
		}
	}

	// The new row copies the shape of the row it is inserted above: cell
	// widths, borders, shading and paragraph formatting.
	row := tbl.InsertRow(2, docx.RowOptions{})
	cells := cellsOf(row)
	if len(cells) < 3 {
		log.Print("the table does not have the report's three columns")
		return
	}
	cells[0].SetText("F-05")
	cells[1].SetText("Session cookie missing the Secure flag")
	cells[2].SetText("High")
	cells[2].SetShading("F4B6B6")
}

// cellsOf returns the cells of a row in order.
func cellsOf(r *docx.Row) []*docx.Cell {
	var out []*docx.Cell
	for c := range r.Cells() {
		out = append(out, c)
	}
	return out
}
