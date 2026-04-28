package order

import "encoding/xml"

// Element is the minimal interface that any oxml element must satisfy for
// FindByQName to work.  It mirrors the registry.Element interface but is
// redeclared here to avoid an import cycle.
type Element interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
}

// Named is an optional interface for elements that expose their XML name.
// FindByQName uses it when available.
type Named interface {
	XMLName() xml.Name
}

// InsertAt inserts item into *slice at position index, shifting later
// elements right.  If index >= len(*slice) the item is appended.
func InsertAt[T any](slice *[]T, index int, item T) {
	s := *slice
	if index >= len(s) {
		*slice = append(s, item)
		return
	}
	s = append(s, item) // grow by one
	copy(s[index+1:], s[index:])
	s[index] = item
	*slice = s
}

// RemoveAt removes the element at position index from *slice.  No-op if
// index is out of range.
func RemoveAt[T any](slice *[]T, index int) {
	s := *slice
	if index < 0 || index >= len(s) {
		return
	}
	*slice = append(s[:index], s[index+1:]...)
}

// FindByQName returns the index of the first element in slice whose XMLName()
// matches name.  Elements that do not implement Named are skipped.
// Returns -1, false when not found.
func FindByQName(slice []Element, name xml.Name) (int, bool) {
	for i, el := range slice {
		if n, ok := el.(Named); ok {
			if n.XMLName() == name {
				return i, true
			}
		}
	}
	return -1, false
}
