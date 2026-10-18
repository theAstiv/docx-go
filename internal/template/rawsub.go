package template

import (
	"bytes"
	"encoding/xml"
	"sort"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Content the element layer does not model (content controls, text boxes,
// tracked changes, smart tags) reaches the engine as raw token streams.
// Branded cover pages usually live there, so value tags inside it are filled
// when they sit wholly inside one w:t, which is how a tag typed in one go
// and never re-formatted is stored. Everything else there (a split tag, a
// block or image tag, a multi-line value) is reported: an error in Strict
// mode, a warning in Lenient mode, except block and image tags, which are
// template errors in both modes.

const rawLocation = "inside a content control, text box or tracked change"

// rawIssue reports a problem in raw content per the mode.
func (en *engine) rawIssue(loc Location, text, detail string) {
	is := issueErr(loc, text, ErrUnsupportedLocation, detail)
	if en.opts.Mode == Lenient {
		is.Severity = SeverityWarning
	}
	en.issues = append(en.issues, is)
}

// fillRaw fills the value tags in every w:t of a raw subtree.
func (en *engine) fillRaw(re *raw.RawElement, sc *scope, loc Location) {
	if re == nil {
		return
	}
	if re.QName == ns.Qn("w:t") {
		en.fillRawText(re, sc, loc)
		return
	}
	for _, k := range re.Children {
		if c, ok := k.(*raw.RawElement); ok {
			en.fillRaw(c, sc, loc)
		}
	}
}

// fillDrawing fills the raw parts of a drawing that can hold a text box:
// an anchored drawing (raw in Rest) or an inline shape's graphic data.
func (en *engine) fillDrawing(d *wml.CT_Drawing, sc *scope, loc Location) {
	for _, re := range d.Rest {
		en.fillRaw(re, sc, loc)
	}
	if in := d.Inline; in != nil && in.Graphic != nil && in.Graphic.GraphicData != nil {
		for _, re := range in.Graphic.GraphicData.Rest {
			en.fillRaw(re, sc, loc)
		}
	}
}

func (en *engine) fillRawText(re *raw.RawElement, sc *scope, loc Location) {
	text := rawText(re)
	if !strings.Contains(text, "{{") {
		return
	}
	tags, bad := lexTags(text)
	for _, b := range bad {
		en.rawIssue(loc, b.text, "a tag split across formatting "+rawLocation+" cannot be filled; retype it in one piece")
	}
	if len(tags) == 0 {
		return
	}
	var sb strings.Builder
	pos := 0
	for _, t := range tags {
		sb.WriteString(text[pos:t.start])
		pos = t.end
		if t.kind != tagValue {
			en.fail(loc, t, ErrUnsupportedLocation, "block and image tags are not supported "+rawLocation)
			sb.WriteString(t.text)
			continue
		}
		v := en.value(t, sc, loc)
		if strings.ContainsAny(v, "\r\n\t") {
			en.rawIssue(loc, t.text, "line breaks and tabs cannot be written "+rawLocation+"; they were replaced by spaces")
			v = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(v)
		}
		sb.WriteString(v)
	}
	sb.WriteString(text[pos:])
	out := sb.String()
	re.Children = []xml.Token{xml.CharData(out)}
	if out != strings.TrimSpace(out) {
		setRawPreserve(re)
	}
}

// rawText returns the text content of a raw w:t.
func rawText(re *raw.RawElement) string {
	var sb strings.Builder
	for _, k := range re.Children {
		if cd, ok := k.(xml.CharData); ok {
			sb.Write(cd)
		}
	}
	return sb.String()
}

// setRawPreserve marks a raw w:t xml:space="preserve".
func setRawPreserve(re *raw.RawElement) {
	name := xml.Name{Space: ns.XML, Local: "space"}
	for i, a := range re.Attrs {
		if a.Name.Local == "space" {
			re.Attrs[i].Value = "preserve"
			return
		}
	}
	re.Attrs = append(re.Attrs, xml.Attr{Name: name, Value: "preserve"})
}

// rawTexts returns the w:t text nodes of a raw subtree that contain "{{".
func rawTexts(re *raw.RawElement) []string {
	if re == nil {
		return nil
	}
	if re.QName == ns.Qn("w:t") {
		if t := rawText(re); strings.Contains(t, "{{") {
			return []string{t}
		}
		return nil
	}
	var out []string
	for _, k := range re.Children {
		if c, ok := k.(*raw.RawElement); ok {
			out = append(out, rawTexts(c)...)
		}
	}
	return out
}

// ScanUnparsed reports, as warnings, parts the engine cannot fill that
// contain "{{": footnotes, endnotes and comments are kept as opaque bytes.
func ScanUnparsed(parts map[string][]byte) []Issue {
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []Issue
	for _, name := range names {
		if bytes.Contains(parts[name], []byte("{{")) {
			is := issueErr(Location{Part: name}, "", ErrUnsupportedLocation,
				"tags in footnotes, endnotes and comments are not filled")
			is.Severity = SeverityWarning
			out = append(out, is)
		}
	}
	return out
}
