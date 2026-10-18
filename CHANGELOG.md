# Changelog

docx-go follows [semantic versioning](https://semver.org/). **Before 1.0, the public API may change between minor versions.** Requires Go ≥ 1.23.

## v0.3.0: report building and templating

Everything needed to generate a professional report: from scratch with the API, or by filling a Word template from JSON data.

Every document this release generates, and every template it fills, passes the OOXML schema validator and opens without a "Repaired document" prompt in Word, LibreOffice and Google Docs. LibreOffice is checked in CI on every change; the Word and Google Docs checks were done by hand.

### Templating: new package `docx/template`

Write placeholders in an ordinary Word document, then fill it:

```go
warnings, err := template.Execute(doc, data, template.Options{Mode: template.Strict})
```

- **Tags:** `{{client.name}}` values with dotted paths, `{{#each}}` loops, `{{#if}}` and `{{#unless}}` blocks. Inside a loop, `{{.field}}` is a field of the current item and `{{.}}` is the item itself. Loops and blocks nest.
- **Where block tags go:** alone in their own paragraph, so the paragraphs and tables between the two tags repeat or disappear; or at the edges of table rows, so the rows repeat.
- **Where tags work:** body, tables, headers and footers, plus content controls and text boxes, such as Word's built-in cover pages.
- **Word's run splits:** a tag that Word stored in several pieces, because of spell-check marks, formatting changes or revision ids, is rejoined using the formatting of the piece that holds `{{`.
- **Data-bound content controls** (a cover page's Title or Date) show values stored in the document properties or custom XML. Those are filled too, so Word shows the filled value when it opens the file.
- **Tables of contents** repeat heading text, tags included. They are left for Word to rebuild: call `doc.SetUpdateFieldsOnOpen(true)`. `Execute` warns if a TOC holds tags and you haven't.
- **Modes and errors:** `Strict` (the default) returns one `*template.Error` that lists every missing key and malformed tag, with its location, and leaves the document unchanged. `Lenient` writes missing values as empty text and returns warnings.
- **Checking a template:** `template.Placeholders(doc)` lists a template's tags, so data can be checked against it before rendering.
- **Repeated content** gets fresh drawing ids and keeps bookmarks on the first copy only, so Word sees no duplicates.
- **Tested** against templates made in Word itself.

Not supported yet: `{{else}}`, helpers, conditions inside a paragraph, rich-text and image values, and tags in footnotes, endnotes and comments (`Execute` warns about those).

### Report building

| Feature | API |
|---|---|
| Inline images | `Run.AddImage(r, ImageOptions)` (PNG, JPEG, GIF), `Run.Images()`, `Image.Size/SetSize/AltText/Bytes`; identical images are stored once |
| Hyperlinks | `Paragraph.AddHyperlink(url, opts)`, `AddInternalLink(bookmark, opts)`, `Hyperlinks()`, `Hyperlink.AddRun/URL/Anchor/Text` |
| Bookmarks | `Paragraph.AddBookmark(name)`, `Document.Bookmark(name)`, `Document.Bookmarks()` |
| Styles | `Document.Styles()`, `Styles.ByID/All/AddParagraphStyle/AddCharacterStyle/AddTableStyle/EnsureBuiltin`, `Style.Format()/Font()/SetBasedOn/SetNext`, document defaults |
| Table formatting | `Table/Cell.SetBorders(BorderOptions)`, `SetWidth(TableWidth)`, `Table.SetCellMargins/SetLayout`, `Cell.SetShading`, `Row.SetHeader/SetCantSplit` |
| Fields | `Paragraph.AddField/AddPageNumber/AddPageCount/Fields()`, `Document.AddTOC(opts)`, `Document.SetUpdateFieldsOnOpen(bool)`; `Fields()` also finds fields that span paragraphs, such as a Word-made TOC |
| Cell, header and footer text | `Cell/Header/Footer.SetText(s)` replaces the content with one paragraph and returns it |

Examples: [`examples/report`](examples/report) builds a report with every feature; [`examples/template_report`](examples/template_report) fills a template from JSON.

### Behaviour changes

Check these when upgrading from v0.2.0:

- **`Cell`, `Header` and `Footer.AddParagraph` now append**, like `Body.AddParagraph`. Before, they inserted before the last paragraph. A new cell, header or footer holds one empty paragraph: fill it with `SetText`, or `AddParagraph` adds a second one.
- **`Header` and `Footer.AddTable` append** the table, followed by an empty paragraph, because Word wants a header or footer to end with one.
- **Properties are written in schema order.** Paragraph, run and section properties are sorted into the order the schema requires when saved. Input that was out of order is normalised; valid input is unchanged.
- **Left and right indents** are written as `w:left` / `w:right`, which every Word version accepts. `w:start` / `w:end` are still read.
- **`Styles.EnsureBuiltin`** returns the document's existing equivalent style when there is one, such as `Standard` in a localized template instead of a second `Normal`. Apply the returned `Style.ID()`, not the constant.
- **`AddBookmark`, `AddHyperlink`, `AddInternalLink` and `AddTOC` return an `error`** for invalid names, URLs or options.

### Fixes

- **Revision ids kept:** paragraphs and runs keep their `w:rsid*` and `w14:paraId` attributes on save. Before, an unchanged save dropped them.
- **Headers and footers:** header and footer references are written before the page size and margins, as the schema requires. Every v0.2.0 document with a header or footer was affected.
- **Table rows:** `w:tblPrEx` is written before `w:trPr`.
- **Unique ids:** drawing and bookmark ids stay unique across the whole document, including inside content controls, tracked changes and text boxes. Duplicates are a cause of Word's "unreadable content" warning.
- **Empty defaults:** an empty `w:rPrDefault` / `w:pPrDefault` is written back as it was.
- **Faster many-image reports:** adding many images or bookmarks no longer rescans the whole document for each one.

## v0.2.0: tables, lists, sections, headers and footers

First tagged release. Licensed under Apache-2.0.

- Paragraphs and runs with paragraph and run formatting.
- Tables with merged cells.
- Numbered and bulleted lists with restart.
- Multi-section documents with default, first-page and even-page headers and footers.
- Document settings (`EvenAndOddHeaders`, `DefaultTabStop`).
- Unmodelled content is kept on round-trip: same names, attributes, child order and text. Namespace prefixes may be re-canonicalised.
