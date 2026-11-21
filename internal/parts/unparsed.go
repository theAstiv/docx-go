package parts

import (
	"regexp"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// unparsedStory matches the story parts this package keeps as opaque bytes:
// footnotes, endnotes and comments.
var unparsedStory = regexp.MustCompile(`^word/(footnotes|endnotes|comments)\.xml$`)

// boundData matches the parts content controls bind to: the core document
// properties and the custom XML data items (not their itemProps).
var boundData = regexp.MustCompile(`^(docProps/core\.xml|customXml/item\d+\.xml)$`)

// BoundDataParts returns, by path, the parts data-bound content controls
// read from (w:dataBinding): docProps/core.xml, and each customXml/item*.xml
// whose item id a content control in the document names. Word re-syncs a
// bound control from its part when it opens a file, so text written into
// such a control must also be written to the part. Custom XML items no
// control binds (add-in or SharePoint data) are left out: their text is not
// template text.
//
// Core properties changed through EditCoreProps are written into the part
// first, so the bytes returned include them. If that fails the part's
// previous bytes are returned, and Save reports the error.
func (p *Package) BoundDataParts() map[string][]byte {
	_ = p.flushCoreProps() // a failure resurfaces in Save, which flushes again
	out := map[string][]byte{}
	var bound map[string]bool
	for _, name := range p.pkg.PartPaths() {
		if !boundData.MatchString(name) {
			continue
		}
		if strings.HasPrefix(name, "customXml/") {
			if bound == nil {
				bound = p.boundStoreItems()
			}
			if !bound[p.customXMLItemID(name)] {
				continue
			}
		}
		out[name] = p.pkg.Part(name)
	}
	return out
}

// relCustomXMLProps links a custom XML item to its properties part, which
// holds the item id (ds:datastoreItem/@ds:itemID).
const relCustomXMLProps = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps"

var (
	storeItemIDAttr = regexp.MustCompile(`storeItemID\s*=\s*["']([^"']*)["']`)
	itemIDAttr      = regexp.MustCompile(`itemID\s*=\s*["']([^"']*)["']`)
)

// boundStoreItems returns the store item ids the document's content
// controls bind to, normalised by normItemID: the body, headers and
// footers, and the unparsed note and comment parts.
func (p *Package) boundStoreItems() map[string]bool {
	scan := new(wml.IDScan)
	if p.DocPart != nil {
		if body := p.DocPart.Body(); body != nil {
			wml.ScanBlocks(body.Children, scan)
		}
	}
	for _, h := range p.UniqueHeaders() {
		wml.ScanBlocks(h.Part.CT().Children, scan)
	}
	for _, f := range p.UniqueFooters() {
		wml.ScanBlocks(f.Part.CT().Children, scan)
	}
	ids := map[string]bool{}
	for _, id := range scan.StoreItemIDs {
		ids[normItemID(id)] = true
	}
	for _, data := range p.UnparsedStoryParts() {
		for _, m := range storeItemIDAttr.FindAllSubmatch(data, -1) {
			ids[normItemID(string(m[1]))] = true
		}
	}
	return ids
}

// customXMLItemID returns the normalised item id of a custom XML item, read
// from its properties part; "" when it has none.
func (p *Package) customXMLItemID(item string) string {
	for _, r := range p.pkg.Rels(item).ByType(relCustomXMLProps) {
		if r.TargetMode == "External" {
			continue
		}
		name, ok := p.pkg.FindPart(resolveRelPath(item, r.Target))
		if !ok {
			continue
		}
		if m := itemIDAttr.FindSubmatch(p.pkg.Part(name)); m != nil {
			return normItemID(string(m[1]))
		}
	}
	return ""
}

// normItemID puts a GUID in one form, so "{6c3c…}" and "6C3C…" compare equal.
func normItemID(id string) string {
	return strings.ToUpper(strings.Trim(strings.TrimSpace(id), "{}"))
}

// SetPartData replaces the bytes of an existing part. It is for parts this
// package keeps opaque (BoundDataParts); typed parts are written from their
// element trees on Save and would overwrite it. Writing the core properties
// part drops its parsed copy, which is re-read from the new bytes on next
// use.
func (p *Package) SetPartData(path string, data []byte) {
	p.pkg.SetPart(path, data)
	if p.core != nil && p.core.path == path {
		p.core = nil
	}
}

// UnparsedStoryParts returns the footnotes, endnotes and comments parts by
// path. They hold user-visible text but are not parsed, so callers that act
// on text (templating) can at least see what they cannot reach.
func (p *Package) UnparsedStoryParts() map[string][]byte {
	out := map[string][]byte{}
	for _, name := range p.pkg.PartPaths() {
		if unparsedStory.MatchString(name) {
			out[name] = p.pkg.Part(name)
		}
	}
	return out
}
