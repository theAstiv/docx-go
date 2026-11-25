package opc

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// ErrPartName is returned (wrapped) by Open when a ZIP entry name is not a
// valid part name (ECMA-376 Part 2, 9.1.1.1: absolute, holding a
// backslash, an empty, "." or ".." segment, or a segment ending in a dot),
// or when two entries name the same part, exactly or ignoring ASCII case.
var ErrPartName = errors.New("opc: invalid part name")

// checkPartName reports whether name, a ZIP entry name, is a valid part name.
func checkPartName(name string) error {
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsRune(name, '\\') {
		return fmt.Errorf("%w: %q", ErrPartName, name)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.HasSuffix(seg, ".") {
			return fmt.Errorf("%w: %q", ErrPartName, name)
		}
	}
	return nil
}

// IsPartName reports whether name (unrooted, as parts are stored) may name an
// ordinary part: a valid part name that is not a relationships part, a
// "_rels" folder, or the content types stream, which the package writes
// itself.
func IsPartName(name string) bool {
	return checkPartName(name) == nil && !isRelsFile(name) &&
		!strings.EqualFold(path.Base(name), "_rels") &&
		!strings.EqualFold(name, contentTypesName)
}

// isRelsFile reports whether name is a relationships part: a file named
// "<source>.rels" in a "_rels" folder. Other files ending in ".rels" are
// ordinary parts.
func isRelsFile(name string) bool {
	dir, file := path.Split(name)
	return strings.HasSuffix(strings.ToLower(file), ".rels") &&
		strings.EqualFold(path.Base(strings.TrimSuffix(dir, "/")), "_rels") && dir != ""
}

// relsPathToPartPath inverts relsPath: it returns the path of the part a
// relationships part belongs to.
//
//	"_rels/.rels"                  → ""
//	"_rels/x.xml.rels"             → "x.xml"
//	"word/_rels/document.xml.rels" → "word/document.xml"
func relsPathToPartPath(relsName string) string {
	dir, file := path.Split(relsName) // dir is "<parent>_rels/" (isRelsFile)
	source := file[:len(file)-len(".rels")]
	parent := dir[:len(dir)-len("_rels/")]
	return parent + source
}
