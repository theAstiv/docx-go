package parts

import "testing"

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
		if back := resolveRelPath(c.source, got); back != c.part {
			t.Errorf("resolveRelPath(%q, %q) = %q, want %q", c.source, got, back, c.part)
		}
	}
}
