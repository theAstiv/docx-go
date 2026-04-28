// Command hello_world demonstrates creating a minimal .docx with docx-go.
package main

import (
	"log"
	"os"

	"github.com/theAstiv/docx-go/docx"
)

func main() {
	d := docx.New()
	body := d.Body()

	// Heading paragraph
	h := body.AddParagraph()
	h.Format().SetStyle("Heading1")
	h.AddRun("Hello, World!")

	// Normal paragraph with mixed formatting
	p := body.AddParagraph()
	p.Format().SetAlignment(docx.AlignJustify)
	p.AddRun("This is a ")
	bold := p.AddRun("bold")
	bold.Font().SetBold(docx.On())
	p.AddRun(" and ")
	italic := p.AddRun("italic")
	italic.Font().SetItalic(docx.On())
	p.AddRun(" paragraph.")

	// Paragraph with spacing
	spaced := body.AddParagraph()
	before := docx.Pt(12)
	spaced.Format().SetSpaceBefore(&before)
	spaced.AddRun("Paragraph with 12pt space before.")

	f, err := os.Create("hello.docx")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := d.Save(f); err != nil {
		log.Fatal(err)
	}
	log.Println("Written hello.docx")
}
