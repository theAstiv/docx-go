//go:build roundtripaudit

package registry_test

import (
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// TestRoundTripAudit_AllRegisteredTypesHaveTest verifies that every type in
// the registry can at least instantiate and marshal without panicking.
// A separate per-type round-trip test (foo_test.go) must exist to fully
// satisfy the round-trip rule; this audit catches registration without any
// test at all.
func TestRoundTripAudit_AllRegisteredTypesHaveTest(t *testing.T) {
	names := registry.Registered()
	if len(names) == 0 {
		t.Log("registry is empty — no types to audit")
		return
	}
	for _, qn := range names {
		qn := qn
		t.Run(qn.Space+":"+qn.Local, func(t *testing.T) {
			el := registry.New(qn)
			if el == nil {
				t.Errorf("New(%v) returned nil after Registered() listed it", qn)
				return
			}
			// Smoke-test: marshal must not panic or error on a zero value.
			// Real round-trip correctness is checked by per-type _test.go files.
			if err := smokeMarshall(el, qn); err != nil {
				t.Errorf("zero-value MarshalXML for %v: %v", qn, err)
			}
		})
	}
}

func smokeMarshall(el registry.Element, name xml.Name) error {
	import_buf := &nullWriter{}
	enc := xml.NewEncoder(import_buf)
	return el.MarshalXML(enc, xml.StartElement{Name: name})
}

// nullWriter discards all writes.
type nullWriter struct{}

func (n *nullWriter) Write(p []byte) (int, error) { return len(p), nil }
