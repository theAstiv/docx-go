# Architecture

## Three-layer model

```
┌─────────────────────────────────────────┐
│  docx/                                  │  Public proxy layer
│  Paragraph, Run, Font, Table, …         │  No encoding/xml
└────────────────┬────────────────────────┘
                 │ refs
┌────────────────▼────────────────────────┐
│  internal/parts/                        │  Part + package layer
│  DocumentPart, StylesPart, …            │  OPC graph, ZIP
└────────────────┬────────────────────────┘
                 │ refs
┌────────────────▼────────────────────────┐
│  internal/oxml/                         │  Element layer
│  CT_P, CT_R, CT_PPr, CT_RPr, …         │  XSD mirror
│  MarshalXML / UnmarshalXML              │
└─────────────────────────────────────────┘
```

### `internal/oxml/` — element layer

Pure XSD mirror.  One file per OOXML complex type.  Owns `MarshalXML` /
`UnmarshalXML`.  Knows nothing about files, ZIPs, or the public API.

Sub-packages:

| Package | Purpose |
|---|---|
| `wml/` | WordprocessingML `CT_*` types |
| `dml/` | DrawingML `CT_*` types |
| `common/` | Shared simple types (`OnOff`, measures) |
| `ns/` | Namespace URI → prefix table; `Qn()` helper |
| `registry/` | `Register` / `New` dispatcher |
| `raw/` | `RawElement` token-stream passthrough |
| `order/` | `InsertAt`, `RemoveAt`, `FindByQName` |
| `roundtrip/` | Generic `Verify[T]` test helper |
| `internal/nsfix/` | `encoding/xml` namespace bug workaround |

### `internal/parts/` — part + package layer

Owns the OPC graph: which parts exist, their content types, their
relationships, the ZIP.  One type per part (`DocumentPart`, `StylesPart`,
`HeaderPart`, …).

### `internal/opc/` — OPC primitives

ZIP read/write, `[Content_Types].xml`, `.rels` files.  Imports only stdlib.

### `docx/` — public proxy layer

Holds refs to oxml elements via parts.  Property-style getters/setters.
**Zero `encoding/xml` imports** — enforced by `forbidigo` lint.

---

## Extensibility primitives

### Element registry

```go
registry.Register(ns.Qn("w14:foo"), func() registry.Element { return &CT_Foo{} })
```

Adding a new element = one struct + one `Register` call.  No core changes.

### Unknown-element passthrough (`RawElement`)

Every container holds:

```go
Extras []*raw.RawElement
```

`RawElement` captures the token stream on unmarshal and replays it on
marshal.  This closes the class of "open, save unchanged, file is now broken"
bugs found in godocx.

### Ordered-insertion helpers

The schema is `xs:sequence`.  Per-`CT_*` schema-position maps live next to
each type:

```go
// in internal/oxml/wml/ppr.go
var pPrChildOrder = map[xml.Name]int{
    ns.Qn("w:pStyle"):        0,
    ns.Qn("w:keepNext"):      1,
    // ...
}
```

`order.InsertAt` uses these maps to insert children at the correct position.

---

## Round-trip safety rule

Every `oxml` type ships **all three** in the same PR:

1. `MarshalXML`
2. `UnmarshalXML`
3. A round-trip test (`roundtrip.Verify[T]`)

CI enforces this with `go test -tags=roundtripaudit`.

---

## Import boundary enforcement

`depguard` in `.golangci.yml` machine-enforces:

- `internal/opc/**` must not import `internal/oxml`, `internal/parts` or `docx`
- `internal/oxml/**` must not import `internal/parts`, `internal/opc` or `docx`
- `docx/**` may import `internal/parts` and `internal/oxml/wml` (element
  construction only); it must not import any other `internal/oxml`
  sub-package, `internal/opc` or `encoding/xml`
