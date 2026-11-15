// Command docxfill fills a Word template (.docx with {{tags}}) from JSON
// data, using docx-go's templating (package docx/template).
//
// Usage:
//
//	docxfill [-lenient] -o out.docx template.docx data.json
//	docxfill -placeholders template.docx
//
// data.json may be "-" to read standard input, and -o may be "-" to write
// the document to standard output. The JSON must be an object; numbers keep
// the form they are written in.
//
// By default a tag whose key is missing from the data is an error; with
// -lenient it is filled with empty text and reported as a warning.
// Warnings go to standard error. The output file is written only when
// filling succeeds, and never partially.
//
// Exit status: 0 on success (warnings may have been printed), 1 for a usage
// or file error, 2 when the template or its data has errors (each one is
// listed).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// Exit statuses.
const (
	exitOK       = 0
	exitUsage    = 1
	exitTemplate = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("docxfill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	lenient := fs.Bool("lenient", false, "fill missing keys with empty text instead of failing")
	out := fs.String("o", "", `output .docx ("-" for standard output)`)
	list := fs.Bool("placeholders", false, "list the template's tags and exit")
	fs.Usage = func() {
		say(stderr, "usage: docxfill [-lenient] -o out.docx template.docx data.json")
		say(stderr, "       docxfill -placeholders template.docx")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	if *list {
		if fs.NArg() != 1 {
			fs.Usage()
			return exitUsage
		}
		return placeholders(fs.Arg(0), stdout, stderr)
	}
	if fs.NArg() != 2 || *out == "" {
		fs.Usage()
		return exitUsage
	}

	doc, err := openDocx(fs.Arg(0))
	if err != nil {
		say(stderr, "docxfill:", err)
		return exitUsage
	}
	data, err := readData(fs.Arg(1), stdin)
	if err != nil {
		say(stderr, "docxfill:", err)
		return exitUsage
	}

	opts := template.Options{Mode: template.Strict}
	if *lenient {
		opts.Mode = template.Lenient
	}
	warnings, err := template.Execute(doc, data, opts)
	for _, w := range warnings {
		say(stderr, w)
	}
	var terr *template.Error
	if errors.As(err, &terr) {
		for _, is := range terr.Issues {
			say(stderr, is)
		}
		sayf(stderr, "docxfill: %d error(s); %s not written\n", len(terr.Issues), *out)
		return exitTemplate
	}
	if err != nil {
		say(stderr, "docxfill:", err)
		return exitUsage
	}

	if err := writeDocx(doc, *out, stdout); err != nil {
		say(stderr, "docxfill:", err)
		return exitUsage
	}
	return exitOK
}

func placeholders(path string, stdout, stderr io.Writer) int {
	doc, err := openDocx(path)
	if err != nil {
		say(stderr, "docxfill:", err)
		return exitUsage
	}
	ps, err := template.Placeholders(doc)
	for _, p := range ps {
		sayf(stdout, "%s\t%s\t%s\t%s\n", p.Kind, p.Path, p.Tag, p.Location)
	}
	var terr *template.Error
	if errors.As(err, &terr) {
		for _, is := range terr.Issues {
			say(stderr, is)
		}
		return exitTemplate
	}
	if err != nil {
		say(stderr, "docxfill:", err)
		return exitUsage
	}
	return exitOK
}

func openDocx(path string) (*docx.Document, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is the explicit CLI argument
	if err != nil {
		return nil, err
	}
	doc, err := docx.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

// readData decodes a JSON object from path, or from stdin when path is "-".
func readData(path string, stdin io.Reader) (map[string]any, error) {
	src := stdin
	name := "standard input"
	if path != "-" {
		b, err := os.ReadFile(path) // #nosec G304 -- path is the explicit CLI argument
		if err != nil {
			return nil, err
		}
		src, name = bytes.NewReader(b), path
	}
	dec := json.NewDecoder(src)
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("%s: %w (the data must be a JSON object)", name, err)
	}
	if data == nil {
		return nil, fmt.Errorf("%s: the data must be a JSON object, not null", name)
	}
	return data, nil
}

// writeDocx saves doc to path, or to stdout when path is "-". A file is
// written to a temporary name in the same directory and renamed into
// place, so a failure never leaves a partial document.
func writeDocx(doc *docx.Document, path string, stdout io.Writer) error {
	var buf bytes.Buffer
	if err := doc.Save(&buf); err != nil {
		return err
	}
	if path == "-" {
		_, err := stdout.Write(buf.Bytes())
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".docxfill-*.docx")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(buf.Bytes())
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

// say and sayf print a line of output. A failed write to the terminal or a
// closed pipe cannot be reported anywhere, so its error is dropped.
func say(w io.Writer, a ...any) { _, _ = fmt.Fprintln(w, a...) }

func sayf(w io.Writer, format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
