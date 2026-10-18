# docx-go

A permissively-licensed Go library for creating, reading, and modifying `.docx`
(OOXML WordprocessingML) files — without losing unknown content.

**Status**: Phase 3a (report essentials) is implemented: inline images, hyperlinks, bookmarks, table formatting, editable styles, and page-number/TOC fields join the Phase 2 feature set. Phase 3b adds templating: fill a Word template from JSON data. The new features are covered by round-trip, real-fixture and fuzz tests; opening the output in Word, LibreOffice and Google Docs is still pending (Phase 3 exit gate). Not yet stable for production use (API may change before v1.0).

---

## Templating (Phase 3b)

Write placeholders in an ordinary Word document, then fill it from JSON-shaped data with package [`docx/template`](docx/template):

```go
doc, _ := docx.OpenBytes(templateBytes)
var data map[string]any
_ = json.Unmarshal(jsonBytes, &data)
warnings, err := template.Execute(doc, data, template.Options{Mode: template.Strict})
```

| Tag | Meaning |
|---|---|
| `{{client.name}}` | A value; dotted paths reach into nested objects. Numbers print as written (`3`, `4.5`); line breaks and tabs in values become breaks and tabs |
| `{{#each items}}` … `{{/each}}` | Repeat once per item. Inside, `{{.field}}` is a field of the item and `{{.}}` the item itself |
| `{{#if flag}}` … `{{/if}}` | Keep the block when `flag` is truthy (not missing, `false`, `""`, `0`, empty or `null`) |
| `{{#unless flag}}` … `{{/unless}}` | Keep the block when `flag` is falsy |

Block tags go **alone in their own paragraph** (the paragraphs and tables between the two tags repeat or disappear) or **at the edges of table rows**: `{{#each rows}}` at the start of a row's first cell and `{{/each}}` at the end of the last cell of the same or a later row repeats those rows.

- Placeholders work in the body, tables, headers and footers, and (values only) in content controls and text boxes, such as Word's built-in cover pages. Tags Word split across runs (spell-check marks, formatting changes) are rejoined; the formatting of the run holding `{{` is used.
- Data-bound content controls (a cover page's Title, Subtitle, Date) show values stored in the document properties or custom XML; those are filled too, so Word shows the filled value when it opens the file.
- A table of contents repeats the headings' tags; it is left for Word to rebuild. Call `doc.SetUpdateFieldsOnOpen(true)` so Word refreshes the TOC and page numbers when the document opens (`Execute` warns if a TOC holds tags and you haven't).
- Tested against templates made in Word itself (`testdata/fixtures/word/template_*.docx`).
- `Strict` (the default) returns a `*template.Error` listing every missing key and every malformed tag, and leaves the document unchanged. `Lenient` writes missing values as empty text and returns warnings. A missing key in `{{#if}}` is simply false.
- `template.Placeholders(doc)` lists a template's tags, so data can be checked against a template before rendering.
- Repeated content gets fresh drawing ids, keeps bookmarks on the first copy only, and keeps list numbering continuous.
- Not supported: `{{else}}`, helpers, conditions inside a paragraph, rich-text values, and tags in footnotes, endnotes and comments (`Execute` warns about those).

See [`examples/template_report`](examples/template_report) for a gap-assessment report built from a template and [`data.json`](examples/template_report/data.json).

## What works (Phase 3a)

| Feature | API surface |
|---|---|
| Inline images | `Run.AddImage(r, ImageOptions)` (PNG/JPEG/GIF), `Run.Images()`, `Image.Size/SetSize/AltText/Bytes`; identical images are stored once |
| Hyperlinks | `Paragraph.AddHyperlink(url, opts)`, `AddInternalLink(bookmark, opts)`, `Hyperlinks()`, `Hyperlink.AddRun/URL/Anchor/Text` |
| Bookmarks | `Paragraph.AddBookmark(name)`, `Document.Bookmark(name)`, `Document.Bookmarks()` |
| Styles | `Document.Styles()`, `Styles.ByID/All/AddParagraphStyle/AddCharacterStyle/AddTableStyle/EnsureBuiltin/DocDefaultsFont`, `Style.Format()/Font()/SetBasedOn/SetNext` |
| Table formatting | `Table/Cell.SetBorders(BorderOptions)`, `SetWidth(TableWidth)`, `Table.SetCellMargins/SetLayout`, `Cell.SetShading`, `Row.SetHeader/SetCantSplit` |
| Fields | `Paragraph.AddField/AddPageNumber/AddPageCount/Fields()`, `Document.AddTOC(opts)`, `Document.SetUpdateFieldsOnOpen(bool)` |

See [`examples/report`](examples/report) for a report that uses all of them. SVG and floating (anchored) images are not supported yet; anchored images in existing documents are preserved unchanged.

## What works (Phase 2)

| Feature | API surface |
|---|---|
| Paragraphs and runs | `Body/Cell/Header/Footer.AddParagraph()` (appends), `Cell/Header/Footer.SetText(s)` (one paragraph; returns it), `Paragraph.AddRun()`, `Paragraph.Text()`. A new cell, header or footer holds one empty paragraph: fill it with `SetText`, or `AddParagraph` adds a second |
| Paragraph formatting | `ParagraphFormat` — alignment, indents, spacing, line spacing, OnOff flags |
| Run formatting | `RunFont` — bold, italic, size, color, underline, highlight, vert-align |
| Tables | `Body.AddTable(rows, cols, opts)`, `Row`, `Cell`, merged cells (`SetSpan`, `SetVMerge`) |
| Numbered lists | `Document.CreateList(opts)`, `Paragraph.SetList(list, level)`, `List.RestartAt(lvl, start)` |
| Headers and footers | `Section.SetHeader/Footer(type)`, default/first/even variants |
| Multi-section documents | `Paragraph.AddSectionBreak()`, `Document.LastSection()` |
| Document settings | `EvenAndOddHeaders`, `DefaultTabStop` |
| Unknown-content preservation | All unmodelled elements pass through byte-for-byte |

**Not yet supported**: comments, tracked changes, SVG and anchored images, multi-paragraph bookmarks, resolving the full style inheritance chain.

---

## Mission

1. Round-trip any valid OOXML `.docx` with zero data loss (modelled or not).
2. Expose property-style proxies for the modelled subset.
3. Produce output that Word, LibreOffice, and Google Docs open without "Repaired Document" warnings.

The closest Go alternatives (`unioffice`, `fumiama/go-docx`) are AGPL or commercial.
This library fills the gap left by `python-docx` in the Go ecosystem.

---

## Architecture

```
docx/             ← public proxy layer   (no encoding/xml)
  ↓
internal/parts/   ← OPC / part graph layer
  ↓
internal/oxml/    ← XSD mirror (MarshalXML / UnmarshalXML)
```

See [`CLAUDE.md`](./CLAUDE.md) for design decisions and [`/.claude/PLAN.md`](./.claude/PLAN.md) for the phased task breakdown.

---

## License

Apache License 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).

---

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md).
Minimum Go version: **1.23** (required for `iter.Seq` / range-over-func).

