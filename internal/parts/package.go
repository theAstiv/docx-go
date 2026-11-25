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

	// StructGen counts edits that can detach a block list from the document
	// (removing a paragraph, table or row, replacing a container's content,
	// templating). The docx layer owns it: proxies record it with their
	// parent hint and trust the hint only while it is unchanged.
	StructGen uint64

	// core is the parsed docProps/core.xml, loaded on first use (coreprops.go).
	core *corePropsState

	// The stored OPC paths of the typed parts. A loaded package keeps the
	// paths its relationships name (word/document2.xml, word/Styles.xml),
	// and Save writes each part back there; the Path* constants are only the
	// names New and the Ensure* creators start from.
	docPath, styPath, setPath, numPath string
}

// DocPath returns the OPC path of the main document part, the source of its
// relationships (e.g. "word/document.xml").
func (p *Package) DocPath() string { return p.docPath }

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
	p.docPath, p.styPath = PathDocument, PathStyles
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
	// The target may be absolute ("/word/document.xml", as System.IO.Packaging
	// and the Open XML SDK write it) and may differ in case from the ZIP
	// entry; OPC part names compare case-insensitively.
	docPath, docData, ok := p.locate("", docRels[0])
	if !ok {
		return nil, fmt.Errorf("parts: open: officeDocument target %q is not a part of the package", docRels[0].Target)
	}
	if docData == nil {
		return nil, fmt.Errorf("parts: open: part %q not found", docPath)
	}
	p.docPath = docPath
	// Every typed role the package resolves is claimed here; one part may
	// hold only one (roles.go).
	claims := roleClaims{}
	claim := func(name, role string) error {
		if err := claims.claim(name, role); err != nil {
			return fmt.Errorf("parts: open: %w", err)
		}
		return nil
	}
	if err := claim(docPath, "main document"); err != nil {
		return nil, err
	}
	if err := pkg.RelsErr(docPath); err != nil {
		return nil, fmt.Errorf("parts: open: %w", err)
	}
	p.DocPart = new(DocumentPart)
	if err := p.DocPart.Parse(docData); err != nil {
		return nil, fmt.Errorf("parts: open: parse document: %w", err)
	}

	// Locate the styles part. Every Word document has one; when the
	// relationship is missing, or names no part inside the package, a
	// default one is added under a free name (attach), so an orphaned
	// word/styles.xml is never overwritten.
	if styRels := pkg.Rels(docPath).ByType(opc.RelTypeStyles); len(styRels) > 0 {
		if styPath, styData, ok := p.locate(docPath, styRels[0]); ok {
			p.styPath = styPath
			if err := claim(styPath, "styles"); err != nil {
				return nil, err
			}
			if styData != nil {
				p.StyPart = new(StylesPart)
				if err := p.StyPart.Parse(styData); err != nil {
					return nil, fmt.Errorf("parts: open: parse styles: %w", err)
				}
			} else {
				// The relationship names a missing part: create it there.
				p.pkg.ContentTypes.SetOverride("/"+styPath, ContentTypeStyles)
			}
		}
	}
	if p.StyPart == nil {
		p.StyPart = NewStylesPart()
	}
	if p.styPath == "" {
		p.styPath = p.attach(opc.RelTypeStyles, "styles.xml", ContentTypeStyles)
	}

	// Locate the settings part (optional).
	if setRels := pkg.Rels(docPath).ByType(opc.RelTypeSettings); len(setRels) > 0 {
		if setPath, setData, _ := p.locate(docPath, setRels[0]); setData != nil {
			if err := claim(setPath, "settings"); err != nil {
				return nil, err
			}
			p.SetPart = new(SettingsPart)
			if err := p.SetPart.Parse(setData); err != nil {
				return nil, fmt.Errorf("parts: open: parse settings: %w", err)
			}
			p.setPath = setPath
		}
	}

	// Locate the numbering part (optional).
	if numRels := pkg.Rels(docPath).ByType(opc.RelTypeNumbering); len(numRels) > 0 {
		if numPath, numData, _ := p.locate(docPath, numRels[0]); numData != nil {
			if err := claim(numPath, "numbering"); err != nil {
				return nil, err
			}
			p.NumPart = new(NumberingPart)
			if err := p.NumPart.Parse(numData); err != nil {
				return nil, fmt.Errorf("parts: open: parse numbering: %w", err)
			}
			p.numPath = numPath
		}
	}

	// Locate header and footer parts (optional). Two relationships to one
	// part share a single tree; separate trees would each be saved over the
	// other, and the last one would win.
	hdrByPath := map[string]*HeaderPart{}
	for _, rel := range pkg.Rels(docPath).ByType(opc.RelTypeHeader) {
		hdrPath, hdrData, _ := p.locate(docPath, rel)
		if hdrData == nil {
			continue
		}
		if err := claim(hdrPath, "header"); err != nil {
			return nil, err
		}
		hp := hdrByPath[hdrPath]
		if hp == nil {
			if err := pkg.RelsErr(hdrPath); err != nil {
				return nil, fmt.Errorf("parts: open: %w", err)
			}
			hp = new(HeaderPart)
			if err := hp.Parse(hdrData); err != nil {
				return nil, fmt.Errorf("parts: open: parse header %q: %w", hdrPath, err)
			}
			hdrByPath[hdrPath] = hp
		}
		p.Headers = append(p.Headers, &HeaderEntry{RelID: rel.ID, Part: hp, path: hdrPath})
	}
	ftrByPath := map[string]*FooterPart{}
	for _, rel := range pkg.Rels(docPath).ByType(opc.RelTypeFooter) {
		ftrPath, ftrData, _ := p.locate(docPath, rel)
		if ftrData == nil {
			continue
		}
		if err := claim(ftrPath, "footer"); err != nil {
			return nil, err
		}
		fp := ftrByPath[ftrPath]
		if fp == nil {
			if err := pkg.RelsErr(ftrPath); err != nil {
				return nil, fmt.Errorf("parts: open: %w", err)
			}
			fp = new(FooterPart)
			if err := fp.Parse(ftrData); err != nil {
				return nil, fmt.Errorf("parts: open: parse footer %q: %w", ftrPath, err)
			}
			ftrByPath[ftrPath] = fp
		}
		p.Footers = append(p.Footers, &FooterEntry{RelID: rel.ID, Part: fp, path: ftrPath})
	}

	// Core properties are parsed lazily, and a part that does not parse is
	// replaced on the first edit, so a core-properties relationship to the
	// main document would overwrite it.
	if cp := p.corePropsPath(); cp != "" {
		if err := claim(cp, "core properties"); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// locate resolves relationship r of the part at source (resolveTarget) and
// returns the stored name of the part it names (compared ignoring case) with
// its bytes, or the resolved name and nil when there is no such part. ok is
// false, with no name, when r names no part inside the package (an external
// or invalid target).
func (p *Package) locate(source string, r opc.Relationship) (name string, data []byte, ok bool) {
	name, ok = resolveTarget(source, r)
	if !ok {
		return "", nil, false
	}
	if stored, found := p.pkg.FindPart(name); found {
		return stored, p.pkg.Part(stored), true
	}
	if p.nestsWithPart(name) {
		return "", nil, false
	}
	return name, nil, true
}

// nestsWithPart reports whether name, which no part has, is a folder of a
// stored part ("word" beside "word/document.xml") or lies inside one
// ("word/document.xml/x"). OPC forbids both (ECMA-376 Part 2, 9.1.1.1), and
// a ZIP holding them is no longer a valid package.
func (p *Package) nestsWithPart(name string) bool {
	lower := strings.ToLower(name)
	for _, stored := range p.pkg.PartPaths() {
		s := strings.ToLower(stored)
		if strings.HasPrefix(s, lower+"/") || strings.HasPrefix(lower, s+"/") {
			return true
		}
	}
	return false
}

// findTarget returns the stored name of the part relationship r of the part
// at source points to, and whether there is one.
func (p *Package) findTarget(source string, r opc.Relationship) (string, bool) {
	name, data, ok := p.locate(source, r)
	return name, ok && data != nil
}

// taken reports whether name is in use, ignoring case: by a stored part or
// by a typed part not saved yet.
func (p *Package) taken(name string) bool {
	if _, ok := p.pkg.FindPart(name); ok {
		return true
	}
	for _, used := range []string{p.docPath, p.styPath, p.setPath, p.numPath} {
		if used != "" && strings.EqualFold(used, name) {
			return true
		}
	}
	for _, h := range p.Headers {
		if strings.EqualFold(h.path, name) {
			return true
		}
	}
	for _, f := range p.Footers {
		if strings.EqualFold(f.path, name) {
			return true
		}
	}
	return false
}

// freeName returns a part name beside the main document part, based on base
// ("styles.xml"), that no part uses: base itself, else base with a number
// before the extension ("styles2.xml"). With numbered set, numbering always
// applies and starts at 1 ("header1.xml").
func (p *Package) freeName(base string, numbered bool) string {
	dir := path.Dir(p.docPath)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if !numbered {
		if name := path.Join(dir, base); !p.taken(name) {
			return name
		}
	}
	for n := 1; ; n++ {
		if !numbered && n == 1 {
			continue
		}
		if name := path.Join(dir, fmt.Sprintf("%s%d%s", stem, n, ext)); !p.taken(name) {
			return name
		}
	}
}

// attach picks a free part name from base, adds a relationship of relType
// from the main document part to it and its content-type override, and
// returns the name.
//
// attach is called only when the package has no part of that role, so any
// relationship of relType the main document already has names nothing
// usable (an external or invalid target, or a missing part); those are
// removed, or the document would have two relationships for one role.
func (p *Package) attach(relType, base, contentType string) string {
	rels := p.pkg.Rels(p.docPath)
	for _, r := range rels.ByType(relType) {
		rels.Remove(r.ID)
	}
	name := p.freeName(base, false)
	p.pkg.Rels(p.docPath).Add(relType, path.Base(name), "")
	p.pkg.ContentTypes.SetOverride("/"+name, contentType)
	return name
}

// AddHeaderPart creates a new HeaderPart, registers it in OPC, and returns its entry.
// The returned RelID is used in sectPr headerReference elements.
// The part is named headerN.xml beside the main document part, with the first
// N no part uses: a header part no relationship references (a leftover in
// the archive) is not overwritten.
func (p *Package) AddHeaderPart() *HeaderEntry {
	hdrPath := p.freeName("header.xml", true)
	relID := p.pkg.Rels(p.docPath).Add(opc.RelTypeHeader, path.Base(hdrPath), "")
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
// The part is named as in AddHeaderPart (footerN.xml).
func (p *Package) AddFooterPart() *FooterEntry {
	ftrPath := p.freeName("footer.xml", true)
	relID := p.pkg.Rels(p.docPath).Add(opc.RelTypeFooter, path.Base(ftrPath), "")
	p.pkg.ContentTypes.SetOverride("/"+ftrPath, ContentTypeFooter)
	entry := &FooterEntry{
		RelID: relID,
		Part:  NewFooterPart(),
		path:  ftrPath,
	}
	p.Footers = append(p.Footers, entry)
	return entry
}

// UniqueHeaders returns the header entries with one entry per part: when two
// relationships name one header part, they share a tree, and code that edits
// or reads every header (search and replace, templating) must visit it once.
func (p *Package) UniqueHeaders() []*HeaderEntry {
	seen := make(map[*HeaderPart]bool, len(p.Headers))
	out := make([]*HeaderEntry, 0, len(p.Headers))
	for _, h := range p.Headers {
		if !seen[h.Part] {
			seen[h.Part] = true
			out = append(out, h)
		}
	}
	return out
}

// UniqueFooters is UniqueHeaders for footers.
func (p *Package) UniqueFooters() []*FooterEntry {
	seen := make(map[*FooterPart]bool, len(p.Footers))
	out := make([]*FooterEntry, 0, len(p.Footers))
	for _, f := range p.Footers {
		if !seen[f.Part] {
			seen[f.Part] = true
			out = append(out, f)
		}
	}
	return out
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
	p.setPath = p.attach(opc.RelTypeSettings, "settings.xml", ContentTypeSettings)
	return p.SetPart
}

// EnsureNumberingPart creates the NumberingPart if it does not exist yet and
// wires up the OPC content-type and relationship entries.  Idempotent.
func (p *Package) EnsureNumberingPart() *NumberingPart {
	if p.NumPart != nil {
		return p.NumPart
	}
	p.NumPart = NewNumberingPart()
	p.numPath = p.attach(opc.RelTypeNumbering, "numbering.xml", ContentTypeNumbering)
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
		p.pkg.SetPart(p.docPath, docData)
	}
	if p.StyPart != nil {
		styData, err := p.StyPart.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal styles: %w", err)
		}
		p.pkg.SetPart(p.styPath, styData)
	}
	if p.SetPart != nil {
		setData, err := p.SetPart.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal settings: %w", err)
		}
		p.pkg.SetPart(p.setPath, setData)
	}
	if p.NumPart != nil {
		numData, err := p.NumPart.Marshal()
		if err != nil {
			return fmt.Errorf("parts: save: marshal numbering: %w", err)
		}
		p.pkg.SetPart(p.numPath, numData)
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
	if err := p.flushCoreProps(); err != nil {
		return fmt.Errorf("parts: save: %w", err)
	}
	return p.pkg.Save(w)
}

