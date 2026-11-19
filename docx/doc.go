// Package docx is the public proxy layer of docx-go.
//
// It exposes an ergonomic, property-style API for creating, reading, and
// modifying .docx documents.  This package intentionally imports no
// encoding/xml: all XML work is delegated to internal/parts and
// internal/oxml.  It uses internal/oxml/wml for element construction only;
// no other internal/oxml sub-package is imported (enforced by depguard).
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
//
// # Iterators and removal
//
// Iterators such as [Body.Paragraphs] and [Table.Rows] walk the elements
// present when iteration starts, so removing the element just yielded, or
// any other, inside the loop is safe: every element is still visited once.
// An element removed before the loop reaches it is still yielded, and its
// Remove reports [ErrNotInDocument]. A proxy whose element, or whose
// container, was removed refuses Remove with ErrNotInDocument, and the new
// content from its Insert methods is not attached to the document.
package docx
