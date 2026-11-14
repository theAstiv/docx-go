package template

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/textmerge"
)

// A table of contents is a TOC field whose cached result repeats every
// heading, tags included: a template heading "{{.id}}: {{.title}}" inside a
// loop also appears, verbatim, in the TOC above the loop. Filling it there
// would fail (".id" outside its loop) and is pointless: Word rebuilds the
// TOC from the filled headings when fields update. So text inside a TOC
// field's result is not template text. The scanner lives in
// internal/textmerge, shared with docx.Replace.

// skipSet holds the w:t nodes the engine must leave alone.
type skipSet = textmerge.SkipSet

// tocSkips returns the w:t nodes inside TOC field results in blocks.
func tocSkips(blocks []wml.BlockElement) skipSet { return textmerge.TOCSkips(blocks) }

// tocSkipsParagraphs is tocSkips for a list of paragraphs (raw content
// typed one paragraph at a time).
func tocSkipsParagraphs(paras []*wml.CT_P) skipSet { return textmerge.TOCSkipsParagraphs(paras) }

// TOCTagParts returns the parts whose table of contents repeats tags from
// headings. Such a TOC shows the tags until Word updates fields, so callers
// should make Word update fields when the document opens.
func TOCTagParts(stories []Story) []string {
	var out []string
	for _, s := range stories {
		for t := range tocSkips(*s.Blocks) {
			if strings.Contains(t.Content, "{{") {
				out = append(out, s.Part)
				break
			}
		}
	}
	return out
}
