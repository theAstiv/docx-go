// Package docx is the public proxy layer of docx-go.
//
// It exposes an ergonomic, property-style API for creating, reading, and
// modifying .docx documents.  This package intentionally imports neither
// encoding/xml nor any internal/oxml sub-package directly — all XML work is
// delegated to internal/parts and internal/oxml through typed proxies.
//
// Architecture: docx → internal/parts → internal/oxml
package docx
