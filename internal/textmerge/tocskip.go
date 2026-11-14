package textmerge

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// A table of contents is a TOC field whose cached result repeats every
// heading. Word rebuilds it from the headings when fields update, so text
// inside it is not worth filling or replacing: the edit would be lost, and a
// template heading's tags would be filled out of their loop. The field spans
// paragraphs, so the state is tracked across a whole block list.

// SkipSet holds the w:t nodes inside TOC, INDEX and TOA field results.
type SkipSet map[*wml.CT_Text]bool

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
	skip  SkipSet
}

// TOCSkips returns the w:t nodes inside TOC field results in blocks,
// descending into tables.
func TOCSkips(blocks []wml.BlockElement) SkipSet {
	s := &tocScanner{stack: nil, skip: SkipSet{}}
	s.blocks(blocks)
	return s.skip
}

// TOCSkipsParagraphs is TOCSkips for a list of paragraphs (raw content typed
// one paragraph at a time).
func TOCSkipsParagraphs(paras []*wml.CT_P) SkipSet {
	s := &tocScanner{stack: nil, skip: SkipSet{}}
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
					top.toc = IsTOCInstr(top.instr.String())
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

// IsTOCInstr reports whether a field instruction is a table of contents
// (TOC), or one of its relatives that also repeats document text: a table
// of figures uses TOC too; INDEX and TOA collect entries from the text.
func IsTOCInstr(instr string) bool {
	word, _, _ := strings.Cut(strings.TrimSpace(instr), " ")
	switch strings.ToUpper(word) {
	case "TOC", "INDEX", "TOA":
		return true
	}
	return false
}
