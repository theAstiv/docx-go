// Command iterate_paragraphs walks every paragraph and run in a .docx and
// prints the text content with basic formatting annotations to stdout.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/theAstiv/docx-go/docx"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: iterate_paragraphs <input.docx>")
	}

	f, err := os.Open(os.Args[1])
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

	pIdx := 0
	for p := range d.Body().Paragraphs() {
		pIdx++
		fmt.Printf("Paragraph %d: %q\n", pIdx, p.Text())

		rIdx := 0
		for r := range p.Runs() {
			rIdx++
			props := runProps(r.Font())
			if len(props) == 0 {
				fmt.Printf("  Run %d: %q\n", rIdx, r.Text())
			} else {
				fmt.Printf("  Run %d: %q [%s]\n", rIdx, r.Text(), strings.Join(props, ", "))
			}
		}
	}
	fmt.Printf("\n%d paragraph(s) total\n", pIdx)
}

// runProps returns a slice of human-readable property labels for the run.
func runProps(f *docx.RunFont) []string {
	var props []string
	if b := f.Bold(); b != nil && *b {
		props = append(props, "bold")
	}
	if i := f.Italic(); i != nil && *i {
		props = append(props, "italic")
	}
	if s := f.Strike(); s != nil && *s {
		props = append(props, "strike")
	}
	if u := f.Underline(); u != "" {
		props = append(props, "underline="+string(u))
	}
	if c := f.Color(); c != "" {
		props = append(props, "color=#"+c)
	}
	if pt := f.SizePt(); pt != 0 {
		props = append(props, fmt.Sprintf("%.1fpt", pt))
	}
	return props
}
