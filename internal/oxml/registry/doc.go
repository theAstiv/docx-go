// Package registry is the element registration and dispatch layer.
//
// Calling Register(qname, factory) at init time teaches the unmarshaller how
// to construct the right Go type for a given XML tag.  New element types
// require only one Register call — no changes to core parsing logic.
package registry
