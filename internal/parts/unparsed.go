package parts

import "regexp"

// unparsedStory matches the story parts this package keeps as opaque bytes:
// footnotes, endnotes and comments.
var unparsedStory = regexp.MustCompile(`^word/(footnotes|endnotes|comments)\.xml$`)

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
