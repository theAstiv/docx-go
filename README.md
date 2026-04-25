# docx-go

A permissively-licensed Go library for creating, reading, and modifying `.docx`
(OOXML WordprocessingML) files — without losing unknown content.

**Status**: Phase 0 bootstrap — not yet usable.

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

TBD (MIT / Apache-2.0 / BSD-3 — pending decision).

---

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md).
Minimum Go version: **1.23** (required for `iter.Seq` / range-over-func).

