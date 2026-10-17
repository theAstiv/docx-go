package docx

import (
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// ErrInvalidTOCLevels reports a TOCOptions.Levels value that is not a range
// such as "1-3" with both ends between 1 and 9.
var ErrInvalidTOCLevels = errors.New("docx: invalid TOC levels")

// tocPlaceholder is the text Word shows until the TOC is updated.
const tocPlaceholder = "Right-click to update table of contents."

// Field is a proxy for a complex field: the runs from w:fldChar begin through
// end. docx-go never computes field results; the cached result is whatever the
// document (or the caller of AddField) stored, and Word refreshes it on update.
type Field struct {
	instr  string
	result string
}

// Instruction returns the field code with surrounding whitespace trimmed,
// for example `PAGE` or `TOC \o "1-3" \h`.
func (f *Field) Instruction() string { return strings.TrimSpace(f.instr) }

// Code returns the field type, the first word of the instruction in upper
// case (PAGE, NUMPAGES, TOC, ...).
func (f *Field) Code() string {
	fields := strings.Fields(f.instr)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

// CachedResult returns the text stored between the separate and end markers.
// Text produced by nested fields is included; their instructions are not.
func (f *Field) CachedResult() string { return f.result }

// AddField appends a complex field to the paragraph. instr is the field code
// without surrounding spaces (`PAGE`, `NUMPAGES`, `REF intro \h`); it is not
// validated. cachedResult is displayed until Word updates the field.
func (p *Paragraph) AddField(instr, cachedResult string) *Field {
	for _, r := range wml.NewFieldRuns(instr, cachedResult) {
		p.p.Children = append(p.p.Children, r)
	}
	return &Field{instr: " " + instr + " ", result: cachedResult}
}

// AddPageNumber appends a PAGE field. The cached result "1" is replaced by
// Word when the document is laid out.
func (p *Paragraph) AddPageNumber() *Field { return p.AddField("PAGE", "1") }

// AddPageCount appends a NUMPAGES field. The cached result "1" is replaced by
// Word when the document is laid out.
func (p *Paragraph) AddPageCount() *Field { return p.AddField("NUMPAGES", "1") }

// Fields iterates over the outermost complex fields of the paragraph, in
// order. Runs inside hyperlinks are included.
//
// Limits: only fields whose begin and end markers are both in this paragraph
// are reported, and simple fields (w:fldSimple) are not. A field that spans
// paragraphs is not found; the common case is a table of contents written by
// Word, which begins in one paragraph and ends many paragraphs later. Fields
// created by AddField, AddPageNumber, AddPageCount and Document.AddTOC always
// sit in a single paragraph and are found.
func (p *Paragraph) Fields() iter.Seq[*Field] {
	return func(yield func(*Field) bool) {
		var (
			depth   int
			inInstr bool
			cur     *Field
		)
		var instr, result strings.Builder
		visit := func(r *wml.CT_R) bool {
			for _, c := range r.Children {
				switch el := c.(type) {
				case *wml.CT_FldChar:
					switch el.FldCharType {
					case wml.FldCharBegin:
						depth++
						if depth == 1 {
							cur = new(Field)
							inInstr = true
							instr.Reset()
							result.Reset()
						}
					case wml.FldCharSeparate:
						if depth == 1 {
							inInstr = false
						}
					case wml.FldCharEnd:
						if depth == 1 && cur != nil {
							cur.instr = instr.String()
							cur.result = result.String()
							f := cur
							cur = nil
							depth = 0
							if !yield(f) {
								return false
							}
							continue
						}
						if depth > 0 {
							depth--
						}
					}
				case *wml.CT_InstrText:
					if depth == 1 && inInstr {
						instr.WriteString(el.Content)
					}
				case *wml.CT_Text:
					if depth >= 1 && !inInstr {
						result.WriteString(el.Content)
					}
				}
			}
			return true
		}
		for _, child := range p.p.Children {
			switch el := child.(type) {
			case *wml.CT_R:
				if !visit(el) {
					return
				}
			case *wml.CT_Hyperlink:
				for _, hc := range el.Children {
					if r, ok := hc.(*wml.CT_R); ok && !visit(r) {
						return
					}
				}
			}
		}
	}
}

// TOCOptions configures Document.AddTOC. The zero value is valid.
type TOCOptions struct {
	// Levels is the heading-level range to include, such as "1-3" (the default).
	Levels string
	// Hyperlinks makes each entry a link to its heading (the \h switch).
	Hyperlinks bool
}

// AddTOC appends a table of contents field in a new paragraph at the end of
// the body and returns it.
//
// The TOC shows placeholder text until Word updates it: the user right-clicks
// it, or the document asks Word to refresh all fields on open with
// SetUpdateFieldsOnOpen(true). The TOC1-TOC3 styles (and Hyperlink, when
// Hyperlinks is set) are added to the document if missing.
func (d *Document) AddTOC(opts TOCOptions) (*Field, error) {
	levels := opts.Levels
	if levels == "" {
		levels = "1-3"
	}
	if err := validateTOCLevels(levels); err != nil {
		return nil, err
	}
	instr := fmt.Sprintf(`TOC \o "%s"`, levels)
	if opts.Hyperlinks {
		instr += ` \h`
	}
	instr += ` \z \u`

	styles := d.Styles()
	for _, s := range []BuiltinStyle{StyleTOC1, StyleTOC2, StyleTOC3} {
		styles.EnsureBuiltin(s)
	}
	if opts.Hyperlinks {
		styles.EnsureBuiltin(StyleHyperlink)
	}
	return d.Body().AddParagraph().AddField(instr, tocPlaceholder), nil
}

func validateTOCLevels(levels string) error {
	if len(levels) != 3 || levels[1] != '-' {
		return fmt.Errorf("%w: %q (want a range such as \"1-3\")", ErrInvalidTOCLevels, levels)
	}
	lo, hi := levels[0], levels[2]
	if lo < '1' || lo > '9' || hi < '1' || hi > '9' || lo > hi {
		return fmt.Errorf("%w: %q (levels run from 1 to 9, low to high)", ErrInvalidTOCLevels, levels)
	}
	return nil
}

// UpdateFieldsOnOpen reports whether the document asks Word to refresh all
// fields when it opens (w:updateFields in settings).
func (d *Document) UpdateFieldsOnOpen() bool {
	if d.pkg.SetPart == nil {
		return false
	}
	b := d.pkg.SetPart.CT().UpdateFields()
	return b != nil && *b
}

// SetUpdateFieldsOnOpen asks Word to refresh all fields (TOC, page numbers,
// cross-references) when the document opens. Word shows a confirmation prompt
// first. It creates the settings part on first call.
func (d *Document) SetUpdateFieldsOnOpen(enabled bool) {
	v := enabled
	d.pkg.EnsureSettingsPart().CT().SetUpdateFields(&v)
}
