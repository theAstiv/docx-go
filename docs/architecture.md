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
   │                           the document's stories, image creation
   └── internal/template/      engine over []wml.BlockElement
```

The engine works on WordprocessingML block trees only, so it cannot reach the
package graph.  `docx/template` gets the document's stories (body, headers,
footers) through `docx/internal/bridge`, which `docx` fills in at init time.
Being internal to `docx/`, the bridge adds nothing to the public API.

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
