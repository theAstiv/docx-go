package bridge

import (
	"github.com/theAstiv/docx-go/internal/oxml/wml"
	itemplate "github.com/theAstiv/docx-go/internal/template"
)

// Stories returns the stories (body, headers, footers) of doc, which is a
// *docx.Document. It takes any because this package cannot import docx:
// docx imports it to set the variable. Set by package docx in init.
var Stories func(doc any) []itemplate.Story

// UnparsedParts returns the bytes of doc's footnotes, endnotes and comments
// parts, which docx keeps opaque. Set by package docx in init.
var UnparsedParts func(doc any) map[string][]byte

// BoundParts returns the bytes of the parts data-bound content controls read
// from (docProps/core.xml, customXml/item*.xml). Set by package docx in init.
var BoundParts func(doc any) map[string][]byte

// SetPart replaces the bytes of one of doc's opaque parts. Set by package
// docx in init.
var SetPart func(doc any, path string, data []byte)

// AddImage stores an image in doc, with its relationship on the part at
// path part, and returns the inline picture to place, with docPrID as its
// wp:docPr id. Set by package docx in init.
var AddImage func(doc any, part string, img itemplate.ImageValue, docPrID int64) (*wml.CT_Drawing, error)

// Snapshot records the state of doc's package that adding images and links
// changes, and returns the function that restores it. Set by package docx
// in init.
var Snapshot func(doc any) (restore func(), err error)
