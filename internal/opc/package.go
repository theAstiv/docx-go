package opc

import (
	"bytes"
	"fmt"
	"io"
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
}

// NewPackage creates an empty Package with sensible defaults.
func NewPackage() *Package {
	return &Package{
		ContentTypes: &ContentTypes{},
		parts:        make(map[string][]byte),
		rels:         map[string]*Rels{"": {}},
	}
}

// Open reads an OPC package from r.
func Open(r io.ReaderAt, size int64) (*Package, error) {
	raw, err := readZip(r, size)
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
				return nil, fmt.Errorf("opc: parse rels %s: %w", name, err)
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
func (p *Package) Rels(partPath string) *Rels {
	if r, ok := p.rels[partPath]; ok {
		return r
	}
	r := &Rels{}
	p.rels[partPath] = r
	return r
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

// isRelsFile returns true if name looks like a .rels file.
func isRelsFile(name string) bool {
	return len(name) > 5 && name[len(name)-5:] == ".rels"
}

// relsPathToPartPath inverts relsPath: converts a .rels archive path back to
// the part path it belongs to.
//
//	"_rels/.rels"                  → ""
//	"word/_rels/document.xml.rels" → "word/document.xml"
func relsPathToPartPath(relsName string) string {
	// Strip the trailing ".rels" extension.
	base := relsName[:len(relsName)-5]
	// base is e.g. "word/_rels/document.xml" or "_rels/"
	// Find and remove the "_rels/" component.
	const marker = "/_rels/"
	idx := findSuffix(base, "/_rels/")
	if idx < 0 {
		// Package-level: "_rels/.rels" → base is "_rels/" → ""
		if base == "_rels/" {
			return ""
		}
		return base
	}
	dir := base[:idx]
	file := base[idx+len(marker):]
	if file == "" {
		return dir
	}
	if dir == "" {
		return file
	}
	return dir + "/" + file
}

func findSuffix(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
