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
	styles    *stylesSnapshot // nil when the package has no styles part
	imgByHash map[[sha256.Size]byte]string
}

// stylesSnapshot records the styles tree so it can be put back in place:
// the same *CT_Styles, *CT_Style and *CT_DocDefaults objects, with their
// contents as they were. Style proxies hold those pointers (and bind their
// formatting to slots inside them), so they keep working after a restore.
type stylesSnapshot struct {
	root        *wml.CT_Styles      // a deep copy of the whole tree
	styles      []*wml.CT_Style     // the original style objects, in order
	styleVals   []*wml.CT_Style     // a deep copy of each
	docDefaults *wml.CT_DocDefaults // the original object; nil when absent
}

// Snapshot records the package's current state.
func (p *Package) Snapshot() (*Snapshot, error) {
	s := &Snapshot{
		opc:       p.pkg.Snapshot(),
		styles:    nil,
		imgByHash: maps.Clone(p.imgByHash),
	}
	if p.StyPart != nil && p.StyPart.styles != nil {
		st, err := snapshotStyles(p.StyPart.styles)
		if err != nil {
			return nil, err
		}
		s.styles = st
	}
	return s, nil
}

func snapshotStyles(cur *wml.CT_Styles) (*stylesSnapshot, error) {
	root, err := wml.Clone(cur)
	if err != nil {
		return nil, fmt.Errorf("parts: snapshot styles: %w", err)
	}
	s := &stylesSnapshot{
		root:        root,
		styles:      append([]*wml.CT_Style(nil), cur.Styles...),
		styleVals:   make([]*wml.CT_Style, len(cur.Styles)),
		docDefaults: cur.DocDefaults,
	}
	for i, st := range cur.Styles {
		if s.styleVals[i], err = wml.Clone(st); err != nil {
			return nil, fmt.Errorf("parts: snapshot style: %w", err)
		}
	}
	return s, nil
}

// Restore puts the package back to the state s recorded. The styles tree,
// every style object and the docDefaults object that existed then keep
// their identity (their contents are put back), and styles added since are
// dropped, so style proxies callers hold stay part of the document.
func (p *Package) Restore(s *Snapshot) {
	p.pkg.Restore(s.opc)
	if s.styles != nil && p.StyPart != nil && p.StyPart.styles != nil {
		s.styles.restore(p.StyPart.styles)
	}
	p.imgByHash = maps.Clone(s.imgByHash)
	// The cached id scan may hold ids added since; drop it so the next
	// allocation rescans.
	p.IDCache = nil
}

func (s *stylesSnapshot) restore(cur *wml.CT_Styles) {
	for i, st := range s.styles {
		*st = *s.styleVals[i]
	}
	root := *s.root
	root.Styles = append([]*wml.CT_Style(nil), s.styles...)
	if s.docDefaults != nil && root.DocDefaults != nil {
		*s.docDefaults = *root.DocDefaults
		root.DocDefaults = s.docDefaults
	}
	*cur = root
}
