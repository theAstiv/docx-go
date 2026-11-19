package template

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/theAstiv/docx-go/internal/textmerge"
)

// Barrier characters stand in for non-text content when a paragraph's text
// is flattened for lexing (see textmerge.Flatten).
const (
	// barrierBreak marks a tab, line break or other run content that a tag
	// cannot contain.
	barrierBreak = textmerge.BarrierBreak
	// barrierBoundary marks a field character, bookmark, hyperlink edge,
	// content control or other element a tag must not be split across.
	barrierBoundary = textmerge.BarrierBoundary
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
	// tagElse is {{else}}: the start of a block's other branch.
	tagElse
)

// tag is one well-formed tag found in flattened text.
type tag struct {
	kind tagKind
	// block is "each", "if" or "unless" for open and close tags.
	block string
	// path is the data path: "client.name", ".field" or ".".
	path string
	// calls are the formatters a value tag pipes its value through, in
	// order: {{total | number 2 | default "—"}}.
	calls []call
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
		if strings.Contains(rest, "|") {
			return t, formattersOnlyInValues
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
		// "else" is a keyword: {{else}} never names a data key ({{.else}}
		// still does inside a loop).
		if b == "else" {
			t.kind = tagElse
			return t, ""
		}
		if rest, ok := strings.CutPrefix(b, "image "); ok {
			rest = strings.TrimSpace(rest)
			if strings.Contains(rest, "|") {
				return t, formattersOnlyInValues
			}
			if !validPath(rest) {
				return t, "invalid path " + quote(rest)
			}
			t.kind, t.path = tagImage, rest
			return t, ""
		}
		path, calls, detail := parseValueBody(b)
		if detail != "" {
			return t, detail
		}
		t.kind, t.path, t.calls = tagValue, path, calls
	}
	return t, ""
}

const formattersOnlyInValues = "formatters are only allowed in value tags"

// call is one formatter in a value tag's pipe: a name and its arguments,
// each a string or a float64.
type call struct {
	name string
	args []any
}

// closingQuote maps each opening quote to the quote that closes it. Word
// turns typed straight quotes into curly ones, so both count.
var closingQuote = map[rune]rune{'"': '"', '\'': '\'', '“': '”', '‘': '’'}

// parseValueBody parses "path | name arg … | name arg …". It returns a
// non-empty detail when the body is invalid.
func parseValueBody(b string) (path string, calls []call, detail string) {
	segs, detail := splitPipes(b)
	if detail != "" {
		return "", nil, detail
	}
	path = strings.TrimSpace(segs[0])
	if !validPath(path) {
		return "", nil, "invalid path " + quote(path)
	}
	for _, seg := range segs[1:] {
		c, detail := parseCall(seg)
		if detail != "" {
			return "", nil, detail
		}
		calls = append(calls, c)
	}
	return path, calls, ""
}

// splitPipes splits s at every "|" outside quotes.
func splitPipes(s string) ([]string, string) {
	var segs []string
	start := 0
	var closer rune
	for i, c := range s {
		switch {
		case closer != 0:
			if c == closer {
				closer = 0
			}
		case c == '|':
			segs = append(segs, s[start:i])
			start = i + 1
		default:
			if q, ok := closingQuote[c]; ok {
				closer = q
			}
		}
	}
	if closer != 0 {
		return nil, "unclosed quote"
	}
	return append(segs, s[start:]), ""
}

// parseCall parses one pipe segment: a formatter name, then arguments that
// are quoted strings or numbers, separated by spaces.
func parseCall(seg string) (call, string) {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return call{}, `empty formatter after "|"`
	}
	end := strings.IndexFunc(seg, unicode.IsSpace)
	if end < 0 {
		end = len(seg)
	}
	c := call{name: seg[:end]}
	if !validIdent(c.name) || strings.Contains(c.name, "-") {
		return call{}, "invalid formatter name " + quote(c.name)
	}
	rest := seg[end:]
	for {
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
		if rest == "" {
			return c, ""
		}
		open, size := utf8.DecodeRuneInString(rest)
		if q, ok := closingQuote[open]; ok {
			n := strings.IndexRune(rest[size:], q)
			if n < 0 {
				return call{}, "unclosed quote" // splitPipes rejects this first
			}
			c.args = append(c.args, rest[size:size+n])
			rest = rest[size+n+utf8.RuneLen(q):]
			if rest != "" {
				if r, _ := utf8.DecodeRuneInString(rest); !unicode.IsSpace(r) {
					return call{}, "arguments of " + c.name + " must be separated by spaces"
				}
			}
			continue
		}
		n := strings.IndexFunc(rest, unicode.IsSpace)
		if n < 0 {
			n = len(rest)
		}
		word := rest[:n]
		f, ok := parseNumber(word)
		if !ok {
			return call{}, "argument " + quote(word) + " of " + c.name + " must be a quoted string or a number"
		}
		c.args = append(c.args, f)
		rest = rest[n:]
	}
}

// parseNumber accepts decimal numbers such as 2, -1.5 and 1e3, and nothing
// strconv would also read (NaN, Inf, hex).
func parseNumber(s string) (float64, bool) {
	for _, c := range s {
		if !strings.ContainsRune("+-.0123456789eE", c) {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
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
