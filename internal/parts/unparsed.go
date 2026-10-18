package parts

import "regexp"

// unparsedStory matches the story parts this package keeps as opaque bytes:
// footnotes, endnotes and comments.
var unparsedStory = regexp.MustCompile(`^word/(footnotes|endnotes|comments)\.xml$`)

// boundData matches the parts content controls bind to: the core document
// properties and the custom XML data items (not their itemProps).
var boundData = regexp.MustCompile(`^(docProps/core\.xml|customXml/item\d+\.xml)$`)

// BoundDataParts returns, by path, the parts data-bound content controls
// read from (w:dataBinding): docProps/core.xml and customXml/item*.xml. Word
// re-syncs a bound control from its part when it opens a file, so text
// written into such a control must also be written to the part.
func (p *Package) BoundDataParts() map[string][]byte {
	out := map[string][]byte{}
	for _, name := range p.pkg.PartPaths() {
		if boundData.MatchString(name) {
			out[name] = p.pkg.Part(name)
		}
	}
	return out
}

// SetPartData replaces the bytes of an existing part. It is for parts this
// package keeps opaque (BoundDataParts); typed parts are written from their
// element trees on Save and would overwrite it.
func (p *Package) SetPartData(path string, data []byte) {
	p.pkg.SetPart(path, data)
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
