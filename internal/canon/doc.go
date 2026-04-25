// Package canon provides structural-only XML canonicalisation for golden tests
// and the docxdiff tool.
//
// Canonicalisation operations:
//   - Sort attributes lexicographically by Clark-notation xml.Name.
//   - Rewrite namespace prefixes to the canonical mapping in internal/oxml/ns.
//   - Drop redundant namespace declarations.
//   - Sort [Content_Types].xml entries.
//   - Sort *.rels entries by Id.
//   - Sort ZIP entries by archive path; use epoch-0 mtimes.
//
// Text-node content (xml.CharData) is preserved byte-for-byte.  Whitespace
// between elements is not collapsed.  The test oracle and the human diff tool
// use the same canonicaliser so they always agree.
package canon
