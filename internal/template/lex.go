package template

import (
	"strings"
)

// Barrier characters stand in for non-text content when a paragraph's text
// is flattened for lexing. Neither can occur in XML text, so they never
// collide with document content.
const (
	// barrierBreak marks a tab, line break or other run content that a tag
	// cannot contain.
	barrierBreak = '\x00'
	// barrierBoundary marks a field character, bookmark, hyperlink edge,
	// content control or other element a tag must not be split across.
	barrierBoundary = '\x01'
)

type tagKind int

const (
	// tagValue is {{path}}.
	tagValue tagKind = iota
	// tagOpen is {{#each path}}, {{#if path}} or {{#unless path}}.
	tagOpen
	// tagClose is {{/each}}, {{/if}} or {{/unless}}.
	tagClose
	// tagImage is {{image path}}.
	tagImage
)

// tag is one well-formed tag found in flattened text.
type tag struct {
	kind tagKind
	// block is "each", "if" or "unless" for open and close tags.
	block string
	// path is the data path: "client.name", ".field" or ".".
	path string
	// start and end are byte offsets of "{{" and just past "}}".
	start, end int
	// text is the tag as written.
	text string
}

// badTag is text that starts like a tag but is not one.
type badTag struct {
	start, end int
	text       string
	err        error
	detail     string
}

// lexTags finds every tag in s. Text that opens with "{{" but is not a valid
// tag is reported in bad; a lone "}}" is ordinary text.
func lexTags(s string) (tags []tag, bad []badTag) {
	pos := 0
	for {
		i := strings.Index(s[pos:], "{{")
		if i < 0 {
			return tags, bad
		}
		start := pos + i
		j := strings.Index(s[start+2:], "}}")
		if j < 0 {
			b := badTag{start: start, end: len(s), text: printable(s[start:]),
				err: ErrMalformedTag, detail: `no closing "}}"`}
			if strings.ContainsRune(s[start:], barrierBoundary) {
				// The closing braces are most likely inside the hyperlink,
				// field or content control that follows.
				b.err, b.detail = ErrSplitAcrossBoundary, `no closing "}}" before a hyperlink, field or other boundary; retype the tag in one piece`
			}
			bad = append(bad, b)
			return tags, bad
		}
		end := start + 2 + j + 2
		body := s[start+2 : end-2]
		text := s[start:end]
		pos = end

		switch {
		case strings.ContainsRune(body, barrierBoundary):
			bad = append(bad, badTag{start: start, end: end, text: printable(text),
				err: ErrSplitAcrossBoundary, detail: "accept tracked changes and retype the tag in one piece"})
			continue
		case strings.ContainsRune(body, barrierBreak):
			bad = append(bad, badTag{start: start, end: end, text: printable(text),
				err: ErrMalformedTag, detail: "a tag cannot contain a tab or line break"})
			continue
		case strings.Contains(body, "{{"):
			bad = append(bad, badTag{start: start, end: end, text: text,
				err: ErrMalformedTag, detail: `"{{" inside a tag`})
			continue
		}
		t, detail := parseTagBody(body)
		if detail != "" {
			bad = append(bad, badTag{start: start, end: end, text: text, err: ErrMalformedTag, detail: detail})
			continue
		}
		t.start, t.end, t.text = start, end, text
		tags = append(tags, t)
	}
}

// parseTagBody parses what sits between "{{" and "}}". It returns a non-empty
// detail when the body is invalid.
func parseTagBody(body string) (tag, string) {
	var t tag
	b := strings.TrimSpace(body)
	if b == "" {
		return t, "empty tag"
	}
	switch b[0] {
	case '#':
		word, rest, _ := strings.Cut(b[1:], " ")
		rest = strings.TrimSpace(rest)
		if !isBlockWord(word) {
			return t, `unknown block "#` + word + `" (want #each, #if or #unless)`
		}
		if rest == "" {
			return t, "#" + word + " needs a path"
		}
		if !validPath(rest) {
			return t, "invalid path " + quote(rest)
		}
		t.kind, t.block, t.path = tagOpen, word, rest
	case '/':
		word := strings.TrimSpace(b[1:])
		if !isBlockWord(word) {
			return t, `unknown block "/` + word + `" (want /each, /if or /unless)`
		}
		t.kind, t.block = tagClose, word
	default:
		if rest, ok := strings.CutPrefix(b, "image "); ok {
			rest = strings.TrimSpace(rest)
			if !validPath(rest) {
				return t, "invalid path " + quote(rest)
			}
			t.kind, t.path = tagImage, rest
			return t, ""
		}
		if !validPath(b) {
			return t, "invalid path " + quote(b)
		}
		t.kind, t.path = tagValue, b
	}
	return t, ""
}

func isBlockWord(w string) bool {
	return w == "each" || w == "if" || w == "unless"
}

// validPath accepts "a", "a.b.c", ".", ".a" and ".a.b".
func validPath(p string) bool {
	if p == "." {
		return true
	}
	p = strings.TrimPrefix(p, ".")
	if p == "" {
		return false
	}
	for _, seg := range strings.Split(p, ".") {
		if !validIdent(seg) {
			return false
		}
	}
	return true
}

// validIdent accepts [A-Za-z_][A-Za-z0-9_-]*.
func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case i > 0 && (c == '-' || c >= '0' && c <= '9'):
		default:
			return false
		}
	}
	return true
}

func quote(s string) string { return `"` + s + `"` }

// printable replaces barrier characters so a tag can be shown in a message.
func printable(s string) string {
	return strings.NewReplacer(string(barrierBreak), "⏎", string(barrierBoundary), "¦").Replace(s)
}
