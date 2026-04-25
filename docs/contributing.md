# Contributing (detailed guide)

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for the short version.

---

## Development setup

```bash
git clone https://github.com/theAstiv/docx-go
cd docx-go
go test ./...                              # baseline
go test -tags=roundtripaudit ./...         # round-trip audit
golangci-lint run                          # linters
```

Optional (pre-commit hooks):

```bash
# Install lefthook
go install github.com/evilmartians/lefthook@latest
lefthook install
```

---

## Adding a new oxml type

1. Create `internal/oxml/wml/mynewtype.go`.  Name the file after the spec's
   `CT_*` lowercased (e.g. `CT_MyNewType` → `mynewtype.go`).

2. Implement `MarshalXML` and `UnmarshalXML`.  All `EncodeToken` /
   `EncodeElement` return values must be checked.

3. Define `CT_MyNewType`'s schema-position map next to the struct:

   ```go
   var myNewTypeChildOrder = map[xml.Name]int{
       ns.Qn("w:child1"): 0,
       ns.Qn("w:child2"): 1,
   }
   ```

4. Create `internal/oxml/wml/mynewtype_test.go` with a `roundtrip.Verify[*CT_MyNewType]` call.

5. Register the type:

   ```go
   func init() {
       registry.Register(ns.Qn("w:myNewType"), func() registry.Element {
           return &CT_MyNewType{}
       })
   }
   ```

6. Run `go test -tags=roundtripaudit ./...` — CI will fail if step 4 is missing.

---

## Adding a proxy in `docx/`

Proxy structs hold a pointer to the underlying `oxml` type.  They never import
`encoding/xml` directly.

```go
// docx/mything.go
type MyThing struct {
    ct *wml.CT_MyNewType  // via internal/parts
}

func (m *MyThing) DoSomething() { … }
```

Use `docx.On()` / `docx.Off()` / `docx.Inherit()` for tri-state booleans and
the named unit types (`Twip`, `HalfPoint`, …) for measurements.

---

## Round-trip canonicalisation

The test oracle and `cmd/docxdiff` use the same canonicaliser
(`internal/canon`).  Canonicalisation is **structural only**:

- Attribute order is sorted.
- Namespace prefixes are rewritten to canonical forms.
- Text-node bytes are **not** normalised.

This means a test that passes `canon.Equal(a, b)` will also show as equal in
`scripts/docx-diff.sh`, and vice versa.  Never use `xmllint --c14n11` for
comparison — it normalises whitespace, which would mask text-corruption bugs.

---

## Fixture corpus

See `testdata/fixtures/README.md` for provenance.  When adding a fixture:

1. Place it under `testdata/fixtures/{word|libreoffice|gdocs}/`.
2. Run `go test -update ./...` to capture golden snapshots.
3. Review the diff in `testdata/golden/xml/` before committing.
