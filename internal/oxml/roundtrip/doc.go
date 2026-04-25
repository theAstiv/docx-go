// Package roundtrip provides the generic round-trip test helper Verify[T].
// It marshals a value, unmarshals the bytes back, marshals again, and asserts
// the two marshalled forms are byte-equal under structural canonicalisation.
package roundtrip
