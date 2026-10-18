package docx

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestIDCache_ScansOnce(t *testing.T) {
	d := New()
	for i := range 5 {
		if _, err := d.Body().AddParagraph().AddBookmark("bm" + string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	if d.pkg.IDCache == nil {
		t.Fatal("scan not cached")
	}
	cached := d.pkg.IDCache
	if _, err := d.Body().AddParagraph().AddBookmark("bmz"); err != nil {
		t.Fatal(err)
	}
	if d.pkg.IDCache != cached {
		t.Error("AddBookmark rescanned instead of extending the cache")
	}

	// Ids stay unique and match what a fresh walk finds.
	seen := map[int64]bool{}
	fresh := docStory(d.pkg).freshScan()
	if len(fresh.Bookmarks) != 6 || len(cached.Bookmarks) != 6 {
		t.Fatalf("bookmarks: fresh %d, cached %d, want 6", len(fresh.Bookmarks), len(cached.Bookmarks))
	}
	for _, b := range fresh.Bookmarks {
		if seen[b.ID] {
			t.Errorf("duplicate bookmark id %d", b.ID)
		}
		seen[b.ID] = true
	}
}

func TestIDCache_DuplicateNameStillRejected(t *testing.T) {
	d := New()
	if _, err := d.Body().AddParagraph().AddBookmark("intro"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Body().AddParagraph().AddBookmark("intro"); err == nil {
		t.Error("duplicate bookmark name accepted")
	}
}

// Removing content leaves the cache stale; a name that is only in the cache
// must not be refused.
func TestIDCache_NameFreedByRemovalIsReusable(t *testing.T) {
	d := New()
	cell := firstCell(d)
	if _, err := cell.SetText("x").AddBookmark("gone"); err != nil {
		t.Fatal(err)
	}
	cell.SetText("replaced") // drops the bookmarked paragraph

	if _, err := d.Body().AddParagraph().AddBookmark("gone"); err != nil {
		t.Errorf("name freed by removal refused: %v", err)
	}
}

func TestIDCache_InvalidatedByTemplateStories(t *testing.T) {
	d := New()
	if _, err := d.Body().AddParagraph().AddBookmark("a"); err != nil {
		t.Fatal(err)
	}
	if d.pkg.IDCache == nil {
		t.Fatal("scan not cached")
	}
	templateStories(d)
	if d.pkg.IDCache != nil {
		t.Error("cache survived handing out writable stories")
	}
}

func TestIDCache_DocPrIDsUnique(t *testing.T) {
	d := New()
	st := docStory(d.pkg)
	// Scan, then allocate several ids as AddImage does.
	var ids []int64
	for range 4 {
		id := st.nextDocPrID()
		st.noteDocPr(id)
		ids = append(ids, id)
	}
	for i, id := range ids {
		if id != int64(i+1) {
			t.Errorf("ids = %v, want 1..4", ids)
			break
		}
	}
	_ = wml.IDScan{} // keep the wml import tied to the scan type under test
}
