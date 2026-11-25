package docx

import (
	"errors"
	"fmt"
	"iter"
	"net/url"
	"slices"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Errors returned when creating hyperlinks. Test with errors.Is.
var (
	// ErrInvalidURL reports an external link target that is not an absolute
	// URL, or whose scheme is not allowed: only http:, https: and mailto:
	// unless HyperlinkOptions.AllowAnyScheme is set, and never javascript:,
	// vbscript: or data:.
	ErrInvalidURL = errors.New("docx: invalid hyperlink URL")
	// ErrEmptyAnchor reports an internal link without a bookmark name.
	ErrEmptyAnchor = errors.New("docx: empty hyperlink anchor")
)

// HyperlinkOptions configures a new hyperlink. The zero value is valid.
type HyperlinkOptions struct {
	// Tooltip is the screen tip shown on hover; "" for none.
	Tooltip string
	// History controls whether Word records the link as visited (w:history);
	// nil leaves the attribute unset.
	History *bool
	// AllowAnyScheme accepts URL schemes besides http:, https: and mailto:
	// (ftp:, tel:, file:, an application's own scheme), still refusing
	// javascript:, vbscript: and data:. Schemes such as file:, ms-msdt: and
	// search-ms: can leak Windows credentials or start programs when the
	// link is clicked, so set it only for URLs the program chose itself,
	// never for URLs from user input or template data.
	AllowAnyScheme bool
}

// Hyperlink is a proxy for a w:hyperlink element.
type Hyperlink struct {
	h  *wml.CT_Hyperlink
	st *story
	// para is the paragraph the link was found in or added to.
	para *Paragraph
	// gen is para's childGen when the link was found or added; while it is
	// current, the link is in the document (locate.go).
	gen uint64
}

// safeSchemes are the URL schemes a link accepts by default: an allow-list,
// since Windows registers handlers (ms-msdt:, search-ms:, file: to a remote
// share) that a deny-list cannot keep up with.
var safeSchemes = map[string]bool{"http": true, "https": true, "mailto": true}

// codeSchemes run code in the reader's context; never accepted.
var codeSchemes = map[string]bool{"javascript": true, "vbscript": true, "data": true}

// validateHyperlinkURL accepts only absolute URLs with an allowed scheme:
// safeSchemes, or with anyScheme any scheme except codeSchemes. The string
// itself is stored unchanged; it is never re-encoded.
func validateHyperlinkURL(raw string, anyScheme bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %q: %w", ErrInvalidURL, raw, err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("%w: %q is not absolute (no scheme)", ErrInvalidURL, raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if codeSchemes[scheme] || (!anyScheme && !safeSchemes[scheme]) {
		return fmt.Errorf("%w: %q uses scheme %q", ErrInvalidURL, raw, u.Scheme)
	}
	return nil
}

func (p *Paragraph) appendHyperlink(h *wml.CT_Hyperlink, opts HyperlinkOptions) *Hyperlink {
	h.SetTooltip(opts.Tooltip)
	h.SetHistory(opts.History)
	p.p.Children = append(p.p.Children, h)
	// Links look like links only once the Hyperlink character style exists.
	if p.st != nil && p.st.pkg.StyPart != nil {
		(&Styles{st: p.st.pkg.StyPart.Styles()}).EnsureBuiltin(StyleHyperlink)
	}
	return &Hyperlink{h: h, st: p.st, para: p, gen: p.childGen()}
}

// AddHyperlink appends an external link to url and returns its proxy.
// Add the visible text with Hyperlink.AddRun. url must be absolute with an
// http:, https: or mailto: scheme (see HyperlinkOptions.AllowAnyScheme for
// others); otherwise ErrInvalidURL is returned.
// The relationship is created on the part that holds the paragraph, so links
// work in headers and footers too, and identical URLs share one relationship.
func (p *Paragraph) AddHyperlink(target string, opts HyperlinkOptions) (*Hyperlink, error) {
	if err := validateHyperlinkURL(target, opts.AllowAnyScheme); err != nil {
		return nil, err
	}
	h := new(wml.CT_Hyperlink)
	h.RID = p.st.pkg.AddExternalHyperlink(p.st.src, target)
	return p.appendHyperlink(h, opts), nil
}

// AddInternalLink appends a link to the bookmark with the given name and
// returns its proxy. The bookmark need not exist yet, and Word-generated
// names (_Toc…) are accepted; only an empty name is refused (ErrEmptyAnchor).
func (p *Paragraph) AddInternalLink(bookmark string, opts HyperlinkOptions) (*Hyperlink, error) {
	if bookmark == "" {
		return nil, ErrEmptyAnchor
	}
	h := new(wml.CT_Hyperlink)
	h.Anchor = bookmark
	return p.appendHyperlink(h, opts), nil
}

// Hyperlinks iterates over the hyperlinks that are direct children of the
// paragraph.
func (p *Paragraph) Hyperlinks() iter.Seq[*Hyperlink] {
	return func(yield func(*Hyperlink) bool) {
		for _, c := range slices.Clone(p.p.Children) {
			if h, ok := c.(*wml.CT_Hyperlink); ok {
				if !yield(&Hyperlink{h: h, st: p.st, para: p, gen: p.childGen()}) {
					return
				}
			}
		}
	}
}

// AddRun appends a run containing text to the link, styled with the
// Hyperlink character style, and returns its proxy.
func (h *Hyperlink) AddRun(text string) *Run {
	r := new(wml.CT_R)
	if text != "" {
		t := new(wml.CT_Text)
		t.Content = text
		t.Space = "preserve"
		r.Children = append(r.Children, t)
	}
	h.h.Children = append(h.h.Children, r)
	run := &Run{r: r, st: h.st, parent: &h.h.Children, gen: h.childGen()}
	// Use the document's own hyperlink style id, which a localized or custom
	// template may have under a different name than "Hyperlink".
	styleID := string(StyleHyperlink)
	if h.st != nil && h.st.pkg.StyPart != nil {
		if s := (&Styles{st: h.st.pkg.StyPart.Styles()}).EnsureBuiltin(StyleHyperlink); s != nil {
			styleID = s.ID()
		}
	}
	run.Font().SetStyle(styleID)
	return run
}

// Runs iterates over the runs directly inside the link.
func (h *Hyperlink) Runs() iter.Seq[*Run] {
	return func(yield func(*Run) bool) {
		for _, c := range slices.Clone(h.h.Children) {
			if r, ok := c.(*wml.CT_R); ok {
				if !yield(&Run{r: r, st: h.st, parent: &h.h.Children, gen: h.childGen()}) {
					return
				}
			}
		}
	}
}

// URL returns the external target by resolving the link's relationship, or ""
// for an internal link or an unresolvable relationship.
func (h *Hyperlink) URL() string {
	if h.h.RID == "" || h.st == nil {
		return ""
	}
	target, _ := h.st.pkg.HyperlinkTarget(h.st.src, h.h.RID)
	return target
}

// Anchor returns the bookmark name of an internal link, or "".
func (h *Hyperlink) Anchor() string { return h.h.Anchor }

// Tooltip returns the screen tip, or "".
func (h *Hyperlink) Tooltip() string { return h.h.Tooltip() }

// Text returns the visible text of the link.
func (h *Hyperlink) Text() string { return h.h.Text() }
