package parts

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/opc"
)

func TestRelativeTarget(t *testing.T) {
	cases := []struct {
		source, part, want string
	}{
		{"word/document.xml", "word/media/image1.png", "media/image1.png"},
		{"word/header1.xml", "word/media/image1.png", "media/image1.png"},
		// A part outside word/ climbs out of its own directory.
		{"glossary/document.xml", "word/media/image1.png", "../word/media/image1.png"},
		{"word/glossary/document.xml", "word/media/image1.png", "../media/image1.png"},
		// A document stored at the package root.
		{"document.xml", "word/media/image1.png", "word/media/image1.png"},
	}
	for _, c := range cases {
		got, err := relativeTarget(c.source, c.part)
		if err != nil {
			t.Errorf("relativeTarget(%q, %q): %v", c.source, c.part, err)
			continue
		}
		if got != c.want {
			t.Errorf("relativeTarget(%q, %q) = %q, want %q", c.source, c.part, got, c.want)
		}
		// The target must resolve back to the part.
		if back, ok := resolveRelPath(c.source, got); !ok || back != c.part {
			t.Errorf("resolveRelPath(%q, %q) = %q, %v; want %q", c.source, got, back, ok, c.part)
		}
	}
}

func TestResolveTarget(t *testing.T) {
	cases := []struct {
		source, target, mode string
		want                 string // "" = names no part
	}{
		{"word/document.xml", "styles.xml", "", "word/styles.xml"},
		{"word/document.xml", "../media/img.png", "", "media/img.png"},
		{"word/document.xml", "/word/styles.xml", "", "word/styles.xml"},
		{"word/document.xml", "/word/../docProps/core.xml", "", "docProps/core.xml"},
		{"", "docProps/core.xml", "", "docProps/core.xml"},
		{"word/document.xml", "media/50%.png", "", "word/media/50%.png"},
		// H3: nothing outside the package root, no empty or folder names.
		{"word/document.xml", "../../../evil.xml", "", ""},
		{"", "../x", "", ""},
		{"word/document.xml", "", "", ""},
		{"word/document.xml", "/", "", ""},
		{"word/document.xml", "a./b.xml", "", ""},
		{"word/document.xml", `media\img.png`, "", ""},
		// URIs and network paths are not part names.
		{"", "http://x/y", "", ""},
		{"word/document.xml", "file:///c:/x.xml", "", ""},
		{"word/document.xml", "//host/x.xml", "", ""},
		// Names the package writes itself.
		{"word/document.xml", "_rels/document.xml.rels", "", ""},
		{"", "[Content_Types].xml", "", ""},
		// L14: an external relationship never names a part.
		{"word/document.xml", "styles.xml", "External", ""},
		{"word/document.xml", "styles.xml", "external", ""},
	}
	for _, c := range cases {
		r := opc.Relationship{ID: "rId1", Type: opc.RelTypeStyles, Target: c.target, TargetMode: c.mode}
		got, ok := resolveTarget(c.source, r)
		if ok != (c.want != "") || got != c.want {
			t.Errorf("resolveTarget(%q, %q, mode %q) = %q, %v; want %q", c.source, c.target, c.mode, got, ok, c.want)
		}
	}
}
