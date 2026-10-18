package template

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const rawNS = testNS + ` xmlns:wp="` + ns.WP + `" xmlns:a="` + ns.A + `" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:mc="` + ns.MC + `"`

// story parses src as body content and returns it as the document story.
func story(t *testing.T, src string) (*wml.CT_Body, []Story) {
	t.Helper()
	var b wml.CT_Body
	if err := xml.Unmarshal([]byte(`<w:body `+rawNS+`>`+src+`</w:body>`), &b); err != nil {
		t.Fatal(err)
	}
	return &b, []Story{{Part: "word/document.xml", Blocks: &b.Children}}
}

func bodyXML(t *testing.T, b *wml.CT_Body) string {
	t.Helper()
	out, err := xml.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// A cover page in a block content control.
const coverSdt = `<w:sdt><w:sdtPr><w:docPartObj><w:docPartGallery w:val="Cover Pages"/></w:docPartObj></w:sdtPr>` +
	`<w:sdtContent><w:p><w:r><w:t>{{client.name}}</w:t></w:r></w:p>` +
	`<w:p><w:r><w:t xml:space="preserve">Prepared for {{client.name}} </w:t></w:r></w:p></w:sdtContent></w:sdt>`

// A text box in an anchored drawing, in both mc:Choice and mc:Fallback.
const textBox = `<w:p><w:r><mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:anchor>` +
	`<a:graphic><a:graphicData><wps:wsp><wps:txbx><w:txbxContent><w:p><w:r><w:t>{{title}}</w:t></w:r></w:p>` +
	`</w:txbxContent></wps:txbx></wps:wsp></a:graphicData></a:graphic></wp:anchor></w:drawing></mc:Choice>` +
	`<mc:Fallback><w:pict><w:t>{{title}}</w:t></w:pict></mc:Fallback></mc:AlternateContent></w:r></w:p>`

func TestRaw_ContentControlAndTextBox(t *testing.T) {
	b, stories := story(t, coverSdt+textBox)
	data := map[string]any{"client": map[string]any{"name": "Acme & Co"}, "title": "Gap assessment"}
	warnings, err := Execute(stories, data, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: %v", warnings)
	}
	out := bodyXML(t, b)
	if strings.Contains(out, "{{") {
		t.Errorf("tags left:\n%s", out)
	}
	for _, want := range []string{">Acme &amp; Co<", "Prepared for Acme &amp; Co ", ">Gap assessment<"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "Gap assessment"); n != 2 {
		t.Errorf("text box filled %d times, want 2 (Choice and Fallback)", n)
	}
}

func TestRaw_LoopScope(t *testing.T) {
	b, stories := story(t, `<w:p><w:r><w:t>{{#each items}}</w:t></w:r></w:p>`+
		`<w:sdt><w:sdtContent><w:p><w:r><w:t>{{.name}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`+
		`<w:p><w:r><w:t>{{/each}}</w:t></w:r></w:p>`)
	data := map[string]any{"items": []any{map[string]any{"name": "one"}, map[string]any{"name": "two"}}}
	if _, err := Execute(stories, data, Options{}); err != nil {
		t.Fatal(err)
	}
	out := bodyXML(t, b)
	if !strings.Contains(out, ">one<") || !strings.Contains(out, ">two<") {
		t.Errorf("loop items not filled in content controls:\n%s", out)
	}
}

func TestRaw_Unsupported(t *testing.T) {
	cases := []struct {
		name, src string
		data      map[string]any
		mode      Mode
		wantErr   bool
	}{
		{"split tag, strict",
			`<w:sdt><w:sdtContent><w:p><w:r><w:t>{{na</w:t></w:r><w:r><w:rPr><w:b/></w:rPr><w:t>me}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`,
			nil, Strict, true},
		{"split tag, lenient",
			`<w:sdt><w:sdtContent><w:p><w:r><w:t>{{na</w:t></w:r><w:r><w:t>me}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`,
			nil, Lenient, false},
		{"block tag, lenient still fails",
			`<w:sdt><w:sdtContent><w:p><w:r><w:t>{{#if x}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`,
			nil, Lenient, true},
		{"multi-line value, strict",
			`<w:sdt><w:sdtContent><w:p><w:r><w:t>{{v}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`,
			map[string]any{"v": "a\nb"}, Strict, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, stories := story(t, c.src)
			warnings, err := Execute(stories, c.data, Options{Mode: c.mode})
			if c.wantErr {
				if !errors.Is(err, ErrUnsupportedLocation) {
					t.Errorf("err = %v, want ErrUnsupportedLocation", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if len(warnings) == 0 || !errors.Is(warnings[0].Err, ErrUnsupportedLocation) {
				t.Errorf("warnings = %v, want an unsupported-location warning", warnings)
			}
		})
	}
}

func TestRaw_MultiLineLenientUsesSpaces(t *testing.T) {
	b, stories := story(t, `<w:sdt><w:sdtContent><w:p><w:r><w:t>{{v}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`)
	if _, err := Execute(stories, map[string]any{"v": "a\nb\tc"}, Options{Mode: Lenient}); err != nil {
		t.Fatal(err)
	}
	if out := bodyXML(t, b); !strings.Contains(out, ">a b c<") {
		t.Errorf("got:\n%s", out)
	}
}

func TestRaw_Placeholders(t *testing.T) {
	_, stories := story(t, `<w:p><w:r><w:t>{{a}}</w:t></w:r></w:p>`+coverSdt+textBox)
	ps, issues, err := Placeholders(stories)
	if err != nil || len(issues) != 0 {
		t.Fatalf("err %v, issues %v", err, issues)
	}
	var got []string
	for _, p := range ps {
		got = append(got, p.Path+"@"+p.Location.Path)
	}
	want := "a@p[0] client.name@sdt[0] client.name@sdt[0] title@p[1] title@p[1]"
	if strings.Join(got, " ") != want {
		t.Errorf("got  %s\nwant %s", strings.Join(got, " "), want)
	}
}

func TestScanUnparsed(t *testing.T) {
	issues := ScanUnparsed(map[string][]byte{
		"word/footnotes.xml": []byte(`<w:footnotes><w:t>See {{ref}}</w:t></w:footnotes>`),
		"word/comments.xml":  []byte(`<w:comments/>`),
	})
	if len(issues) != 1 || issues[0].Location.Part != "word/footnotes.xml" || issues[0].Severity != SeverityWarning {
		t.Errorf("issues = %v", issues)
	}
}
