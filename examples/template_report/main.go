// Command template_report fills a branded .docx template from JSON data: the
// templating workflow behind a generated assessment report.
//
// Usage:
//
//	go run ./examples/template_report [-template t.docx] [-data d.json] [output.docx]
//
// Without flags it uses the template.docx and data.json embedded in the
// example. The template's syntax:
//
//	{{client.name}}                          a value
//	{{assessment.date | date "2 January 2006"}}  a value through a formatter
//	{{#each areas}} … {{/each}}              a loop over table rows or paragraphs
//	{{#if .critical}} … {{else}} … {{/if}}   a conditional block
//	{{#each gaps}} … {{else}} … {{/each}}    a loop with a block for an empty list
//	{{#if .critical}} (critical){{/if}}      a condition inside a paragraph
//	{{image client.logo}}                    a picture
//	{{summary.verdict}}                      formatted text, from a template.Rich value
//
// JSON cannot hold pictures or formatted text, so fill adds client.logo and
// summary.verdict to the data in Go (see addGoValues).
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

//go:embed template.docx
var defaultTemplate []byte

//go:embed data.json
var defaultData []byte

func main() {
	tplPath := flag.String("template", "", "template .docx (default: the embedded example)")
	dataPath := flag.String("data", "", "JSON data file (default: the embedded example)")
	flag.Parse()
	out := "template-report.docx"
	if flag.NArg() > 0 {
		out = flag.Arg(0)
	}

	tpl, data := defaultTemplate, defaultData
	var err error
	if *tplPath != "" {
		if tpl, err = os.ReadFile(*tplPath); err != nil { // #nosec G304 -- the path is chosen by the user running the example.
			log.Fatal(err)
		}
	}
	if *dataPath != "" {
		if data, err = os.ReadFile(*dataPath); err != nil { // #nosec G304 -- the path is chosen by the user running the example.
			log.Fatal(err)
		}
	}

	doc, warnings, err := fill(tpl, data)
	for _, w := range warnings {
		log.Print(w) // Issue.String starts with its severity
	}
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(out) // #nosec G304 -- the output path is chosen by the user running the example.
	if err != nil {
		log.Fatal(err)
	}
	if err := doc.Save(f); err != nil {
		_ = f.Close()
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Written %s", out)
}

// fill opens the template and fills it from the JSON data.
func fill(tpl, data []byte) (*docx.Document, []template.Issue, error) {
	doc, err := docx.OpenBytes(tpl)
	if err != nil {
		return nil, nil, fmt.Errorf("open template: %w", err)
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, nil, fmt.Errorf("read data: %w", err)
	}
	if err := addGoValues(values); err != nil {
		return nil, nil, err
	}
	// Word then refreshes page numbers and any table of contents from the
	// filled text when the report opens. A TOC repeats heading tags, which
	// Execute leaves for Word to rebuild.
	doc.SetUpdateFieldsOnOpen(true)
	// Strict mode: a key missing from the data is an error, listed with
	// every other one, and the template is left as it was.
	warnings, err := template.Execute(doc, values, template.Options{Mode: template.Strict})
	if err != nil {
		return nil, warnings, fmt.Errorf("fill template: %w", err)
	}
	return doc, warnings, nil
}

// addGoValues adds the values JSON cannot express: the client's logo (a
// template.Image) and a coloured verdict with a link (a template.Rich). A
// value the data already holds is left alone; a missing client or summary
// object is left for Strict mode to report.
func addGoValues(values map[string]any) error {
	if client, ok := values["client"].(map[string]any); ok {
		if _, set := client["logo"]; !set {
			logo, err := clientLogoPNG()
			if err != nil {
				return err
			}
			client["logo"] = template.Image{
				Data:    logo,
				Width:   docx.Inches(0.9).EMU(), // the height follows the aspect ratio
				AltText: "Client logo",
			}
		}
	}
	if summary, ok := values["summary"].(map[string]any); ok {
		if _, set := summary["verdict"]; !set {
			score, _ := summary["score"].(float64)
			maxScore, _ := summary["max"].(float64)
			summary["verdict"] = verdict(score, maxScore)
		}
	}
	return nil
}

// verdict turns the readiness score into bold, coloured text with a link to
// the regulator's site.
func verdict(score, maxScore float64) template.Rich {
	label, colour := "Ready", "1E8449"
	switch {
	case maxScore <= 0 || score/maxScore < 0.5:
		label, colour = "Not ready", "C0392B"
	case score/maxScore < 0.8:
		label, colour = "Partly ready", "B9770E"
	}
	return template.Rich{Runs: []template.RichRun{
		{Text: label, Bold: docx.On(), Color: colour},
		{Text: " (see the "},
		{Text: "Ministry's data protection pages", Link: "https://www.meity.gov.in/"},
		{Text: ")"},
	}}
}
