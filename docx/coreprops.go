package docx

import (
	"strconv"
	"strings"
	"time"

	"github.com/theAstiv/docx-go/internal/oxml/docprops"
	"github.com/theAstiv/docx-go/internal/parts"
)

// CoreProperties is a proxy for the document's core properties
// (docProps/core.xml): title, author, dates and the like, shown by Word
// under File › Info and used by search and document management systems.
//
// Getters return "" (or false) when a property is not set, and never
// change the document. Setters create the part when the document has none;
// an empty string or a zero time removes the property. Properties are not
// updated automatically on Save: set Modified and LastModifiedBy yourself
// if you want them to reflect the change.
type CoreProperties struct {
	pkg *parts.Package
}

// CoreProperties returns the proxy for the document's core properties.
func (d *Document) CoreProperties() *CoreProperties {
	return &CoreProperties{pkg: d.pkg}
}

func (c *CoreProperties) text(name docprops.Name) string {
	ct := c.pkg.CoreProps()
	if ct == nil {
		return ""
	}
	s, _ := ct.Text(name)
	return s
}

func (c *CoreProperties) setText(name docprops.Name, v string) {
	if v == "" {
		if c.pkg.CoreProps() == nil {
			return // nothing to remove; do not create the part
		}
		c.pkg.EditCoreProps().Remove(name)
		return
	}
	c.pkg.EditCoreProps().SetText(name, v)
}

func (c *CoreProperties) time(name docprops.Name) (time.Time, bool) {
	ct := c.pkg.CoreProps()
	if ct == nil {
		return time.Time{}, false
	}
	return ct.Time(name)
}

func (c *CoreProperties) setTime(name docprops.Name, t time.Time) {
	if t.IsZero() {
		if c.pkg.CoreProps() == nil {
			return
		}
		c.pkg.EditCoreProps().Remove(name)
		return
	}
	c.pkg.EditCoreProps().SetTime(name, t)
}

// Title returns the document title (dc:title).
func (c *CoreProperties) Title() string { return c.text(docprops.Title) }

// SetTitle sets the document title. "" removes it.
func (c *CoreProperties) SetTitle(s string) { c.setText(docprops.Title, s) }

// Subject returns the document subject (dc:subject).
func (c *CoreProperties) Subject() string { return c.text(docprops.Subject) }

// SetSubject sets the document subject. "" removes it.
func (c *CoreProperties) SetSubject(s string) { c.setText(docprops.Subject, s) }

// Author returns the document's author (dc:creator).
func (c *CoreProperties) Author() string { return c.text(docprops.Creator) }

// SetAuthor sets the document's author. "" removes it.
func (c *CoreProperties) SetAuthor(s string) { c.setText(docprops.Creator, s) }

// Keywords returns the keywords (cp:keywords) as one string; Word separates
// them with commas or semicolons.
func (c *CoreProperties) Keywords() string { return c.text(docprops.Keywords) }

// SetKeywords sets the keywords. "" removes them.
func (c *CoreProperties) SetKeywords(s string) { c.setText(docprops.Keywords, s) }

// Description returns the description, which Word calls Comments
// (dc:description).
func (c *CoreProperties) Description() string { return c.text(docprops.Description) }

// SetDescription sets the description. "" removes it.
func (c *CoreProperties) SetDescription(s string) { c.setText(docprops.Description, s) }

// Category returns the category (cp:category).
func (c *CoreProperties) Category() string { return c.text(docprops.Category) }

// SetCategory sets the category. "" removes it.
func (c *CoreProperties) SetCategory(s string) { c.setText(docprops.Category, s) }

// LastModifiedBy returns who last saved the document (cp:lastModifiedBy).
func (c *CoreProperties) LastModifiedBy() string { return c.text(docprops.LastModifiedBy) }

// SetLastModifiedBy sets who last saved the document. "" removes it.
func (c *CoreProperties) SetLastModifiedBy(s string) { c.setText(docprops.LastModifiedBy, s) }

// Revision returns the revision number (cp:revision), or 0 when it is not
// set or not a number.
func (c *CoreProperties) Revision() int {
	n, err := strconv.Atoi(strings.TrimSpace(c.text(docprops.Revision)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// SetRevision sets the revision number. 0 or less removes it.
func (c *CoreProperties) SetRevision(n int) {
	if n <= 0 {
		c.setText(docprops.Revision, "")
		return
	}
	c.setText(docprops.Revision, strconv.Itoa(n))
}

// Created returns when the document was created (dcterms:created), and
// false when that is not set or not a valid date.
func (c *CoreProperties) Created() (time.Time, bool) { return c.time(docprops.Created) }

// SetCreated sets the creation time, written in UTC to the second. A zero
// time removes it.
func (c *CoreProperties) SetCreated(t time.Time) { c.setTime(docprops.Created, t) }

// Modified returns when the document was last modified (dcterms:modified),
// and false when that is not set or not a valid date.
func (c *CoreProperties) Modified() (time.Time, bool) { return c.time(docprops.Modified) }

// SetModified sets the last-modified time, written in UTC to the second. A
// zero time removes it.
func (c *CoreProperties) SetModified(t time.Time) { c.setTime(docprops.Modified, t) }