// resolveTarget returns the part name that relationship r of the part at
// source points to, unrooted as parts are stored, and whether it names a
// part inside the package at all. It reports false for an external
// relationship, a target with a URI scheme or host, an empty target, one that
// climbs above the package root, and any result that is not a valid part
// name (opc.IsPartName). Callers treat such a relationship as naming no part:
// its target must never become the name a part is saved under.
//
//	("word/document.xml", "styles.xml")        → "word/styles.xml"
//	("word/document.xml", "../media/img.png")  → "media/img.png"
//	("word/document.xml", "/word/styles.xml")  → "word/styles.xml"
//	("word/document.xml", "../../x.xml")       → false
func resolveTarget(source string, r opc.Relationship) (string, bool) {
	if strings.EqualFold(r.TargetMode, targetModeExternal) {
		return "", false
	}
	return resolveRelPath(source, r.Target)
}

// resolveRelPath is resolveTarget for an internal target.
func resolveRelPath(source, target string) (string, bool) {
	if target == "" {
		return "", false
	}
	if hasScheme(target) || strings.HasPrefix(target, "//") {
		return "", false
	}
	var name string
	if strings.HasPrefix(target, "/") {
		name = strings.TrimPrefix(path.Clean(target), "/")
	} else {
		name = path.Join(path.Dir(source), target)
	}
	if name == ".." || strings.HasPrefix(name, "../") || !opc.IsPartName(name) {
		return "", false
	}
	return name, true
}

// hasScheme reports whether target starts with a URI scheme ("http:",
// "file:"): a letter, then letters, digits, "+", "-" or ".", then ":".
func hasScheme(target string) bool {
	for i, c := range target {
		switch {
		case c == ':':
			return i > 0
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case i > 0 && ('0' <= c && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return false
}

// ErrLimitExceeded is returned (wrapped) by Open when the archive exceeds a
// resource limit: too many entries, an entry or the whole archive too large
// uncompressed, or an XML part nested too deeply.
var ErrLimitExceeded = opc.ErrLimitExceeded

// ErrMalformedPart is returned (wrapped) by Open when an XML part holds
// text or a DOCTYPE declaration before its root element, a typed part's root
// element is not the one its role calls for, or one part holds two typed
// roles (roles.go).
var ErrMalformedPart = opc.ErrMalformedPart

// ErrInvalidPartName is returned (wrapped) by Open when a ZIP entry name is
// not a valid part name, or two entries name the same part.
var ErrInvalidPartName = opc.ErrPartName
