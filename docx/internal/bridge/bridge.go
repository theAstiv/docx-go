package bridge

import itemplate "github.com/theAstiv/docx-go/internal/template"

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
