// Package ns is the single source of truth for all OOXML namespace URIs and
// their canonical prefixes.  Every package that needs a Clark-notation
// xml.Name should call ns.Qn("w:foo") rather than constructing names inline.
package ns
