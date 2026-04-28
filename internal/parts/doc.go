// Package parts is the OPC part-graph layer of docx-go.
//
// It owns which parts exist in a package, their content types, their
// relationships, and the ZIP container.  One Go type per part (Document,
// Numbering, Styles, Header, Footer, Image, Theme).  This layer knows nothing
// about the public ergonomic API.
//
// Architecture: internal/parts → internal/oxml, internal/opc
package parts
