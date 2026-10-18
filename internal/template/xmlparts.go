package template

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"sort"
	"strings"
)

// A data-bound content control (w:dataBinding) shows the value stored in
// another part: a document property in docProps/core.xml, or an element of
// a custom XML item. When a template author types a tag into such a control,
// Word writes the tag into that part too, and when it opens a file it
// re-syncs the control from the part. Filling only the visible text would
// leave the tag in the part, and Word would show it again. So value tags in
// those parts are filled as well, from the root data.
//
// Only text nodes change; every other byte is copied as it was, so the
// part's prefixes and declarations are untouched (re-encoding foreign XML
// with encoding/xml would mangle them, golang/go#9519).

// fillXMLParts fills the tags in each part and returns the parts that
// changed.
func (en *engine) fillXMLParts(parts map[string][]byte, sc *scope) map[string][]byte {
	out := map[string][]byte{}
	for _, name := range sortedNames(parts) {
		src := parts[name]
		if !bytes.Contains(src, []byte("{{")) {
			continue
		}
		filled, changed := en.fillXMLPart(name, src, sc)
		if changed {
			out[name] = filled
		}
	}
	return out
}

func (en *engine) fillXMLPart(name string, src []byte, sc *scope) ([]byte, bool) {
	loc := Location{Part: name}
	var out bytes.Buffer
	written := int64(0)
	changed := false
	eachText(src, func(start, end int64, text string) {
		tags, bad := lexTags(text)
		for _, b := range bad {
			en.issues = append(en.issues, issueErr(loc, b.text, b.err, b.detail))
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
				en.fail(loc, t, ErrUnsupportedLocation, "only value tags are supported in document properties and bound data")
				sb.WriteString(t.text)
				continue
			}
			// A property holds one line of text.
			v := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(en.value(t, sc, loc))
			sb.WriteString(v)
		}
		sb.WriteString(text[pos:])
		out.Write(src[written:start])
		if err := xml.EscapeText(&out, []byte(sb.String())); err != nil && en.err == nil {
			en.err = err
		}
		written = end
		changed = true
	})
	out.Write(src[written:])
	return out.Bytes(), changed
}

// eachText calls fn for every text node of src that contains "{{", with
// its byte range in src and its decoded text. It stops quietly at the first
// syntax error: a part Word wrote is well formed, and an unparsable one is
// left as it was.
func eachText(src []byte, fn func(start, end int64, text string)) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) || err != nil {
			return
		}
		if cd, ok := tok.(xml.CharData); ok && bytes.Contains(cd, []byte("{{")) {
			fn(start, dec.InputOffset(), string(cd))
		}
	}
}

// xmlPartPlaceholders lists the tags in the parts' text nodes.
func xmlPartPlaceholders(parts map[string][]byte) ([]Placeholder, []Issue) {
	var ps []Placeholder
	var issues []Issue
	for _, name := range sortedNames(parts) {
		loc := Location{Part: name}
		eachText(parts[name], func(_, _ int64, text string) {
			tags, bad := lexTags(text)
			for _, b := range bad {
				issues = append(issues, issueErr(loc, b.text, b.err, b.detail))
			}
			for _, t := range tags {
				ps = append(ps, Placeholder{Kind: kindName(t), Path: t.path, Tag: t.text, Location: loc})
			}
		})
	}
	return ps, issues
}

func sortedNames(m map[string][]byte) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
