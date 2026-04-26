// Package docx is the public proxy layer of docx-go.
//
// It exposes an ergonomic, property-style API for creating, reading, and
// modifying .docx documents.  This package intentionally imports neither
// encoding/xml nor any internal/oxml sub-package directly — all XML work is
// delegated to internal/parts and internal/oxml through typed proxies.
//
// Architecture: docx → internal/parts → internal/oxml
//
// # Thread safety
//
// No type in this package is safe for concurrent use without external
// synchronization.  A [Document] and all proxy objects derived from it
// (Body, Paragraph, Run, ParagraphFormat, RunFont) share mutable state.
// Callers that need concurrent access must guard the entire Document with a
// sync.Mutex or similar.
package docx
