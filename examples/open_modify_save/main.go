// Command open_modify_save demonstrates opening an existing .docx, modifying
// properties on existing paragraphs and runs, and saving the result.
package main

import (
	"log"
	"os"

	"github.com/theAstiv/docx-go/docx"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: open_modify_save <input.docx> [output.docx]")
	}
	inputPath := os.Args[1]
	outputPath := "modified.docx"
	if len(os.Args) >= 3 {
		outputPath = os.Args[2]
	}

	f, err := os.Open(inputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}

	d, err := docx.Open(f, fi.Size())
	if err != nil {
		log.Fatal(err)
	}

	body := d.Body()

	// Walk the existing paragraphs and apply formatting changes.
	var pIdx int
	for p := range body.Paragraphs() {
		switch pIdx {
		case 0:
			// Centre-align the first paragraph.
			p.Format().SetAlignment(docx.AlignCenter)
		case 1:
			// Add 12pt space before the second paragraph.
			before := docx.Pt(12)
			p.Format().SetSpaceBefore(&before)
		}

		// Bold the first run of every paragraph (if it has one).
		for r := range p.Runs() {
			r.Font().SetBold(docx.On())
			break
		}

		pIdx++
	}
	log.Printf("modified %d existing paragraph(s)", pIdx)

	// Append a new paragraph marking the edit.
	p := body.AddParagraph()
	p.AddRun("— edited with ")
	lib := p.AddRun("docx-go")
	lib.Font().SetItalic(docx.On())
	lib.Font().SetColor("4472C4") // Word blue

	out, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := d.Save(out); err != nil {
		out.Close()
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
	log.Printf("written %s", outputPath)
}
