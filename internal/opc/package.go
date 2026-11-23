package opc

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Package is an OPC package: a ZIP archive containing parts, a
// [Content_Types].xml, and .rels relationship files.
type Package struct {
	// ContentTypes is the parsed content-type registry.
	ContentTypes *ContentTypes

	// parts holds raw XML (or binary) bytes keyed by part path.
	parts map[string][]byte

	// rels holds relationship sets keyed by the source part path.
	// "" is the package-level relationship set (_rels/.rels).
	rels map[string]*Rels

	// badRels holds, by source part path, the relationships parts that
	// could not be parsed. Their bytes stay in parts under their own name,
	// so a part this package never reads keeps them as they were.
	badRels map[string]badRels
}

type badRels struct {
	name string // the relationships part's own name
	err  error
}

// NewPackage creates an empty Package with sensible defaults.
func NewPackage() *Package {
	return &Package{
		ContentTypes: &ContentTypes{},
		parts:        make(map[string][]byte),
		rels:         map[string]*Rels{"": {}},
		badRels:      map[string]badRels{},
	}
}

// Open reads an OPC package from r.
func Open(r io.ReaderAt, size int64) (*Package, error) {
	raw, err := readZip(r, size, defaultLimits)
	if err != nil {
		return nil, fmt.Errorf("opc: open zip: %w", err)
	}

	pkg := NewPackage()

	// Parse [Content_Types].xml.
	ctData, ok := raw[contentTypesName]
	if !ok {
		return nil, fmt.Errorf("opc: missing %s", contentTypesName)
	}
	pkg.ContentTypes, err = parseContentTypes(ctData)
	if err != nil {
		return nil, err
	}
	delete(raw, contentTypesName)

	// Separate .rels files from content parts.
	for name, data := range raw {
		if isRelsFile(name) {
			partPath := relsPathToPartPath(name)
			rels, err := parseRels(data)
			if err != nil {
				// The package's own relationships are needed to find
				// anything; others only matter for parts that are read
				// (see RelsErr).
				if partPath == "" {
					return nil, fmt.Errorf("opc: parse rels %s: %w", name, err)
				}
				pkg.parts[name] = data
				pkg.badRels[partPath] = badRels{name: name, err: fmt.Errorf("opc: parse rels %s: %w", name, err)}
				continue
			}
			pkg.rels[partPath] = rels
		} else {
			pkg.parts[name] = data
		}
	}

	// Ensure root rels exists.
	if pkg.rels[""] == nil {
		pkg.rels[""] = &Rels{}
	}

	return pkg, nil
}

// OpenBytes is a convenience wrapper around Open for in-memory data.
func OpenBytes(data []byte) (*Package, error) {
	return Open(bytes.NewReader(data), int64(len(data)))
}

// Save writes the package to w.
func (p *Package) Save(w io.Writer) error {
	entries := make([]zipEntry, 0, len(p.parts)+len(p.rels)+1)

	// [Content_Types].xml
	ctData, err := p.ContentTypes.marshal()
	if err != nil {
		return fmt.Errorf("opc: marshal content types: %w", err)
	}
	entries = append(entries, zipEntry{Name: contentTypesName, Data: ctData})

	// Content parts.
	for name, data := range p.parts {
		entries = append(entries, zipEntry{Name: name, Data: data})
	}

	// Relationship files.
	for partPath, rels := range p.rels {
		if len(rels.entries) == 0 {
			continue
		}
		data, err := rels.marshal()
		if err != nil {
			return fmt.Errorf("opc: marshal rels for %q: %w", partPath, err)
		}
		entries = append(entries, zipEntry{Name: relsPath(partPath), Data: data})
	}

	return writeZip(w, entries)
}

// Part returns the raw bytes for a part by its path, or nil if absent.
func (p *Package) Part(path string) []byte {
	return p.parts[path]
}

// FindPart returns the stored name of the part whose name equals name
// ignoring ASCII case, as OPC part names compare (ECMA-376 Part 2, 9.1.1.1),
// and whether there is one. An exact match wins over a case variant.
func (p *Package) FindPart(name string) (string, bool) {
	if _, ok := p.parts[name]; ok {
		return name, true
	}
	for stored := range p.parts {
		if strings.EqualFold(stored, name) {
			return stored, true
		}
	}
	return "", false
}

// SetPart stores raw bytes for a part.
func (p *Package) SetPart(path string, data []byte) {
	p.parts[path] = data
}

// DeletePart removes a part and its relationships.
func (p *Package) DeletePart(path string) {
	delete(p.parts, path)
	delete(p.rels, path)
}

// Rels returns the relationship set for a source part path.
// "" means the package-level (_rels/.rels).  A new empty set is created on
// first access.
//
// The source is matched ignoring ASCII case, as part names compare. A set
// that could not be parsed (RelsErr) is replaced by the new empty one.
func (p *Package) Rels(partPath string) *Rels {
	if r, ok := p.rels[partPath]; ok {
		return r
	}
	for src, r := range p.rels {
		if strings.EqualFold(src, partPath) {
			return r
		}
	}
	if bad, ok := p.badRels[partPath]; ok {
		delete(p.parts, bad.name)
		delete(p.badRels, partPath)
	}
	r := &Rels{}
	p.rels[partPath] = r
	return r
}

// RelsErr returns the error that parsing the relationships of the part at
// partPath gave, or nil. Open keeps a malformed relationships part as
// opaque bytes instead of failing, since most parts are never read; a
// caller that reads partPath must check this first.
func (p *Package) RelsErr(partPath string) error {
	for src, bad := range p.badRels {
		if strings.EqualFold(src, partPath) {
			return bad.err
		}
	}
	return nil
}

// PartPaths returns all content part paths (excludes .rels and content-types).
func (p *Package) PartPaths() []string {
	out := make([]string, 0, len(p.parts))
	for name := range p.parts {
		out = append(out, name)
	}
	return out
}

// RelsParts returns the source part paths for every non-empty relationship set
// in the package.  "" means the package-level set (_rels/.rels).
func (p *Package) RelsParts() []string {
	out := make([]string, 0, len(p.rels))
	for partPath, r := range p.rels {
		if len(r.entries) > 0 {
			out = append(out, partPath)
		}
	}
	return out
}
