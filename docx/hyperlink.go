package docx

import (
	"errors"
	"fmt"
	"iter"
	"net/url"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Errors returned when creating hyperlinks. Test with errors.Is.
var (
	// ErrInvalidURL reports an external link target that is not an absolute
	// URL, or that uses a scheme that can run code (javascript:, vbscript:, data:).
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
}

// Hyperlink is a proxy for a w:hyperlink element.
type Hyperlink struct {
	h  *wml.CT_Hyperlink
	st *story
}

// validateHyperlinkURL accepts only absolute URLs with a safe scheme. The
// string itself is stored unchanged; it is never re-encoded.
func validateHyperlinkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %q: %w", ErrInvalidURL, raw, err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("%w: %q is not absolute (no scheme)", ErrInvalidURL, raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "javascript", "vbscript", "data":
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
	return &Hyperlink{h: h, st: p.st}
}

// AddHyperlink appends an external link to url and returns its proxy.
// Add the visible text with Hyperlink.AddRun. url must be absolute with a safe
// scheme (https:, http:, mailto:, ...); otherwise ErrInvalidURL is returned.
// The relationship is created on the part that holds the paragraph, so links
// work in headers and footers too, and identical URLs share one relationship.
func (p *Paragraph) AddHyperlink(target string, opts HyperlinkOptions) (*Hyperlink, error) {
	if err := validateHyperlinkURL(target); err != nil {
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
		for _, c := range p.p.Children {
			if h, ok := c.(*wml.CT_Hyperlink); ok {
				if !yield(&Hyperlink{h: h, st: p.st}) {
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
	run := &Run{r: r, st: h.st}
	run.Font().SetStyle(string(StyleHyperlink))
	return run
}

// Runs iterates over the runs directly inside the link.
func (h *Hyperlink) Runs() iter.Seq[*Run] {
	return func(yield func(*Run) bool) {
		for _, c := range h.h.Children {
			if r, ok := c.(*wml.CT_R); ok {
				if !yield(&Run{r: r, st: h.st}) {
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
