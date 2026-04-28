// Command docxsave opens a .docx with docx-go and saves it to a new file.
// Used to generate round-tripped outputs for downstream validation.
//
// Usage:
//
//	docxsave <input.docx> <output.docx>
package main

import (
	"log"
	"os"

	"github.com/theAstiv/docx-go/docx"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: docxsave <input.docx> <output.docx>")
	}

	f, err := os.Open(os.Args[1]) // #nosec G304 — path is the explicit CLI argument
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

	out, err := os.Create(os.Args[2]) // #nosec G304 G306 — path is the explicit CLI argument
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
}
