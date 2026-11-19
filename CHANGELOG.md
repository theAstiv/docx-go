# Changelog

docx-go follows [semantic versioning](https://semver.org/). **Before 1.0, the public API may change between minor versions.** Requires Go ≥ 1.23.

## Unreleased

### Breaking

- Templating: `{{else}}` is now a keyword. A data key named `else` can no longer be written as `{{else}}`; inside a loop, `{{.else}}` still reads it.
- Templating: template errors no longer depend on the data. `Execute` now checks every branch for unbalanced or misplaced block tags, section breaks inside blocks and malformed tags in content controls, including blocks the data drops and loops over empty lists. A template with such an error in a dropped block used to succeed; it now fails in both modes. Values in branches that are not rendered are still not required. An error repeated by a loop is reported once.

### Added

- Templating: `{{else}}` in `{{#if}}`, `{{#unless}}` and `{{#each}}`. It goes alone in a paragraph, or at the start of a row's first cell in a row range, where it begins the rows of the else branch. In `{{#each}}`, the else branch shows once, with the outer data, when the list is empty, `null` or missing; without an `{{else}}`, a missing list is still a missing key. `Placeholders` reports it with kind `"else"`.
- Templating: formatters. A value tag pipes its value through them, left to right: `{{total | currency "€" 2}}`, `{{due | date "2 Jan 2006"}}`, `{{name | upper | default "—"}}`.
  - Built in: `upper`, `lower`, `default "x"` (for a missing key, `null` or `""`), `number N ["thousands" "decimal"]`, `currency "symbol" [N ["thousands" "decimal"]]` and `date "layout"` (RFC 3339 input, Go layout).
  - Arguments are quoted strings or numbers. Word's curly quotes count as quotes.
  - `number` and `currency` round half away from zero on the decimal value, so `2.675` gives `2.68`.
  - `Options.Funcs` adds formatters, or replaces built-ins other than `default` (for example, a `number` with local digit grouping). A panic in one is reported as an error.
  - New sentinels: `ErrUnknownFormatter` (fails in both modes, even in a branch the data drops) and `ErrFormat` (a value a formatter rejects: an error in Strict mode, a warning in Lenient mode). A wrong argument to a built-in is `ErrMalformedTag`.
  - `Placeholder.Formatters` lists a tag's formatter names.

## v0.4.0: editing existing documents

Open a document, then remove and insert content, find and replace text, read its plain text and set its properties. Templates can be filled without writing Go, using the new `docxfill` command.

Every document this release generates passes the OOXML schema validator and opens without a "Repaired document" prompt in LibreOffice (checked in CI). Word and Google Docs were checked by hand. [`examples/edit`](examples/edit) edits the report that [`examples/report`](examples/report) builds.

### Added

- `Document.Text()`, `Body.Text()`, `Header.Text()` and `Footer.Text()` return a story's plain text. Every paragraph ends with `\n`. In a table, cells are separated by `\t` and each row ends with `\n`, and a cell's paragraphs are joined with `\n`. Headers and footers are not part of `Document.Text()`. Text in text boxes is not included yet.
- `Document.CoreProperties()` reads and writes the core document properties (`docProps/core.xml`): `Title`, `Subject`, `Author`, `Keywords`, `Description`, `Category`, `LastModifiedBy`, `Revision`, `Created` and `Modified`.
  - Setting a property on a document without the part creates it. An empty string or a zero time removes the property.
  - A document whose properties are not changed keeps `core.xml` byte-for-byte.
  - Properties this API does not name are kept.
  - `Modified` and `LastModifiedBy` are not updated automatically on save.
- Paragraph tab stops, borders and shading on `ParagraphFormat`, so they work for paragraphs and paragraph styles alike:
  - Tab stops: `TabStops`, `SetTabStops`, `AddTabStop`, `RemoveTabStop` and `ClearTabStop`, with `TabAlign*` alignments and `TabLeader*` fill characters. Stops are written sorted by position. `ClearTabStop` cancels a stop inherited from the style.
  - Borders: `SetBorders(ParagraphBorderOptions{Top, Left, Bottom, Right, Between, Bar})`, `Borders` and `ClearBorders`.
  - Shading: `SetShading(fill)` and `Shading`.
- Structural editing of existing documents:
  - Removal: `Paragraph.Remove()`, `Table.Remove()`, `Row.Remove()` and `Run.Remove()`. They return `ErrNotInDocument` (already removed), `ErrSectionBreak` (the paragraph carries a section break), `ErrUnbalancedField` (the content holds only part of a field, such as the start of a table of contents) or `ErrLastRow`.
  - A removal also removes the other end of any bookmark or comment range it cuts.
  - A cell, header or footer always keeps a final paragraph.
  - Removing the row where a vertical merge starts passes the start to the row below.
  - Insertion: `Paragraph.InsertParagraphBefore/After(text)`, `Table.InsertParagraphBefore/After(text)`, `Paragraph.InsertTableAfter(rows, cols, opts)` and `Table.InsertRow(i, opts)`.
  - `InsertRow` copies the neighbouring row's cell structure, widths, borders, shading and paragraph formatting, and extends a vertical merge it is inserted into.
- Find and replace: `Document.Replace(old, new, ReplaceOptions)` and `Paragraph.Replace`.
  - Text that Word split across runs (spell-check marks, formatting or revision changes) is found. The replacement takes the formatting of the run holding the match's first character.
  - `\n` and `\t` in the replacement become a line break and a tab.
  - Options: `IgnoreCase`, `SkipHeaders`, `SkipFooters` and `Limit`.
  - The result reports `Count`, and in `Skipped` the matches that could not be replaced: those split by a field, bookmark or hyperlink edge, those inside content controls or tracked insertions, and those inside a table of contents. Field results are replaced; field instructions are not searched.
- `cmd/docxfill`: fill a template from JSON on the command line.
  - `docxfill [-lenient] -o out.docx template.docx data.json`, where `data.json` may be `-` for stdin and `-o -` writes to stdout.
  - `docxfill -placeholders template.docx` lists the template's tags.
  - Warnings go to stderr. Exit status is 2 for template or data errors (each one listed), and 1 for usage or file errors.
  - The output is written only on success, never partially.

### Fixes

- A table created without a style (`AddTable(…, TableOptions{})`) had no `w:tblPr`, which the schema requires. It now always has one.
- `TableOptions.ColWidth` was written only on the table grid. Word lays out an autofit table from the cell widths, so a table whose cells were still empty collapsed to a sliver (LibreOffice was not affected). The width is now also written on every cell of the column (`w:tcW`).

### Behaviour changes

- `Paragraph.Text()` (and `Hyperlink.Text()`) now include text inside content the library keeps unparsed: tracked insertions (`w:ins`), smart tags, inline content controls, simple fields and custom XML. Tracked deletions (`w:del`, `w:moveFrom`) are still excluded. Before, all of this text was silently dropped.
- `Cell.Text()` now includes nested tables (laid out as in `Body.Text()`) and content controls in the cell.
- Templating: whether a paragraph is a block tag (`{{#each}}`, `{{/if}}`, …) is now decided on its whole visible text, including text in tracked insertions and content controls. A block tag typed with track changes on now works. A block-tag paragraph that also holds other inserted text used to be removed along with that text; it is now reported as a misplaced block tag.

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
- **Elements in no namespace:** an unknown element in no namespace inside a namespaced one is written with `xmlns=""`. Before, it fell into its parent's namespace on save, and on the next open it was read back as a different element.
- **Faster many-image reports:** adding many images or bookmarks no longer rescans the whole document for each one.

## v0.2.0: tables, lists, sections, headers and footers

First tagged release. Licensed under Apache-2.0.

- Paragraphs and runs with paragraph and run formatting.
- Tables with merged cells.
- Numbered and bulleted lists with restart.
- Multi-section documents with default, first-page and even-page headers and footers.
- Document settings (`EvenAndOddHeaders`, `DefaultTabStop`).
- Unmodelled content is kept on round-trip: same names, attributes, child order and text. Namespace prefixes may be re-canonicalised.
