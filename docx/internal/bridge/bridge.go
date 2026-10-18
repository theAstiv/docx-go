package bridge

import itemplate "github.com/theAstiv/docx-go/internal/template"

// Stories returns the stories (body, headers, footers) of doc, which is a
// *docx.Document. It takes any because this package cannot import docx:
// docx imports it to set the variable. Set by package docx in init.
var Stories func(doc any) []itemplate.Story
