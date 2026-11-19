// Package template fills a .docx template from data.
//
// A template is an ordinary Word document containing placeholders:
//
//	{{client.name}}                     a value (dotted paths into nested maps)
//	{{#each findings}} … {{/each}}      repeat paragraphs or table rows per item
//	{{#if flag}} … {{/if}}              keep the block only when flag is truthy
//	{{#unless flag}} … {{/unless}}      keep the block only when flag is falsy
//	{{#if flag}} … {{else}} … {{/if}}   the else branch replaces the block when it is dropped
//
// Inside {{#each}}, {{.field}} refers to a field of the current item and
// {{.}} to the item itself. Data is JSON-shaped (map[string]any).
//
// {{else}} works in all three blocks. In {{#each}} it renders once, with the
// outer data, when the list is empty, null or (only when an else is present)
// missing. It goes where the other block tags go: alone in a paragraph, or
// at the start of a row's first cell, where it starts the rows of the else
// branch. "else" is a keyword: inside a loop, write {{.else}} for a field
// with that name.
//
// This package is part of the public proxy layer: it imports no encoding/xml.
// The engine lives in internal/template.
package template
