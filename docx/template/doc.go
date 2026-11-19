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
// {{#if}} and {{#unless}}, with or without {{else}}, also work inside one
// paragraph: "Dear {{#if vip}}valued {{/if}}customer" keeps or removes the
// text between the tags, and the paragraph stays. The removed text may hold
// tabs, breaks, inline pictures, whole hyperlinks, whole fields and whole
// content controls, but not part of a field, one end of a bookmark or
// comment, or a footnote, endnote or comment reference
// (ErrSplitAcrossBoundary). Removing a bookmark that a cross-reference or
// internal link points at is a warning. {{#if}} … {{/if}} with no
// {{else}} that fills a one-cell row on its own is a row range, as before:
// the row is kept or removed. {{#each}} cannot repeat part of a paragraph.
//
// {{image path}} places a picture inline, in the tag's run, from a
// template.Image value (PNG, JPEG or GIF bytes, an optional size, and the
// required alt text). It works in the body, tables, headers and footers and
// inside loops (each copy gets its own drawing id; identical bytes are
// stored once), but not in content controls or text boxes. Adding an image
// changes the package, so Execute records it first and puts it back if
// execution fails: a failed Execute still leaves the document unchanged.
//
// A value tag can pipe its value through formatters, left to right:
//
//	{{name | upper}}                     UPPER CASE (lower: lower case)
//	{{note | default "—"}}               "—" when note is missing, null or ""
//	{{total | number 2}}                 1,234.50; number 2 "." "," gives 1.234,50
//	{{total | currency "€" 2}}           €1,234.50 (decimals default to 2)
//	{{due | date "2 Jan 2006"}}          an RFC 3339 date or time, in Go's layout
//
// Arguments are quoted strings or numbers; Word's curly quotes count as
// quotes. number and currency round half away from zero on the decimal
// value, so 2.675 gives 2.68. Only default accepts a missing key; any other
// formatter leaves it missing. Options.Funcs adds formatters, or replaces a
// built-in other than default. An unknown formatter or a wrong argument to a
// built-in fails in both modes; a value a formatter rejects is ErrFormat.
//
// This package is part of the public proxy layer: it imports no encoding/xml.
// The engine lives in internal/template.
package template
