package registry

import (
	"encoding/xml"
	"fmt"
	"sync"
)

// Element is the interface every oxml type must satisfy.  MarshalXML and
// UnmarshalXML use the standard encoding/xml signatures so the registry
// integrates with the stdlib decoder without adapters.
type Element interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
}

// factory creates a zero-value Element of a specific registered type.
type factory func() Element

var (
	mu     sync.RWMutex
	table  = map[xml.Name]factory{}
	sealed bool
)

// Register associates qname with a factory function.  It must be called from
// package init() functions only.  Calling Register after the first call to New
// panics to prevent races.
func Register(qname xml.Name, f factory) {
	mu.Lock()
	defer mu.Unlock()
	if sealed {
		panic(fmt.Sprintf("registry: Register called after first use for %v", qname))
	}
	if _, dup := table[qname]; dup {
		panic(fmt.Sprintf("registry: duplicate registration for %v", qname))
	}
	table[qname] = f
}

// New returns a fresh Element for qname, or nil if qname is not registered.
// The first call to New seals the registry against further registrations.
func New(qname xml.Name) Element {
	mu.RLock()
	f, ok := table[qname]
	if !ok {
		mu.RUnlock()
		sealOnce()
		return nil
	}
	mu.RUnlock()
	sealOnce()
	return f()
}

// Registered returns the set of all registered xml.Names.  Primarily for the
// round-trip audit.
func Registered() []xml.Name {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]xml.Name, 0, len(table))
	for qn := range table {
		out = append(out, qn)
	}
	return out
}

func sealOnce() {
	mu.Lock()
	sealed = true
	mu.Unlock()
}