- `internal/template/**` must not import `internal/parts`, `internal/opc` or
  `docx` (see [Templating](#templating))

`forbidigo` additionally bans `encoding/xml` from `docx/**`.

Without these guards the architecture is social, not mechanical.  Each rule
is checked by planting a violation: a glob that matches no files fails
silently (the `docx/` rule once never fired because `**/docx/**/*.go` needs a
directory below `docx/`).

---

## Templating

```
docx/template/          public API: Execute, Options, Issue, Error
   │
   ├── docx/internal/bridge/   function variables set by docx in init():
   │                           the document's stories, the unparsed note parts
   └── internal/template/      engine over []wml.BlockElement
```

The engine works on WordprocessingML block trees only, so it cannot reach the
package graph.  `docx/template` gets the document's stories (body, headers,
footers) through `docx/internal/bridge`, which `docx` fills in at init time.
Being internal to `docx/`, the bridge adds nothing to the public API.

`Execute` runs in four steps per story: copy the blocks (`wml.Clone`),
normalise split tags, expand blocks and fill values, then swap the copies in.
Any error-severity issue skips the swap, so a failed call leaves the
document unchanged.

### Design decisions

| Decision | Why |
|---|---|
| Syntax `{{path}}`, `{{#each}}`, `{{#if}}`, `{{#unless}}`; `{{.field}}` and `{{.}}` inside loops; paths without a leading `.` always start at the root data | Familiar to template authors; one unambiguous lookup rule |
| Data is `map[string]any`, JSON-shaped; no struct reflection | Matches `json.Unmarshal` output |
| Values are plain text: written through the encoder (escaped), `\n` → `w:br`, `\t` → `w:tab`, numbers as written (`strconv.FormatFloat(v, 'f', -1, 64)`) | No markup injection; JSON numbers are `float64` and must not print as `3e+00` |
| Block tags sit alone in a paragraph, or at row edges: open at the start of a row's first cell, close at the end of the last cell of the same or a later row | Keeps the template row readable in Word with no throwaway marker rows. Anything else is `ErrMisplacedBlock` |
| A tag split across runs collapses into the run holding `{{` (its formatting wins). `w:proofErr` and `w:lastRenderedPageBreak` inside a tag are dropped and `_GoBack` is moved before it; any other boundary (field, bookmark, hyperlink edge, content control, tracked change) is `ErrSplitAcrossBoundary` | These are the splits Word actually produces. Merging only moves text between `w:t` nodes, so paragraph text never changes |
| `Strict` (default) collects every missing key into one `*template.Error`; `Lenient` writes `""` and warns. Structural template errors fail in both modes. A missing `#if` key is false; a missing `#each` list is a missing key | A broken template is a template bug; an absent optional section is normal data, but a mistyped list name must not silently print nothing |
| Copies made by `{{#each}}` drop `w14:paraId`/`w14:textId` and bookmarks (the first copy keeps them), get fresh `wp:docPr` ids, and lose comment anchors (with a warning). The fix-ups run as a token filter inside `wml.CloneWith`, so they reach raw content too | Duplicate bookmark and drawing ids are known causes of Word's "unreadable content" warning; duplicate paraIds are invalid |
| In content Word stores raw (content controls, text boxes, tracked changes), each group of sibling `w:p`, and the runs of an inline container such as a paragraph-level content control, is parsed into typed paragraphs, merged and filled like body text, then written back. Block tags there fail. Footnotes, endnotes and comments are unparsed: `Execute` warns if they contain `{{` | Word's own cover pages showed that tags there are split as freely as in the body (`{{` + `report.title` + `}}` in three runs); the round-trip rule makes typing and writing back lossless |
| Value tags in the parts data-bound content controls read from (`docProps/core.xml`, `customXml/item*.xml`) are filled too, from the root data. Only text nodes change; every other byte is copied | Typing a tag into a bound control (a cover page's Title) makes Word store it in the bound part as well, and Word re-syncs the control from that part on open: filling only the visible text would show the tag again. Re-encoding these parts with `encoding/xml` would mangle their prefixes |
| Text inside a table of contents (a `TOC`, `INDEX` or `TOA` field's result, tracked across paragraphs) is not template text: not filled, not reported. `Execute` warns when such a TOC holds tags and the document does not ask Word to update fields on open | A TOC repeats every heading verbatim, so a heading `{{.id}}` inside a loop also appears above the loop, where it cannot resolve. Word rebuilds the TOC from the filled headings when fields update |
| A cell keeps a closing paragraph only if it had one before expansion; an emptied table is removed (warning) | `w:tc` needs a block and Word wants it to end with a paragraph, but a cell that already ended with a nested table must stay as it was |
| `CT_P` and `CT_R` keep their attributes (`w:rsid*`, `w14:paraId`) | Word templates carry them on nearly every element; dropping them broke "unknown content round-trips untouched" |

---

## OPC package graph

```
[Content_Types].xml     ← extension + per-part content types
_rels/.rels             ← package-level relationships
word/document.xml       ← root document part
word/_rels/document.xml.rels
  → word/styles.xml     (styles)
  → word/settings.xml   (settings)
  → word/numbering.xml  (numbering)
  → word/header1.xml    (header)
  → word/footer1.xml    (footer)
  → word/media/image1.* (image)       [Phase 3]
```
