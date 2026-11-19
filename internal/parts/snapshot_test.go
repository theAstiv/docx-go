package parts

import (
	"bytes"
	"testing"

	"github.com/theAstiv/docx-go/internal/opc"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func saveBytes(t *testing.T, p *Package) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := p.Save(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Restore undoes images, links and styles added after the snapshot: the
// saved package is byte-identical to one saved before them.
func TestSnapshotRestore(t *testing.T) {
	p := New()
	if _, _, err := p.AddImage(PathDocument, []byte("first"), "png"); err != nil {
		t.Fatal(err)
	}
	before := saveBytes(t, p)
	stylesBefore := p.StyPart.Styles()
	rels := p.pkg.Rels(PathDocument)
	ct := p.pkg.ContentTypes

	s, err := p.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	// A new image (new media part, relationship and content type), a reused
	// one, an image on a header, a link and a style.
	if _, _, err := p.AddImage(PathDocument, []byte("second"), "jpeg"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.AddImage(PathDocument, []byte("first"), "png"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.AddImage("word/header1.xml", []byte("third"), "gif"); err != nil {
		t.Fatal(err)
	}
	p.AddExternalHyperlink(PathDocument, "https://example.com")
	p.StyPart.Styles().Styles = append(p.StyPart.Styles().Styles, &wml.CT_Style{})
	if bytes.Equal(saveBytes(t, p), before) {
		t.Fatal("the changes did not change the package")
	}

	p.Restore(s)
	if after := saveBytes(t, p); !bytes.Equal(after, before) {
		t.Error("restored package differs from the one saved before the changes")
	}
	// Pointers taken before the snapshot still reach the package.
	if p.StyPart.Styles() != stylesBefore || p.pkg.Rels(PathDocument) != rels || p.pkg.ContentTypes != ct {
		t.Error("restore replaced the styles tree, a relationship set or the content types")
	}
	// The media index was restored too: the second image is new again, and
	// the first is still deduplicated.
	if _, name, _ := p.AddImage(PathDocument, []byte("second"), "jpeg"); name != "image2.jpeg" {
		t.Errorf("re-added image stored as %q, want image2.jpeg", name)
	}
	if _, name, _ := p.AddImage(PathDocument, []byte("first"), "png"); name != "image1.png" {
		t.Errorf("first image stored as %q, want image1.png", name)
	}
	if n := len(p.pkg.Rels(PathDocument).ByType(opc.RelTypeImage)); n != 2 {
		t.Errorf("%d image relationships, want 2", n)
	}
}

// A style object that existed at the snapshot is the same object after a
// restore, with its old contents; styles added since are gone.
func TestSnapshotRestore_StyleIdentity(t *testing.T) {
	p := New()
	kept := &wml.CT_Style{}
	p.StyPart.Styles().Styles = append(p.StyPart.Styles().Styles, kept)
	s, err := p.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	p.StyPart.Styles().Styles = append(p.StyPart.Styles().Styles, &wml.CT_Style{})
	kept.PPr = &wml.CT_PPr{}
	p.Restore(s)
	got := p.StyPart.Styles().Styles
	if len(got) != 1 || got[0] != kept {
		t.Fatalf("styles after restore = %v, want just the original object", got)
	}
	if kept.PPr != nil {
		t.Error("the style's contents were not put back")
	}
}
