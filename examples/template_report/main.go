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
//	{{client.name}}                 a value
//	{{#each areas}} … {{/each}}     a loop over table rows or paragraphs
//	{{#if .critical}} … {{/if}}     a conditional block
//	{{#unless gaps}} … {{/unless}}  the reverse
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
		log.Printf("warning: %s", w)
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
	// Strict mode: a key missing from the data is an error, listed with
	// every other one, and the template is left as it was.
	warnings, err := template.Execute(doc, values, template.Options{Mode: template.Strict})
	if err != nil {
		return nil, warnings, fmt.Errorf("fill template: %w", err)
	}
	return doc, warnings, nil
}
