package parts

import (
	"regexp"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// customXMLItem matches the custom XML data items content controls bind to
// (not their itemProps). Part names compare ignoring case (OPC), so the
// pattern does too.
var customXMLItem = regexp.MustCompile(`(?i)^customXml/item\d+\.xml$`)

// BoundDataParts returns, by path, the parts data-bound content controls
// read from (w:dataBinding): the core properties part the package
// relationships name, and each customXml/item*.xml whose item id a content
// control in the document names. Word re-syncs a bound control from its part
// when it opens a file, so text written into such a control must also be
// written to the part. Custom XML items no control binds (add-in or
// SharePoint data) are left out: their text is not template text.
//
// Core properties changed through EditCoreProps are written into the part
// first, so the bytes returned include them. If that fails the part's
// previous bytes are returned, and Save reports the error.
func (p *Package) BoundDataParts() map[string][]byte {
	_ = p.flushCoreProps() // a failure resurfaces in Save, which flushes again
	out := map[string][]byte{}
	if cp := p.corePropsPath(); cp != "" {
		if data := p.pkg.Part(cp); data != nil {
			out[cp] = data
		}
	}
	var bound map[string]bool
	for _, name := range p.pkg.PartPaths() {
		if !customXMLItem.MatchString(name) {
			continue
		}
		if bound == nil {
			bound = p.boundStoreItems()
		}
		if bound[p.customXMLItemID(name)] {
			out[name] = p.pkg.Part(name)
		}
	}
	return out
}

// relCustomXMLProps links a custom XML item to its properties part, which
// holds the item id (ds:datastoreItem/@ds:itemID).
const relCustomXMLProps = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps"

var itemIDAttr = regexp.MustCompile(`itemID\s*=\s*["']([^"']*)["']`)

// boundStoreItems returns the store item ids the document's content
// controls bind to, normalised by normItemID: the body, headers and
// footers, and the note and comment parts.
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
	p.ScanNotes(scan)
	ids := map[string]bool{}
	for _, id := range scan.StoreItemIDs {
		ids[normItemID(id)] = true
	}
	return ids
}

// customXMLItemID returns the normalised item id of a custom XML item, read
// from its properties part; "" when it has none.
func (p *Package) customXMLItemID(item string) string {
	for _, r := range p.pkg.Rels(item).ByType(relCustomXMLProps) {
		name, ok := p.findTarget(item, r)
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
	if p.core != nil && strings.EqualFold(p.core.path, path) {
		p.core = nil
	}
}

// UnparsedStoryParts returns the footnotes, endnotes and comments parts the
// main document's relationships name, by path. They hold user-visible text
// but are not parsed, so callers that act on text (templating) can at least
// see what they cannot reach.
func (p *Package) UnparsedStoryParts() map[string][]byte {
	out := map[string][]byte{}
	for _, kind := range noteKinds {
		if path := p.NotePath(kind); path != "" {
			out[path] = p.pkg.Part(path)
		}
	}
	return out
}
