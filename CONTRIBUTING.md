# Contributing to docx-go

## Round-trip rule (non-negotiable)

Every `oxml` type **must ship all three of these in the same PR**:

1. `MarshalXML`
2. `UnmarshalXML`
3. A round-trip test — `marshal → unmarshal → marshal → byte-equal under canonicalisation`

CI enforces this via the `roundtripaudit` build tag.  A PR that registers a new
type without a round-trip test **will not merge**.

## Lint rules

```
golangci-lint run          # full suite (see .golangci.yml)
go vet ./...
staticcheck ./...
```

Key rules enforced in CI:

| Rule | What it prevents |
|---|---|
| `forbidigo` | `encoding/xml` imports in `docx/` |
| `depguard` | Layer leaks (oxml importing parts, docx importing oxml internals) |
| `exhaustruct` | Forgetting a field when a CT struct grows |
| `errcheck` | Silently swallowed `MarshalXML` errors |

## PR checklist

- [ ] All existing tests pass (`go test -race -count=1 ./...`)
- [ ] Round-trip audit passes (`go test -tags=roundtripaudit ./...`)
- [ ] Lint clean (`golangci-lint run`)
- [ ] New `oxml` types have `MarshalXML`, `UnmarshalXML`, and a round-trip test
- [ ] No `encoding/xml` imports added to `docx/`
- [ ] No layer imports violate the three-layer boundary

## Commit style

```
type(scope): short description

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`.
