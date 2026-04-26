package parts

import (
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/theAstiv/docx-go/internal/opc"
)

// Package is the top-level OPC package with its known typed parts pre-wired.
// It is the boundary between the OPC layer (raw bytes / ZIP / relationships)
// and the docx proxy layer (ergonomic API). The docx layer only holds a
// *parts.Package and never imports internal/opc directly.
type Package struct {
	pkg     *opc.Package
	DocPart *DocumentPart
	StyPart *StylesPart
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

	return p, nil
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
	return p.pkg.Save(w)
}

// resolveRelPath resolves a relationship target relative to the source part path.
// Absolute targets (starting with "/") are returned as-is after stripping the
// leading slash to match OPC's unrooted part naming convention.
// Relative targets are resolved against the source part's directory using path.Join,
// which correctly handles "../" traversals.
// e.g. ("word/document.xml", "styles.xml") → "word/styles.xml"
// e.g. ("word/document.xml", "../media/img.png") → "media/img.png"
func resolveRelPath(sourcePart, target string) string {
	if strings.HasPrefix(target, "/") {
		return target[1:]
	}
	return path.Join(path.Dir(sourcePart), target)
}
