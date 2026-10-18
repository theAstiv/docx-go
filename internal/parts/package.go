package parts

import (
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/theAstiv/docx-go/internal/opc"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// HeaderEntry pairs a document-level relationship ID with a HeaderPart.
type HeaderEntry struct {
	RelID string
	Part  *HeaderPart
	path  string // OPC path (e.g. "word/header1.xml")
}

// FooterEntry pairs a document-level relationship ID with a FooterPart.
type FooterEntry struct {
	RelID string
	Part  *FooterPart
	path  string // OPC path (e.g. "word/footer1.xml")
}

// Package is the top-level OPC package with its known typed parts pre-wired.
// It is the boundary between the OPC layer (raw bytes / ZIP / relationships)
// and the docx proxy layer (ergonomic API). The docx layer only holds a
// *parts.Package and never imports internal/opc directly.
type Package struct {
	pkg     *opc.Package
	DocPart *DocumentPart
	StyPart *StylesPart
	NumPart *NumberingPart // nil until first list is created or loaded from file
	SetPart *SettingsPart  // nil until settings are modified or loaded from file
	Headers []*HeaderEntry // all header parts keyed by relID
	Footers []*FooterEntry // all footer parts keyed by relID

	// imgByHash maps the SHA-256 of each media part to its path, so identical
	// bytes are stored once. Built lazily by AddImage.
	imgByHash map[[sha256.Size]byte]string

	// IDCache is the docx layer's cached scan of document-wide ids (drawing
	// ids, bookmarks). nil means "not scanned"; the docx layer owns its
	// contents and invalidation.
	IDCache *wml.IDScan
}

// New creates an empty Package with a minimal valid OOXML structure:
// content types, package-level relationship to document.xml, and a
// document-level relationship to styles.xml.
func New() *Package {
	p := new(Package)
	p.pkg = opc.NewPackage()

	p.pkg.ContentTypes.SetDefault("rels", "application/vnd.openxmlformats-package.relationships+xml")
	p.pkg.ContentTypes.SetDefault("xml", "application/xml")
	p.pkg.ContentTypes.SetOverride("/"+PathDocument, ContentTypeDocument)
	p.pkg.ContentTypes.SetOverride("/"+PathStyles, ContentTypeStyles)

	p.pkg.Rels("").Add(opc.RelTypeOfficeDocument, PathDocument, "")
	p.pkg.Rels(PathDocument).Add(opc.RelTypeStyles, "styles.xml", "")

	p.DocPart = NewDocumentPart()
	p.StyPart = NewStylesPart()
	return p
}

// Open reads a .docx archive and parses the known parts.
func Open(r io.ReaderAt, size int64) (*Package, error) {
	pkg, err := opc.Open(r, size)
	if err != nil {
		return nil, fmt.Errorf("parts: open: %w", err)
	}

	p := new(Package)
	p.pkg = pkg

	// Locate the main document part via the package-level relationship.
	docRels := pkg.Rels("").ByType(opc.RelTypeOfficeDocument)
	if len(docRels) == 0 {
		return nil, fmt.Errorf("parts: open: no officeDocument relationship")
	}
	docPath := docRels[0].Target

	docData := pkg.Part(docPath)
	if docData == nil {
		return nil, fmt.Errorf("parts: open: part %q not found", docPath)
	}
	p.DocPart = new(DocumentPart)
	if err := p.DocPart.Parse(docData); err != nil {
		return nil, fmt.Errorf("parts: open: parse document: %w", err)
	}

	// Locate the styles part (optional).
	styRels := pkg.Rels(docPath).ByType(opc.RelTypeStyles)
	if len(styRels) > 0 {
		styPath := resolveRelPath(docPath, styRels[0].Target)
		if styData := pkg.Part(styPath); styData != nil {
			p.StyPart = new(StylesPart)
			if err := p.StyPart.Parse(styData); err != nil {
				return nil, fmt.Errorf("parts: open: parse styles: %w", err)
			}
		}
	}
	if p.StyPart == nil {
		p.StyPart = NewStylesPart()
	}

	// Locate the settings part (optional).
	setRels := pkg.Rels(docPath).ByType(opc.RelTypeSettings)
	if len(setRels) > 0 {
		setPath := resolveRelPath(docPath, setRels[0].Target)
		if setData := pkg.Part(setPath); setData != nil {
			p.SetPart = new(SettingsPart)
			if err := p.SetPart.Parse(setData); err != nil {
				return nil, fmt.Errorf("parts: open: parse settings: %w", err)
			}
		}
	}

	// Locate the numbering part (optional).
	numRels := pkg.Rels(docPath).ByType(opc.RelTypeNumbering)
	if len(numRels) > 0 {
		numPath := resolveRelPath(docPath, numRels[0].Target)
		if numData := pkg.Part(numPath); numData != nil {
			p.NumPart = new(NumberingPart)
			if err := p.NumPart.Parse(numData); err != nil {
				return nil, fmt.Errorf("parts: open: parse numbering: %w", err)
			}
		}
	}

	// Locate header parts (optional).
	for _, rel := range pkg.Rels(docPath).ByType(opc.RelTypeHeader) {
		hdrPath := resolveRelPath(docPath, rel.Target)
		hdrData := pkg.Part(hdrPath)
		if hdrData == nil {
			continue
		}
		hp := new(HeaderPart)
		if err := hp.Parse(hdrData); err != nil {
			return nil, fmt.Errorf("parts: open: parse header %q: %w", hdrPath, err)
		}
		p.Headers = append(p.Headers, &HeaderEntry{RelID: rel.ID, Part: hp, path: hdrPath})
	}

	// Locate footer parts (optional).
	for _, rel := range pkg.Rels(docPath).ByType(opc.RelTypeFooter) {
		ftrPath := resolveRelPath(docPath, rel.Target)
		ftrData := pkg.Part(ftrPath)
		if ftrData == nil {
			continue
		}
		fp := new(FooterPart)
		if err := fp.Parse(ftrData); err != nil {
			return nil, fmt.Errorf("parts: open: parse footer %q: %w", ftrPath, err)
		}
		p.Footers = append(p.Footers, &FooterEntry{RelID: rel.ID, Part: fp, path: ftrPath})
	}

	return p, nil
}

// AddHeaderPart creates a new HeaderPart, registers it in OPC, and returns its entry.
// The returned RelID is used in sectPr headerReference elements.
// The numeric suffix is max(existing suffixes)+1 to avoid path collisions when the
// loaded archive has non-sequential header numbering (e.g. header1.xml, header3.xml).
func (p *Package) AddHeaderPart() *HeaderEntry {
	n := 1
	for _, h := range p.Headers {
		var i int
		_, _ = fmt.Sscanf(path.Base(h.path), "header%d.xml", &i)
		if i >= n {
			n = i + 1
		}
	}
	hdrPath := fmt.Sprintf("word/header%d.xml", n)
	relID := p.pkg.Rels(PathDocument).Add(opc.RelTypeHeader, fmt.Sprintf("header%d.xml", n), "")
	p.pkg.ContentTypes.SetOverride("/"+hdrPath, ContentTypeHeader)
	entry := &HeaderEntry{
		RelID: relID,
		Part:  NewHeaderPart(),
		path:  hdrPath,
	}
	p.Headers = append(p.Headers, entry)
	return entry
}

// AddFooterPart creates a new FooterPart, registers it in OPC, and returns its entry.
// The numeric suffix is max(existing suffixes)+1 (same collision-safety rationale as AddHeaderPart).
func (p *Package) AddFooterPart() *FooterEntry {
	n := 1
	for _, f := range p.Footers {
		var i int
		_, _ = fmt.Sscanf(path.Base(f.path), "footer%d.xml", &i)
		if i >= n {
			n = i + 1
		}
	}
	ftrPath := fmt.Sprintf("word/footer%d.xml", n)
	relID := p.pkg.Rels(PathDocument).Add(opc.RelTypeFooter, fmt.Sprintf("footer%d.xml", n), "")
	p.pkg.ContentTypes.SetOverride("/"+ftrPath, ContentTypeFooter)
	entry := &FooterEntry{
		RelID: relID,
		Part:  NewFooterPart(),
		path:  ftrPath,
	}
	p.Footers = append(p.Footers, entry)
	return entry
}

// HeaderByRelID returns the HeaderEntry with the given relationship ID, or nil.
func (p *Package) HeaderByRelID(relID string) *HeaderEntry {
	for _, h := range p.Headers {
		if h.RelID == relID {
			return h
		}
	}
	return nil
}

// FooterByRelID returns the FooterEntry with the given relationship ID, or nil.
func (p *Package) FooterByRelID(relID string) *FooterEntry {
	for _, f := range p.Footers {
		if f.RelID == relID {
			return f
		}
	}
	return nil
}

// EnsureSettingsPart creates the SettingsPart if it does not exist yet and
// wires up the OPC content-type and relationship entries.  Idempotent.
func (p *Package) EnsureSettingsPart() *SettingsPart {
	if p.SetPart != nil {
		return p.SetPart
	}
	p.SetPart = NewSettingsPart()
	p.pkg.ContentTypes.SetOverride("/"+PathSettings, ContentTypeSettings)
	p.pkg.Rels(PathDocument).Add(opc.RelTypeSettings, "settings.xml", "")
	return p.SetPart
}

// EnsureNumberingPart creates the NumberingPart if it does not exist yet and
// wires up the OPC content-type and relationship entries.  Idempotent.
func (p *Package) EnsureNumberingPart() *NumberingPart {
	if p.NumPart != nil {
		return p.NumPart
	}
	p.NumPart = NewNumberingPart()
	p.pkg.ContentTypes.SetOverride("/"+PathNumbering, ContentTypeNumbering)
	p.pkg.Rels(PathDocument).Add(opc.RelTypeNumbering, "numbering.xml", "")
	return p.NumPart
}

// Save marshals all modified parts back into the OPC archive and writes
// the resulting .docx ZIP to w.
func (p *Package) Save(w io.Writer) error {
	if p.DocPart != nil {
		docData, err := p.DocPart.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal document: %w", err)
		}
		p.pkg.SetPart(PathDocument, docData)
	}
	if p.StyPart != nil {
		styData, err := p.StyPart.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal styles: %w", err)
		}
		p.pkg.SetPart(PathStyles, styData)
	}
	if p.SetPart != nil {
		setData, err := p.SetPart.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal settings: %w", err)
		}
		p.pkg.SetPart(PathSettings, setData)
	}
	if p.NumPart != nil {
		numData, err := p.NumPart.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal numbering: %w", err)
		}
		p.pkg.SetPart(PathNumbering, numData)
	}
	for _, entry := range p.Headers {
		hdrData, err := entry.Part.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal header %q: %w", entry.path, err)
		}
		p.pkg.SetPart(entry.path, hdrData)
	}
	for _, entry := range p.Footers {
		ftrData, err := entry.Part.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal footer %q: %w", entry.path, err)
		}
		p.pkg.SetPart(entry.path, ftrData)
	}
	return p.pkg.Save(w)
}

// resolveRelPath resolves a relationship target relative to the source part path.
// Absolute targets (starting with "/") are returned as-is after stripping the
// leading slash to match OPC's unrooted part naming convention.
// Relative targets are resolved against the source part's directory using path.Join,
// which correctly handles "../" traversals.
//
//	("word/document.xml", "styles.xml")          → "word/styles.xml"
//	("word/document.xml", "../media/img.png")    → "media/img.png"
func resolveRelPath(sourcePart, target string) string {
	if strings.HasPrefix(target, "/") {
		return target[1:]
	}
	return path.Join(path.Dir(sourcePart), target)
}
