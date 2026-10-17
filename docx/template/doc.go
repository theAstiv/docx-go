// Package template fills a .docx template from data.
//
// A template is an ordinary Word document containing placeholders:
//
//	{{client.name}}                     a value (dotted paths into nested maps)
//	{{#each findings}} … {{/each}}      repeat paragraphs or table rows per item
//	{{#if flag}} … {{/if}}              keep the block only when flag is truthy
//	{{#unless flag}} … {{/unless}}      keep the block only when flag is falsy
//
// Inside {{#each}}, {{.field}} refers to a field of the current item and
// {{.}} to the item itself. Data is JSON-shaped (map[string]any).
//
// This package is part of the public proxy layer: it imports no encoding/xml.
// The engine lives in internal/template.
package template
