package template_test

import (
	"fmt"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// BenchmarkExecute fills a report-shaped template: values, a paragraph
// loop and a row loop over 500 items.
func BenchmarkExecute(b *testing.B) {
	items := make([]any, 500)
	for i := range items {
		items[i] = map[string]any{"name": fmt.Sprintf("Item %d", i), "qty": i}
	}
	data := map[string]any{"title": "Report", "items": items}
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		doc := docx.New()
		paras(doc, "{{title}}", "{{#each items}}", "{{.name}}: {{.qty}}", "{{/each}}")
		table(doc, [][]string{{"{{#each items}}{{.name}}", "{{.qty}}{{/each}}"}})
		b.StartTimer()
		if _, err := template.Execute(doc, data, template.Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
