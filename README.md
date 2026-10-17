# docx-go

A permissively-licensed Go library for creating, reading, and modifying `.docx`
(OOXML WordprocessingML) files — without losing unknown content.

**Status**: Phase 2 complete — paragraphs, runs, tables, lists, headers/footers, and multi-section documents are fully supported. Not yet stable for production use (API may change before v1.0).

---

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

**Not yet supported**: images (Phase 3), hyperlinks, bookmarks, comments, fields, full style editing.

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

