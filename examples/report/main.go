// Command report generates a sample report that uses every Phase 3a feature:
// styles, a table of contents, bookmarks, internal and external hyperlinks,
// a formatted table with a repeating header row, an inline image with alt
// text, and "Page X of Y" in the footer.
//
// Usage:
//
//	go run ./examples/report [output.docx]
//
// Word asks to update fields when it opens the file; accept to fill the table
// of contents and page numbers.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"

	"github.com/theAstiv/docx-go/docx"
)

func main() {
	out := "report.docx"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	d, err := build()
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(out) // #nosec G304 -- the output path is chosen by the user running the example.
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
	log.Printf("Written %s", out)
}

// finding is one row of the findings table.
type finding struct {
	id, title, severity string
}

var findings = []finding{
	{"F-01", "Unauthenticated admin endpoint", "High"},
	{"F-02", "Verbose error messages leak stack traces", "Medium"},
	{"F-03", "Missing rate limit on password reset", "Medium"},
	{"F-04", "Outdated TLS configuration on legacy host", "Low"},
}

// severityFill maps a severity to a cell shading colour (hex RRGGBB).
var severityFill = map[string]string{
	"High":   "F4B6B6",
	"Medium": "FCE4A8",
	"Low":    "CDE8C5",
}

func build() (*docx.Document, error) {
	d := docx.New()
	body := d.Body()
	styles := d.Styles()

	// Styles: Word-compatible built-ins, then a customised heading colour.
	for _, s := range []docx.BuiltinStyle{
		docx.StyleNormal, docx.StyleTitle, docx.StyleHeading1, docx.StyleHeading2,
		docx.StyleTableGrid, docx.StyleCaption,
	} {
		styles.EnsureBuiltin(s)
	}
	styles.ByID("Heading1").Font().SetColor("1F3864")
	styles.DocDefaultsFont().SetSizePt(11)
	after := docx.Pt(8)
	styles.DocDefaultsFormat().SetSpaceAfter(&after)

	// Title page: no footer on the first page.
	d.LastSection().EnableTitlePage(true)
	title := body.AddParagraph()
	title.Format().SetStyle("Title")
	title.Format().SetAlignment(docx.AlignCenter)
	title.AddRun("Security Assessment Report")
	sub := body.AddParagraph()
	sub.Format().SetAlignment(docx.AlignCenter)
	sub.AddRun("Example Corp. internal review")
	body.AddParagraph().AddRun("").AddBreak(docx.BreakPage)

	// Table of contents.
	contents := body.AddParagraph()
	contents.Format().SetStyle("Heading1")
	contents.AddRun("Contents")
	if _, err := d.AddTOC(docx.TOCOptions{Levels: "1-2", Hyperlinks: true}); err != nil {
		return nil, fmt.Errorf("toc: %w", err)
	}
	body.AddParagraph().AddRun("").AddBreak(docx.BreakPage)

	// Section 1 with a bookmark, an internal link and an external link.
	h1 := body.AddParagraph()
	h1.Format().SetStyle("Heading1")
	h1.AddRun("1. Summary")
	if _, err := h1.AddBookmark("sec_summary"); err != nil {
		return nil, fmt.Errorf("bookmark: %w", err)
	}

	intro := body.AddParagraph()
	intro.AddRun("This report lists four findings. Details are in ")
	link, err := intro.AddInternalLink("sec_findings", docx.HyperlinkOptions{Tooltip: "Jump to the findings table"})
	if err != nil {
		return nil, fmt.Errorf("internal link: %w", err)
	}
	link.AddRun("section 2")
	intro.AddRun("; the scoring method follows ")
	ext, err := intro.AddHyperlink("https://www.first.org/cvss/", docx.HyperlinkOptions{Tooltip: "CVSS specification"})
	if err != nil {
		return nil, fmt.Errorf("external link: %w", err)
	}
	ext.AddRun("CVSS")
	intro.AddRun(".")

	// Section 2: formatted table with a repeating header row.
	h2 := body.AddParagraph()
	h2.Format().SetStyle("Heading1")
	h2.AddRun("2. Findings")
	if _, err := h2.AddBookmark("sec_findings"); err != nil {
		return nil, fmt.Errorf("bookmark: %w", err)
	}

	tbl := body.AddTable(len(findings)+1, 3, docx.TableOptions{
		Style:    "TableGrid",
		ColWidth: []docx.Twip{docx.Inches(0.8), docx.Inches(4.2), docx.Inches(1.0)},
	})
	tbl.SetWidth(docx.WidthPercent(docx.Percent(100)))
	tbl.SetLayout(docx.LayoutFixed)
	edge := &docx.Border{Style: docx.BorderSingle, Width: 4, Color: "808080"}
	tbl.SetBorders(docx.BorderOptions{
		Top: edge, Left: edge, Bottom: edge, Right: edge, InsideH: edge, InsideV: edge,
	})
	cellPad := docx.Pt(4)
	tbl.SetCellMargins(docx.CellMargins{Top: &cellPad, Bottom: &cellPad})

	header := []string{"ID", "Finding", "Severity"}
	var rows []*docx.Row
	for r := range tbl.Rows() {
		rows = append(rows, r)
	}
	rows[0].SetHeader(true)
	rows[0].SetCantSplit(docx.On())
	for i, c := range cellsOf(rows[0]) {
		c.SetShading("1F3864")
		run := cellPara(c).AddRun(header[i])
		run.Font().SetBold(docx.On())
		run.Font().SetColor("FFFFFF")
	}
	for i, f := range findings {
		row := rows[i+1]
		row.SetCantSplit(docx.On())
		cells := cellsOf(row)
		cellPara(cells[0]).AddRun(f.id)
		cellPara(cells[1]).AddRun(f.title)
		cellPara(cells[2]).AddRun(f.severity)
		cells[2].SetShading(severityFill[f.severity])
	}

	// Section 3: inline image with alt text and a caption.
	h3 := body.AddParagraph()
	h3.Format().SetStyle("Heading1")
	h3.AddRun("3. Severity distribution")

	pic := body.AddParagraph()
	pic.Format().SetAlignment(docx.AlignCenter)
	data, err := sampleChart()
	if err != nil {
		return nil, err
	}
	if _, err := pic.AddRun("").AddImage(bytes.NewReader(data), docx.ImageOptions{
		Width:   docx.Inches(4).EMU(),
		AltText: "Bar chart: one high, two medium and one low severity finding",
	}); err != nil {
		return nil, fmt.Errorf("image: %w", err)
	}
	caption := body.AddParagraph()
	caption.Format().SetStyle("Caption")
	caption.Format().SetAlignment(docx.AlignCenter)
	caption.AddRun("Figure 1. Findings by severity")

	// Footer: Page X of Y.
	footer := d.LastSection().SetFooter(docx.FooterDefault)
	fp := footer.AddParagraph()
	fp.Format().SetAlignment(docx.AlignCenter)
	fp.AddRun("Page ")
	fp.AddPageNumber()
	fp.AddRun(" of ")
	fp.AddPageCount()

	// Ask Word to refresh the TOC and page numbers on open.
	d.SetUpdateFieldsOnOpen(true)
	return d, nil
}

