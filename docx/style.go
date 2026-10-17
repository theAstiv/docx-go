package docx

import (
	"iter"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// StyleType identifies the kind of a style.
type StyleType string

// Style kinds, matching the w:type attribute.
const (
	StyleTypeParagraph StyleType = "paragraph"
	StyleTypeCharacter StyleType = "character"
	StyleTypeTable     StyleType = "table"
	StyleTypeNumbering StyleType = "numbering"
)

// BuiltinStyle names a style that Styles.EnsureBuiltin can create.
type BuiltinStyle string

// Built-in styles. The value is the styleId Word writes for the style.
const (
	StyleNormal    BuiltinStyle = wml.StyleIDNormal
	StyleTitle     BuiltinStyle = wml.StyleIDTitle
	StyleHeading1  BuiltinStyle = wml.StyleIDHeading1
	StyleHeading2  BuiltinStyle = wml.StyleIDHeading2
	StyleHeading3  BuiltinStyle = wml.StyleIDHeading3
	StyleHyperlink BuiltinStyle = wml.StyleIDHyperlink
	StyleTableGrid BuiltinStyle = wml.StyleIDTableGrid
	StyleTOC1      BuiltinStyle = wml.StyleIDTOC1
	StyleTOC2      BuiltinStyle = wml.StyleIDTOC2
	StyleTOC3      BuiltinStyle = wml.StyleIDTOC3
	StyleCaption   BuiltinStyle = wml.StyleIDCaption
)

// StyleOptions configures a new style. The zero value is valid: no parent, no
// follow-on style, default priority and not offered in the quick-style gallery.
type StyleOptions struct {
	// BasedOn is the styleId this style inherits from; "" for none.
	BasedOn string
	// Next is the styleId applied to the paragraph after Enter; "" for same style.
	Next string
	// UIPriority orders the style in Word's UI; nil leaves it unset.
	UIPriority *int64
	// QuickStyle shows the style in the quick-style gallery (w:qFormat).
	QuickStyle bool
}

// Styles is a proxy for the document's styles part.
type Styles struct {
	st *wml.CT_Styles
}

// Styles returns the document's style collection.
func (d *Document) Styles() *Styles {
	return &Styles{st: d.pkg.StyPart.Styles()}
}

// ByID returns the style with the given styleId, or nil.
func (s *Styles) ByID(id string) *Style {
	if st := s.st.ByID(id); st != nil {
		return &Style{s: st}
	}
	return nil
}

// All iterates over every style in document order.
func (s *Styles) All() iter.Seq[*Style] {
	return func(yield func(*Style) bool) {
		for _, st := range s.st.Styles {
			if !yield(&Style{s: st}) {
				return
			}
		}
	}
}

// AddParagraphStyle adds a paragraph style, or returns the existing style
// when id is already in use (it is never overwritten).
func (s *Styles) AddParagraphStyle(id, name string, opts StyleOptions) *Style {
	return s.add(id, name, StyleTypeParagraph, opts)
}

// AddCharacterStyle adds a character style, or returns the existing style
// when id is already in use.
func (s *Styles) AddCharacterStyle(id, name string, opts StyleOptions) *Style {
	return s.add(id, name, StyleTypeCharacter, opts)
}

// AddTableStyle adds a table style, or returns the existing style when id is
// already in use. Table-level properties (borders, cell margins) arrive with
// table formatting support; Format and Font style the table's content.
func (s *Styles) AddTableStyle(id, name string, opts StyleOptions) *Style {
	return s.add(id, name, StyleTypeTable, opts)
}

func (s *Styles) add(id, name string, typ StyleType, opts StyleOptions) *Style {
	if existing := s.ByID(id); existing != nil {
		return existing
	}
	st := new(wml.CT_Style)
	st.StyleID = id
	st.Type = string(typ)
	st.Name = name
	st.CustomStyle = true
	st.SetBasedOn(opts.BasedOn)
	st.SetNext(opts.Next)
	st.SetUIPriority(opts.UIPriority)
	if opts.QuickStyle {
		st.SetQFormat(On())
	}
	s.st.Styles = append(s.st.Styles, st)
	return &Style{s: st}
}

// EnsureBuiltin returns the built-in style, inserting a Word-compatible
// definition (and the styles it is based on) when the document lacks it.
// An existing definition is returned untouched, so user edits are kept.
// It returns nil for a name that is not a known built-in.
//
// An existing style is found by styleId, then by display name (so a template
// that calls heading 1 "Überschrift 1" under another id is still matched when
// its w:name is the English built-in name), and for Normal, Default Paragraph
// Font and Normal Table by being the default style of that type. A German
// template whose default paragraph style is "Standard" therefore never gets a
// second default. Because the returned style's ID can differ from the
// BuiltinStyle constant, use Style.ID() when referencing it from paragraphs
// or runs.
func (s *Styles) EnsureBuiltin(name BuiltinStyle) *Style {
	id := string(name)
	def := wml.NewBuiltinStyle(id)
	if def == nil {
		return nil
	}
	if existing := s.findBuiltin(def); existing != nil {
		return existing
	}
	// Create the style this one is based on first, then point at whatever id
	// it ended up with (it may be a template's existing style).
	for _, dep := range wml.BuiltinStyleDeps(id) {
		resolved := s.EnsureBuiltin(BuiltinStyle(dep))
		if resolved == nil || resolved.ID() == dep {
			continue
		}
		if def.BasedOn() == dep {
			def.SetBasedOn(resolved.ID())
		}
		if def.Next() == dep {
			def.SetNext(resolved.ID())
		}
	}
	s.st.Styles = append(s.st.Styles, def)
	return &Style{s: def}
}

// findBuiltin locates the document's existing equivalent of def: same id, or
// same type and display name, or (for the three default styles) the default
// style of def's type.
func (s *Styles) findBuiltin(def *wml.CT_Style) *Style {
	if existing := s.ByID(def.StyleID); existing != nil {
		return existing
	}
	for _, st := range s.st.Styles {
		if st.Type == def.Type && strings.EqualFold(st.Name, def.Name) {
			return &Style{s: st}
		}
	}
	if def.Default {
		for _, st := range s.st.Styles {
			if st.Type == def.Type && st.Default {
				return &Style{s: st}
			}
		}
	}
	return nil
}

// DocDefaultsFont returns a RunFont proxy for the document-wide default run
// properties (w:docDefaults/w:rPrDefault). Calling it creates the empty
// defaults element if the document has none.
func (s *Styles) DocDefaultsFont() *RunFont {
	return &RunFont{slot: &s.docDefaults().RPr}
}

// DocDefaultsFormat returns a ParagraphFormat proxy for the document-wide
// default paragraph properties (w:docDefaults/w:pPrDefault). Calling it
// creates the empty defaults element if the document has none.
func (s *Styles) DocDefaultsFormat() *ParagraphFormat {
	return &ParagraphFormat{slot: &s.docDefaults().PPr}
}

func (s *Styles) docDefaults() *wml.CT_DocDefaults {
	if s.st.DocDefaults == nil {
		s.st.DocDefaults = new(wml.CT_DocDefaults)
	}
	return s.st.DocDefaults
}

// Style is a proxy for a single w:style definition.
type Style struct {
	s *wml.CT_Style
}

// ID returns the styleId, the value paragraphs and runs reference.
func (s *Style) ID() string { return s.s.StyleID }

// Name returns the display name.
func (s *Style) Name() string { return s.s.Name }

// Type returns the style kind.
func (s *Style) Type() StyleType { return StyleType(s.s.Type) }

// BasedOn returns the parent styleId, or "".
func (s *Style) BasedOn() string { return s.s.BasedOn() }

// SetBasedOn sets the parent style. An empty string removes it.
func (s *Style) SetBasedOn(id string) { s.s.SetBasedOn(id) }

// Next returns the styleId applied to the following paragraph, or "".
func (s *Style) Next() string { return s.s.Next() }

// SetNext sets the follow-on style. An empty string removes it.
func (s *Style) SetNext(id string) { s.s.SetNext(id) }

// QuickStyle returns the qFormat tri-state, or nil when unset.
func (s *Style) QuickStyle() *bool { return s.s.QFormat() }

// SetQuickStyle sets the qFormat tri-state; nil removes it.
func (s *Style) SetQuickStyle(b *bool) { s.s.SetQFormat(b) }

// UIPriority returns the UI sort priority, or nil when unset.
func (s *Style) UIPriority() *int64 { return s.s.UIPriority() }

// SetUIPriority sets the UI sort priority; nil removes it.
func (s *Style) SetUIPriority(p *int64) { s.s.SetUIPriority(p) }

// Format returns a ParagraphFormat proxy over the style's w:pPr. The element
// is created lazily when a setter is called.
func (s *Style) Format() *ParagraphFormat { return &ParagraphFormat{slot: &s.s.PPr} }

// Font returns a RunFont proxy over the style's w:rPr. The element is created
// lazily when a setter is called.
func (s *Style) Font() *RunFont { return &RunFont{slot: &s.s.RPr} }
