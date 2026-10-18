package template

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Content the element layer does not model (content controls, text boxes,
// tracked changes, smart tags) reaches the engine as raw token streams.
// Branded cover pages usually live there, and Word splits tags there as
// freely as anywhere else. So every w:p inside raw content is parsed into a
// typed paragraph, merged and filled exactly like body text, and written
// back (the round-trip rule makes that lossless). Sibling paragraphs are
// handled together, so a table of contents in a content control is skipped
// like one in the body. Block and image tags are not supported there.
//
// A w:t outside any paragraph (rare) is still filled when a tag sits wholly
// inside it; anything else there is reported per the mode.

const rawLocation = "inside a content control, text box or tracked change"

// rawIssue reports a problem in raw content per the mode.
func (en *engine) rawIssue(loc Location, text, detail string) {
	is := issueErr(loc, text, ErrUnsupportedLocation, detail)
	if en.opts.Mode == Lenient {
		is.Severity = SeverityWarning
	}
	en.issues = append(en.issues, is)
}

// fillRaw fills the tags of a raw subtree.
func (en *engine) fillRaw(re *raw.RawElement, sc *scope, loc Location) {
	if re == nil {
		return
	}
	if re.QName == ns.Qn("w:t") {
		en.fillRawText(re, sc, loc)
		return
	}
	err := walkRawParagraphs(re, loc, true, func(paras []*wml.CT_P, ploc Location) {
		for t := range tocSkipsParagraphs(paras) {
			en.skip[t] = true
		}
		wasRaw := en.inRaw
		en.inRaw = true
		for i, p := range paras {
			ploc := ploc.child("p", i)
			var bad []badTag
			p.Children, bad = normalizeInlines(p.Children, en.skip)
			for _, b := range bad {
				en.issues = append(en.issues, issueErr(ploc, b.text, b.err, b.detail))
			}
			en.fillParagraph(p, sc, ploc)
		}
		en.inRaw = wasRaw
	}, func(t *raw.RawElement) { en.fillRawText(t, sc, loc) })
	if err != nil && en.err == nil {
		en.err = err
	}
}

// walkRawParagraphs finds every group of sibling w:p elements in a raw
// subtree, parses each group into typed paragraphs and calls fn with it.
// With write set, the paragraphs are written back into the tree afterwards,
// so changes fn makes are kept. A w:t met outside any paragraph goes to
// loose.
func walkRawParagraphs(re *raw.RawElement, loc Location, write bool,
	fn func([]*wml.CT_P, Location), loose func(*raw.RawElement)) error {
	var idx []int
	var paras []*wml.CT_P
	for i, k := range re.Children {
		c, ok := k.(*raw.RawElement)
		if !ok {
			continue
		}
		switch c.QName {
		case ns.Qn("w:p"):
			p, err := rawToParagraph(c)
			if err != nil {
				return err
			}
			idx = append(idx, i)
			paras = append(paras, p)
		case ns.Qn("w:t"):
			loose(c)
		default:
			if !holdsRuns(c) {
				if err := walkRawParagraphs(c, loc, write, fn, loose); err != nil {
					return err
				}
				continue
			}
			// An inline container (the w:sdtContent of a content control
			// inside a paragraph, w:ins, w:smartTag) holds runs directly:
			// type its children as one paragraph's content.
			p, err := rawToParagraph(&raw.RawElement{QName: ns.Qn("w:p"), Attrs: nil, Children: c.Children})
			if err != nil {
				return err
			}
			fn([]*wml.CT_P{p}, loc)
			if write {
				back, err := paragraphToRaw(p)
				if err != nil {
					return err
				}
				c.Children = back.Children
			}
		}
	}
	if len(paras) == 0 {
		return nil
	}
	fn(paras, loc)
	if write {
		for j, p := range paras {
			r, err := paragraphToRaw(p)
			if err != nil {
				return err
			}
			re.Children[idx[j]] = r
		}
	}
	return nil
}

// holdsRuns reports whether a raw element has w:r children of its own.
func holdsRuns(re *raw.RawElement) bool {
	for _, k := range re.Children {
		if c, ok := k.(*raw.RawElement); ok && c.QName == ns.Qn("w:r") {
			return true
		}
	}
	return false
}

// rawToParagraph parses a raw w:p into a typed paragraph.
func rawToParagraph(re *raw.RawElement) (*wml.CT_P, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := re.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("template: raw paragraph: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("template: raw paragraph: %w", err)
	}
	p := new(wml.CT_P)
	if err := xml.Unmarshal(buf.Bytes(), p); err != nil {
		return nil, fmt.Errorf("template: raw paragraph: %w", err)
	}
	return p, nil
}

// paragraphToRaw writes a typed paragraph back as a raw w:p.
func paragraphToRaw(p *wml.CT_P) (*raw.RawElement, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := p.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("template: raw paragraph: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("template: raw paragraph: %w", err)
	}
	re := new(raw.RawElement)
	if err := xml.Unmarshal(buf.Bytes(), re); err != nil {
		return nil, fmt.Errorf("template: raw paragraph: %w", err)
	}
	return re, nil
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