// cellPara returns the first paragraph of a cell. New cells always have one.
func cellPara(c *docx.Cell) *docx.Paragraph {
	for p := range c.Paragraphs() {
		return p
	}
	return c.AddParagraph()
}

// cellsOf returns the cells of a row in order.
func cellsOf(r *docx.Row) []*docx.Cell {
	var out []*docx.Cell
	for c := range r.Cells() {
		out = append(out, c)
	}
	return out
}

// sampleChart draws a small bar chart so the example needs no image file.
func sampleChart() ([]byte, error) {
	const w, h = 320, 160
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.White)
		}
	}
	bars := []struct {
		height int
		col    color.RGBA
	}{
		{120, color.RGBA{R: 0xC0, G: 0x39, B: 0x2B, A: 0xFF}},
		{80, color.RGBA{R: 0xE6, G: 0xA2, B: 0x3C, A: 0xFF}},
		{40, color.RGBA{R: 0x4C, G: 0xA6, B: 0x5B, A: 0xFF}},
	}
	for i, b := range bars {
		x0 := 40 + i*90
		for y := h - 20 - b.height; y < h-20; y++ {
			for x := x0; x < x0+60; x++ {
				img.Set(x, y, b.col)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode chart: %w", err)
	}
	return buf.Bytes(), nil
}
