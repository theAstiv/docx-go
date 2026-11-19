package opc

import (
	"maps"
	"slices"
)

// Snapshot is a copy of a package's parts, relationships and content types,
// taken by Package.Snapshot and put back by Package.Restore.
//
// Part bytes are never changed in place (SetPart replaces them), so the
// copy shares them and costs one map copy plus the relationship entries.
type Snapshot struct {
	contentTypes ContentTypes
	parts        map[string][]byte
	rels         map[string]Rels
}

// Snapshot records the package's current state.
func (p *Package) Snapshot() *Snapshot {
	s := &Snapshot{
		contentTypes: copyContentTypes(p.ContentTypes),
		parts:        maps.Clone(p.parts),
		rels:         make(map[string]Rels, len(p.rels)),
	}
	for k, r := range p.rels {
		s.rels[k] = Rels{entries: slices.Clone(r.entries), nextID: r.nextID}
	}
	return s
}

// Restore puts the package back to the state s recorded, discarding parts,
// relationships and content types added or changed since. The
// *ContentTypes and the *Rels of every relationship set that existed then
// keep their identity, so pointers callers already hold stay valid.
func (p *Package) Restore(s *Snapshot) {
	*p.ContentTypes = copyContentTypes(&s.contentTypes)
	p.parts = maps.Clone(s.parts)
	for k := range p.rels {
		if _, ok := s.rels[k]; !ok {
			delete(p.rels, k)
		}
	}
	for k, r := range s.rels {
		restored := Rels{entries: slices.Clone(r.entries), nextID: r.nextID}
		if cur, ok := p.rels[k]; ok {
			*cur = restored
		} else {
			p.rels[k] = &restored
		}
	}
}

func copyContentTypes(ct *ContentTypes) ContentTypes {
	if ct == nil {
		return ContentTypes{Defaults: nil, Overrides: nil}
	}
	return ContentTypes{Defaults: slices.Clone(ct.Defaults), Overrides: slices.Clone(ct.Overrides)}
}
