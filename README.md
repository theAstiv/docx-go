# docx-go

A permissively-licensed Go library for creating, reading, and modifying `.docx`
(OOXML WordprocessingML) files — without losing unknown content.

**Status**: Phase 3a (report essentials) is implemented: inline images, hyperlinks, bookmarks, table formatting, editable styles, and page-number/TOC fields join the Phase 2 feature set. The new features are covered by round-trip, real-fixture and fuzz tests; validating the output with the OOXML validator and opening it in Word, LibreOffice and Google Docs is still pending (Phase 3 exit gate). Not yet stable for production use (API may change before v1.0).

---

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
| Paragraphs and runs | `Body.AddParagraph()`, `Paragraph.AddRun()`, `Paragraph.Text()` |
| Paragraph formatting | `ParagraphFormat` — alignment, indents, spacing, line spacing, OnOff flags |
| Run formatting | `RunFont` — bold, italic, size, color, underline, highlight, vert-align |
| Tables | `Body.AddTable(rows, cols, opts)`, `Row`, `Cell`, merged cells (`SetSpan`, `SetVMerge`) |
| Numbered lists | `Document.CreateList(opts)`, `Paragraph.SetList(list, level)`, `List.RestartAt(lvl, start)` |
| Headers and footers | `Section.SetHeader/Footer(type)`, default/first/even variants |
| Multi-section documents | `Paragraph.AddSectionBreak()`, `Document.LastSection()` |
| Document settings | `EvenAndOddHeaders`, `DefaultTabStop` |
| Unknown-content preservation | All unmodelled elements pass through byte-for-byte |

**Not yet supported**: templating (Phase 3b), comments, tracked changes, SVG and anchored images, multi-paragraph bookmarks, resolving the full style inheritance chain.

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

