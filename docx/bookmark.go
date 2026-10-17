package docx

import (
	"errors"
	"fmt"
	"iter"
	"unicode"
	"unicode/utf8"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// maxBookmarkNameLen is Word's limit on bookmark name length, in characters.
const maxBookmarkNameLen = 40

// Errors returned by Paragraph.AddBookmark. Test with errors.Is.
var (
	// ErrInvalidBookmarkName reports a name Word would reject: it must start
	// with a letter, contain only letters, digits and underscores, and be at
	// most 40 characters long.
	ErrInvalidBookmarkName = errors.New("docx: invalid bookmark name")
	// ErrReservedBookmarkName reports a name starting with an underscore.
	// Those are reserved for bookmarks Word generates itself (such as _Toc…).
	ErrReservedBookmarkName = errors.New("docx: reserved bookmark name")
	// ErrDuplicateBookmark reports a name already used by another bookmark
	// anywhere in the document.
	ErrDuplicateBookmark = errors.New("docx: duplicate bookmark name")
)

// Bookmark is a proxy for a w:bookmarkStart element.
type Bookmark struct {
	bs *wml.CT_BookmarkStart
}

// Name returns the bookmark name.
func (b *Bookmark) Name() string { return b.bs.Name }

// validateBookmarkName checks a name for creation. Reading existing bookmarks
// is never validated, so documents with Word-generated names stay usable.
func validateBookmarkName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidBookmarkName)
	}
	if name[0] == '_' {
		return fmt.Errorf("%w: %q", ErrReservedBookmarkName, name)
	}
	if utf8.RuneCountInString(name) > maxBookmarkNameLen {
		return fmt.Errorf("%w: %q is longer than %d characters", ErrInvalidBookmarkName, name, maxBookmarkNameLen)
	}
	for i, r := range name {
		switch {
		case unicode.IsLetter(r):
		case i > 0 && (unicode.IsDigit(r) || r == '_'):
		default:
			return fmt.Errorf("%w: %q has an illegal character %q", ErrInvalidBookmarkName, name, r)
		}
	}
	return nil
}

// bookmarkStarts collects every bookmark in the package, in story order.
//
// Bookmarks inside raw content (content controls, tracked changes, smart
// tags, block-level bookmarks) have no typed element; they are returned as
// detached read-only copies carrying the id and name, which is all that
// duplicate checks, id allocation and Document.Bookmarks need.
func (s *story) bookmarkStarts() []*wml.CT_BookmarkStart {
	refs := s.scan().Bookmarks
	out := make([]*wml.CT_BookmarkStart, 0, len(refs))
	for _, ref := range refs {
		if ref.Start != nil {
			out = append(out, ref.Start)
			continue
		}
		bs := new(wml.CT_BookmarkStart)
		bs.ID = ref.ID
		bs.Name = ref.Name
		out = append(out, bs)
	}
	return out
}

// nextBookmarkID returns an id one above the largest in use, or 0 when the
// document has no bookmarks. w:id must be unique across all stories.
func (s *story) nextBookmarkID() int64 {
	next := int64(0)
	for _, b := range s.bookmarkStarts() {
		if b.ID >= next {
			next = b.ID + 1
		}
	}
	return next
}

// AddBookmark wraps the whole paragraph in a bookmark with the given name.
//
// The name must satisfy Word's rules (see ErrInvalidBookmarkName), must not
// start with an underscore (ErrReservedBookmarkName) and must be unique in the
// document (ErrDuplicateBookmark). Bookmarks spanning several paragraphs are
// not supported yet.
func (p *Paragraph) AddBookmark(name string) (*Bookmark, error) {
	if err := validateBookmarkName(name); err != nil {
		return nil, err
	}
	for _, b := range p.st.bookmarkStarts() {
		if b.Name == name {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateBookmark, name)
		}
	}
	id := p.st.nextBookmarkID()

	bs := new(wml.CT_BookmarkStart)
	bs.ID = id
	bs.Name = name
	be := new(wml.CT_BookmarkEnd)
	be.ID = id

	children := make([]wml.InlineElement, 0, len(p.p.Children)+2)
	children = append(children, bs)
	children = append(children, p.p.Children...)
	children = append(children, be)
	p.p.Children = children
	return &Bookmark{bs: bs}, nil
}

// Bookmark returns the bookmark with the given name, or nil.
func (d *Document) Bookmark(name string) *Bookmark {
	for _, b := range docStory(d.pkg).bookmarkStarts() {
		if b.Name == name {
			return &Bookmark{bs: b}
		}
	}
	return nil
}

// Bookmarks iterates over every bookmark in the document (body, headers and
// footers), including Word-generated ones such as _Toc… and _GoBack.
func (d *Document) Bookmarks() iter.Seq[*Bookmark] {
	return func(yield func(*Bookmark) bool) {
		for _, b := range docStory(d.pkg).bookmarkStarts() {
			if !yield(&Bookmark{bs: b}) {
				return
			}
		}
	}
}
