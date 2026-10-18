package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/theAstiv/docx-go/docx"
)

// buildTemplate builds the template that template.docx holds. A real
// template is drawn in Word by a designer; this one is built in code so the
// example needs no binary it cannot regenerate. TestTemplateUpToDate keeps
// the committed file in step (go test ./examples/template_report -update).
//
// It shows every placement the engine supports:
//   - value tags in the body, the header and a table;
//   - a row loop ({{#each}} at the edges of a table row);
//   - a paragraph loop with a nested {{#if}} and a nested loop;
//   - {{#unless}} for an empty list;
//   - a static logo in the header and page numbers in the footer, which
//     pass through untouched.
func buildTemplate() (*docx.Document, error) {
	d := docx.New()
	body := d.Body()
	styles := d.Styles()
	for _, s := range []docx.BuiltinStyle{
		docx.StyleNormal, docx.StyleTitle, docx.StyleHeading1, docx.StyleHeading2, docx.StyleTableGrid,
	} {
		styles.EnsureBuiltin(s)
	}
	styles.ByID("Heading1").Font().SetColor("1F3864")
	styles.DocDefaultsFont().SetSizePt(11)

	// Header: static logo and the client's name.
	header := d.LastSection().SetHeader(docx.HeaderDefault)
	hp := header.SetText("")
	logo, err := logoPNG()
	if err != nil {
		return nil, err
	}
	if _, err := hp.AddRun("").AddImage(bytes.NewReader(logo), docx.ImageOptions{
		Height:  docx.Inches(0.3).EMU(),
		AltText: "Assessor logo",
	}); err != nil {
		return nil, fmt.Errorf("logo: %w", err)
	}
	hp.AddRun("  {{client.name}} · DPDPA gap assessment")

	footer := d.LastSection().SetFooter(docx.FooterDefault)
	fp := footer.SetText("")
	fp.Format().SetAlignment(docx.AlignCenter)
	fp.AddRun("Confidential · page ")
	fp.AddPageNumber()
	fp.AddRun(" of ")
	fp.AddPageCount()

	para := func(style, text string) *docx.Paragraph {
		p := body.AddParagraph()
		if style != "" {
			p.Format().SetStyle(style)
		}
		p.AddRun(text)
		return p
	}

	para("Title", "DPDPA Gap Assessment")
	para("", "Prepared for {{client.name}}, {{client.city}}")
	para("", "Assessment date: {{assessment.date}} · Assessor: {{assessment.assessor}}")

	para("Heading1", "Summary")
	p := para("", "Overall readiness: ")
	score := p.AddRun("{{summary.score}}")
	score.Font().SetBold(docx.On())
	p.AddRun(" of {{summary.max}}")
	para("", "{{summary.text}}")

	// Scorecard: a header row, then one row per area.
	para("Heading1", "Scorecard")
	tbl := body.AddTable(2, 3, docx.TableOptions{
		Style:    "TableGrid",
		ColWidth: []docx.Twip{docx.Inches(3.2), docx.Inches(1.8), docx.Inches(1.0)},
	})
	tbl.SetWidth(docx.WidthPercent(docx.Percent(100)))
	rows := collect(tbl.Rows())
	rows[0].SetHeader(true)
	for i, c := range collect(rows[0].Cells()) {
		c.SetShading("1F3864")
		run := c.SetText("").AddRun([]string{"Area", "Status", "Score"}[i])
		run.Font().SetBold(docx.On())
		run.Font().SetColor("FFFFFF")
	}
	cells := collect(rows[1].Cells())
	cells[0].SetText("{{#each areas}}{{.name}}")
	cells[1].SetText("{{.status}}")
	cells[2].SetText("{{.score}}{{/each}}")

	// Gaps: one section per gap, each with its own bulleted actions.
	para("Heading1", "Gaps")
	para("", "{{#unless gaps}}")
	para("", "No gaps were found.")
	para("", "{{/unless}}")
	actions := d.CreateList(docx.ListOptions{Style: docx.ListBullet})
	para("", "{{#each gaps}}")
	para("Heading2", "{{.id}} · {{.title}}")
	para("", "Provision: {{.provision}}")
	para("", "{{.finding}}")
	para("", "{{#if .critical}}")
	warn := para("", "")
	r := warn.AddRun("Critical: fix before processing resumes.")
	r.Font().SetBold(docx.On())
	r.Font().SetColor("C0392B")
	para("", "{{/if}}")
	para("", "Recommended actions:")
	para("", "{{#each .actions}}")
	para("", "{{.}}").SetList(actions, 0)
	para("", "{{/each}}")
	para("", "{{/each}}")

	para("Heading1", "Next steps")
	para("", "{{next_steps}}")
	return d, nil
}

func collect[T any](seq func(func(T) bool)) []T {
	var out []T
	for v := range seq {
		out = append(out, v)
	}
	return out
}

// logoPNG draws a small two-colour mark so the example needs no image file.
func logoPNG() ([]byte, error) {
	const w, h = 64, 64
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 0x1F, G: 0x38, B: 0x64, A: 0xFF}
			if x > y {
				c = color.RGBA{R: 0x4C, G: 0xA6, B: 0x5B, A: 0xFF}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode logo: %w", err)
	}
	return buf.Bytes(), nil
}
