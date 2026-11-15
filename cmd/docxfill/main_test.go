package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

const (
	tplScalars = "../../testdata/fixtures/word/template_scalars.docx"
	tplData    = "../../testdata/template/word-templates.json"
)

func runCmd(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestFill(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.docx")
	code, _, stderr := runCmd(t, "", "-o", out, tplScalars, tplData)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	data, err := os.ReadFile(out) // #nosec G304 -- test temp file
	if err != nil {
		t.Fatal(err)
	}
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	text := d.Text()
	if strings.Contains(text, "{{") || !strings.Contains(text, "Acme Health & Co") {
		t.Errorf("not filled:\n%s", text)
	}
	// No temporary files are left next to the output.
	entries, _ := os.ReadDir(filepath.Dir(out))
	if len(entries) != 1 {
		t.Errorf("output directory holds %d entries, want 1", len(entries))
	}
}

func TestFill_StdinAndStdout(t *testing.T) {
	data, err := os.ReadFile(tplData)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runCmd(t, string(data), "-o", "-", tplScalars, "-")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := docx.OpenBytes([]byte(stdout)); err != nil {
		t.Errorf("stdout is not a .docx: %v", err)
	}
}

// Missing keys fail by default, listing every one, and write nothing;
// -lenient fills them with empty text and warns.
func TestFill_MissingKeys(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.docx")
	code, _, stderr := runCmd(t, `{"client": {}}`, "-o", out, tplScalars, "-")
	if code != exitTemplate {
		t.Fatalf("exit %d, want %d: %s", code, exitTemplate, stderr)
	}
	if !strings.Contains(stderr, "client.name") || !strings.Contains(stderr, "not written") {
		t.Errorf("stderr does not list the missing keys:\n%s", stderr)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("output written although filling failed")
	}

	code, _, stderr = runCmd(t, `{"client": {}}`, "-lenient", "-o", out, tplScalars, "-")
	if code != exitOK {
		t.Fatalf("-lenient: exit %d: %s", code, stderr)
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("-lenient printed no warnings:\n%s", stderr)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("-lenient wrote no output: %v", err)
	}
}

func TestPlaceholders(t *testing.T) {
	code, stdout, stderr := runCmd(t, "", "-placeholders", tplScalars)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "value\tclient.name\t{{client.name}}\tword/document.xml") {
		t.Errorf("placeholders:\n%s", stdout)
	}
}

func TestUsageErrors(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name  string
		stdin string
		args  []string
	}{
		{"no args", "", nil},
		{"missing -o", "", []string{tplScalars, tplData}},
		{"unknown flag", "", []string{"-x"}},
		{"missing template", "", []string{"-o", filepath.Join(dir, "o.docx"), filepath.Join(dir, "none.docx"), tplData}},
		{"not a docx", "", []string{"-o", filepath.Join(dir, "o.docx"), tplData, tplData}},
		{"data not an object", "[1,2]", []string{"-o", filepath.Join(dir, "o.docx"), tplScalars, "-"}},
		{"data null", "null", []string{"-o", filepath.Join(dir, "o.docx"), tplScalars, "-"}},
		{"bad JSON", "{", []string{"-o", filepath.Join(dir, "o.docx"), tplScalars, "-"}},
		{"placeholders without file", "", []string{"-placeholders"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, _, _ := runCmd(t, c.stdin, c.args...); code != exitUsage {
				t.Errorf("exit %d, want %d", code, exitUsage)
			}
		})
	}
}
