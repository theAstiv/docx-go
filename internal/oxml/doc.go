// Package oxml is the element layer of docx-go.
//
// It is a pure XSD mirror: one file per OOXML complex type, each owning
// MarshalXML / UnmarshalXML.  This package knows nothing about files, ZIPs,
// or the public API.
//
// Sub-packages:
//   - wml/    WordprocessingML CT_* types
//   - dml/    DrawingML CT_* types
//   - common/ shared simple types (OnOff, measures)
//   - ns/     namespace URI → prefix table
//   - registry/ element registration / dispatch
//   - raw/    RawElement token-stream passthrough
//   - order/  ordered-insertion helpers
//   - roundtrip/ generic round-trip test helper
//
// Architecture: internal/oxml ← internal/parts ← docx
package oxml
