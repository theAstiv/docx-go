## What and why

<!-- What this changes, and the problem it fixes. Link the issue: "Fixes #123". -->

## Checklist

- [ ] Tests pass: `go test -race -count=1 ./...`
- [ ] Round-trip audit passes: `go test -tags=roundtripaudit ./...`
- [ ] Lint clean: `gofmt -l .` prints nothing; `go vet ./...`, `staticcheck ./...` and `golangci-lint run` pass
- [ ] New `oxml` types have `MarshalXML`, `UnmarshalXML` and a round-trip test
- [ ] No `encoding/xml` imports in `docx/`, no imports across the three-layer boundary
- [ ] Changes to fixtures or part marshalling: `bash scripts/validate-corpus.sh --delta` adds no new errors
- [ ] User-visible change: entry added to `CHANGELOG.md`

## Checked in

<!-- Optional for refactors. For changes to what docx-go writes, say which apps
     opened the result without a "Repaired document" prompt. -->

- [ ] Word
- [ ] LibreOffice
- [ ] Google Docs
