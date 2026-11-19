package parts

import (
	"crypto/sha256"
	"fmt"
	"maps"

	"github.com/theAstiv/docx-go/internal/opc"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Snapshot is a copy of what adding images, links and styles changes in a
// package: the OPC parts, relationships and content types, the media index
// and the styles. Taken by Package.Snapshot, put back by Package.Restore.
//
// It does not cover the document, header and footer trees, the numbering
// and settings parts, or the core properties: templating works on copies
// of the trees and swaps them in only on success, and never edits the
// others in place.
type Snapshot struct {
	opc       *opc.Snapshot
	styles    *wml.CT_Styles // a deep copy; nil when the package has no styles part
	imgByHash map[[sha256.Size]byte]string
}

// Snapshot records the package's current state.
func (p *Package) Snapshot() (*Snapshot, error) {
	s := &Snapshot{
		opc:       p.pkg.Snapshot(),
		styles:    nil,
		imgByHash: maps.Clone(p.imgByHash),
	}
	if p.StyPart != nil && p.StyPart.styles != nil {
		c, err := wml.Clone(p.StyPart.styles)
		if err != nil {
			return nil, fmt.Errorf("parts: snapshot styles: %w", err)
		}
		s.styles = c
	}
	return s, nil
}

// Restore puts the package back to the state s recorded. The styles tree
// keeps its identity (its contents are replaced), so style proxies callers
// hold stay valid.
func (p *Package) Restore(s *Snapshot) {
	p.pkg.Restore(s.opc)
	if s.styles != nil && p.StyPart != nil && p.StyPart.styles != nil {
		*p.StyPart.styles = *s.styles
	}
	p.imgByHash = maps.Clone(s.imgByHash)
	// The cached id scan may hold ids added since; drop it so the next
	// allocation rescans.
	p.IDCache = nil
}
