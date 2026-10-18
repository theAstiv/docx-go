package template

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// A table of contents is a TOC field whose cached result repeats every
// heading, tags included: a template heading "{{.id}}: {{.title}}" inside a
// loop also appears, verbatim, in the TOC above the loop. Filling it there
// would fail (".id" outside its loop) and is pointless: Word rebuilds the
// TOC from the filled headings when fields update. So text inside a TOC
// field's result is not template text. The field spans paragraphs, so the
// state is tracked across a whole block list.

// skipSet holds the w:t nodes the engine must leave alone.
type skipSet map[*wml.CT_Text]bool

// fieldFrame is one open complex field.
type fieldFrame struct {
	instr    strings.Builder
	inResult bool
	toc      bool
}

// tocScanner walks content in document order and records the text inside
// TOC field results.
type tocScanner struct {
	stack []*fieldFrame
	skip  skipSet
}

// tocSkips returns the w:t nodes inside TOC field results in blocks.
func tocSkips(blocks []wml.BlockElement) skipSet {
	s := &tocScanner{stack: nil, skip: skipSet{}}
	s.blocks(blocks)
	return s.skip
}

// tocSkipsParagraphs is tocSkips for a list of paragraphs (raw content
// typed one paragraph at a time).
func tocSkipsParagraphs(paras []*wml.CT_P) skipSet {
	s := &tocScanner{stack: nil, skip: skipSet{}}
	for _, p := range paras {
		s.inlines(p.Children)
	}
	return s.skip
}

func (s *tocScanner) blocks(blocks []wml.BlockElement) {
	for _, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			s.inlines(el.Children)
		case *wml.CT_Tbl:
			for _, r := range el.Children {
				if row, ok := r.(*wml.CT_Tr); ok {
					for _, c := range row.Children {
						if cell, ok := c.(*wml.CT_Tc); ok {
							s.blocks(cell.Children)
						}
					}
				}
			}
		}
	}
}

func (s *tocScanner) inlines(children []wml.InlineElement) {
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			s.run(el)
		case *wml.CT_Hyperlink:
			s.inlines(el.Children)
		}
	}
}

func (s *tocScanner) run(r *wml.CT_R) {
	for _, rc := range r.Children {
		switch el := rc.(type) {
		case *wml.CT_FldChar:
			switch el.FldCharType {
			case "begin":
				s.stack = append(s.stack, &fieldFrame{instr: strings.Builder{}, inResult: false, toc: false})
			case "separate":
				if top := s.top(); top != nil {
					top.inResult = true
					top.toc = isTOCInstr(top.instr.String())
				}
			case "end":
				if len(s.stack) > 0 {
					s.stack = s.stack[:len(s.stack)-1]
				}
			}
		case *wml.CT_InstrText:
			if top := s.top(); top != nil && !top.inResult {
				top.instr.WriteString(el.Content)
			}
		case *wml.CT_Text:
			if s.inTOCResult() {
				s.skip[el] = true
			}
		}
	}
}

func (s *tocScanner) top() *fieldFrame {
	if len(s.stack) == 0 {
		return nil
	}
	return s.stack[len(s.stack)-1]
}

func (s *tocScanner) inTOCResult() bool {
	for _, f := range s.stack {
		if f.toc && f.inResult {
			return true
		}
	}
	return false
}

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

// isTOCInstr reports whether a field instruction is a table of contents
// (TOC), or one of its relatives that also repeats document text: a table
// of figures uses TOC too; INDEX and TOA collect entries from the text.
func isTOCInstr(instr string) bool {
	word, _, _ := strings.Cut(strings.TrimSpace(instr), " ")
	switch strings.ToUpper(word) {
	case "TOC", "INDEX", "TOA":
		return true
	}
	return false
}
