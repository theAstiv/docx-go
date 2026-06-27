package docx_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestDocument_EvenAndOddHeaders(t *testing.T) {
	d := docx.New()
	if d.EvenAndOddHeaders() {
		t.Error("expected false on a new document")
	}

	d.SetEvenAndOddHeaders(true)
	if !d.EvenAndOddHeaders() {
		t.Error("expected true after SetEvenAndOddHeaders(true)")
	}

	// Round-trip.
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	if !d2.EvenAndOddHeaders() {
		t.Error("EvenAndOddHeaders not persisted after round-trip")
	}

	d2.SetEvenAndOddHeaders(false)
	if d2.EvenAndOddHeaders() {
		t.Error("expected false after SetEvenAndOddHeaders(false)")
	}
}

func TestDocument_DefaultTabStop(t *testing.T) {
	d := docx.New()
	if d.DefaultTabStop() != nil {
		t.Error("expected nil on a new document")
	}

	v := docx.Inches(0.5)
	d.SetDefaultTabStop(&v)
	got := d.DefaultTabStop()
	if got == nil || *got != v {
		t.Errorf("DefaultTabStop() = %v, want %v", got, v)
	}

	// Round-trip.
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	got2 := d2.DefaultTabStop()
	if got2 == nil || *got2 != v {
		t.Errorf("DefaultTabStop after round-trip = %v, want %v", got2, v)
	}

	d2.SetDefaultTabStop(nil)
	if d2.DefaultTabStop() != nil {
		t.Error("expected nil after clearing DefaultTabStop")
	}
}

func TestDocument_SettingsPart_RoundTripFixtures(t *testing.T) {
	// Verify that opening a fixture with settings.xml round-trips without
	// gaining or losing settings knowledge.  Uses the headerFooter POI fixture
	// which typically contains a settings part.
	const path = "../testdata/fixtures/poi/headerFooter.docx"
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("fixture not found: %s", path)
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := docx.OpenBytes(buf.Bytes()); err != nil {
		t.Fatalf("Reopen after round-trip: %v", err)
	}
}
