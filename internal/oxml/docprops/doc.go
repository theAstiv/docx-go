// Package docprops holds the element types of the document property parts:
// cp:coreProperties (docProps/core.xml, OPC part 2 §11).
//
// It sits in the oxml element layer beside wml and dml: it owns
// MarshalXML / UnmarshalXML and knows nothing about the package. The parts
// layer (internal/parts) loads, creates and saves the part.
package docprops
