package docx

import (
	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/parts"
)

// ── HeaderType / FooterType ────────────────────────────────────────────────────

// HeaderType specifies which header slot to use within a section.
type HeaderType = wml.HdrFtrType

const (
	// HeaderDefault is the default (odd-page) header.
	HeaderDefault HeaderType = wml.HdrFtrDefault
	// HeaderFirst is the first-page header (requires EnableTitlePage).
	HeaderFirst HeaderType = wml.HdrFtrFirst
	// HeaderEven is the even-page header (requires even-and-odd headers in settings).
	HeaderEven HeaderType = wml.HdrFtrEven
)

// FooterType mirrors HeaderType for footer slots.
type FooterType = wml.HdrFtrType

const (
	// FooterDefault is the default (odd-page) footer.
	FooterDefault FooterType = wml.HdrFtrDefault
	// FooterFirst is the first-page footer (requires EnableTitlePage).
	FooterFirst FooterType = wml.HdrFtrFirst
	// FooterEven is the even-page footer.
	FooterEven FooterType = wml.HdrFtrEven
)

// ── PageMargins ────────────────────────────────────────────────────────────────

// PageMargins holds the four page margins in twips.
type PageMargins struct {
	Top, Right, Bottom, Left Twip
}

// ── Section ───────────────────────────────────────────────────────────────────

// Section is a proxy for a w:sectPr element, providing ergonomic access to
// page geometry and header/footer references.  The body-level sectPr (the
// final section) is returned by Document.LastSection().
type Section struct {
	sectPr *wml.CT_SectPr
	pkg    *parts.Package
}

// PageWidth returns the page width in twips, or nil if not explicitly set.
func (s *Section) PageWidth() *Twip {
	v := s.sectPr.PgSzW()
	if v == nil {
		return nil
	}
	t := Twip(*v)
	return &t
}

// PageHeight returns the page height in twips, or nil if not explicitly set.
func (s *Section) PageHeight() *Twip {
	v := s.sectPr.PgSzH()
	if v == nil {
		return nil
	}
	t := Twip(*v)
	return &t
}

// SetPageSize sets the page width and height.
func (s *Section) SetPageSize(w, h Twip) {
	s.sectPr.SetPgSz(int64(w), int64(h))
}

// Margins returns the current page margins.
func (s *Section) Margins() PageMargins {
	toTwip := func(v *int64) Twip {
		if v == nil {
			return 0
		}
		return Twip(*v)
	}
	return PageMargins{
		Top:    toTwip(s.sectPr.PgMarTop()),
		Right:  toTwip(s.sectPr.PgMarRight()),
		Bottom: toTwip(s.sectPr.PgMarBottom()),
		Left:   toTwip(s.sectPr.PgMarLeft()),
	}
}

// SetMargins sets all four page margins.
func (s *Section) SetMargins(m PageMargins) {
	s.sectPr.SetPgMargins(int64(m.Top), int64(m.Right), int64(m.Bottom), int64(m.Left))
}

// EnableTitlePage enables or disables the first-page header/footer for this section.
func (s *Section) EnableTitlePage(enable bool) {
	s.sectPr.SetTitlePg(enable)
}

// TitlePg returns true if the first-page header/footer is enabled.
func (s *Section) TitlePg() bool {
	return s.sectPr.TitlePg()
}

// SetHeader creates or retrieves the header of the given type for this section
// and returns its proxy.  It creates a new HeaderPart the first time a given
// type is requested.  If a reference of the given type exists but its part is
// absent (dangling relID), the stale reference is replaced with a new part.
func (s *Section) SetHeader(typ HeaderType) *Header {
	if s.pkg == nil {
		return nil
	}
	for _, ref := range s.sectPr.HeaderRefs() {
		if ref.Type == typ {
			entry := s.pkg.HeaderByRelID(ref.RelID)
			if entry != nil {
				return &Header{pkg: s.pkg, ct: entry.Part.CT()}
			}
			// Dangling ref: remove before adding a fresh one to avoid duplicates.
			s.sectPr.RemoveHeaderRef(typ)
			break
		}
	}
	entry := s.pkg.AddHeaderPart()
	s.sectPr.AddHeaderRef(typ, entry.RelID)
	return &Header{pkg: s.pkg, ct: entry.Part.CT()}
}

// Header returns the header of the given type for this section, or nil.
func (s *Section) Header(typ HeaderType) *Header {
	if s.pkg == nil {
		return nil
	}
	for _, ref := range s.sectPr.HeaderRefs() {
		if ref.Type == typ {
			entry := s.pkg.HeaderByRelID(ref.RelID)
			if entry != nil {
				return &Header{pkg: s.pkg, ct: entry.Part.CT()}
			}
		}
	}
	return nil
}

// SetFooter creates or retrieves the footer of the given type for this section
// and returns its proxy.  Dangling relIDs are replaced (not duplicated).
func (s *Section) SetFooter(typ FooterType) *Footer {
	if s.pkg == nil {
		return nil
	}
	for _, ref := range s.sectPr.FooterRefs() {
		if ref.Type == typ {
			entry := s.pkg.FooterByRelID(ref.RelID)
			if entry != nil {
				return &Footer{pkg: s.pkg, ct: entry.Part.CT()}
			}
			s.sectPr.RemoveFooterRef(typ)
			break
		}
	}
	entry := s.pkg.AddFooterPart()
	s.sectPr.AddFooterRef(typ, entry.RelID)
	return &Footer{pkg: s.pkg, ct: entry.Part.CT()}
}

// Footer returns the footer of the given type for this section, or nil.
func (s *Section) Footer(typ FooterType) *Footer {
	if s.pkg == nil {
		return nil
	}
	for _, ref := range s.sectPr.FooterRefs() {
		if ref.Type == typ {
			entry := s.pkg.FooterByRelID(ref.RelID)
			if entry != nil {
				return &Footer{pkg: s.pkg, ct: entry.Part.CT()}
			}
		}
	}
	return nil
}

// ── Document.LastSection ──────────────────────────────────────────────────────

// LastSection returns a Section proxy for the body-level sectPr (the final
// section of the document).  If no sectPr exists, a default one is created.
func (d *Document) LastSection() *Section {
	body := d.pkg.DocPart.Body()
	if body.SectPr == nil {
		body.SectPr = wml.DefaultSectPr()
	}
	return &Section{sectPr: body.SectPr, pkg: d.pkg}
}
