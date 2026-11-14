package docprops_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/theAstiv/docx-go/internal/oxml/docprops"
	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
)

// wordCore is docProps/core.xml as Word writes it, plus an extension
// element and a property in a namespace this package does not know.
const wordCore = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:x="urn:example:ext">` +
	`<dc:title>Gap assessment &amp; DPIA</dc:title><dc:subject></dc:subject><dc:creator>A. Author</dc:creator>` +
	`<cp:keywords>privacy, dpdpa</cp:keywords><dc:description/><cp:lastModifiedBy>B. Editor</cp:lastModifiedBy>` +
	`<cp:revision>3</cp:revision><dcterms:created xsi:type="dcterms:W3CDTF">2026-10-01T09:30:00Z</dcterms:created>` +
	`<dcterms:modified xsi:type="dcterms:W3CDTF">2026-10-02T10:00:00Z</dcterms:modified>` +
	`<x:reviewed x:by="QA">yes</x:reviewed></cp:coreProperties>`

func parse(t *testing.T, s string) *docprops.CT_CoreProperties {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			c := new(docprops.CT_CoreProperties)
			if err := c.UnmarshalXML(dec, se); err != nil {
				t.Fatal(err)
			}
			return c
		}
	}
}

func marshal(t *testing.T, c *docprops.CT_CoreProperties) string {
	t.Helper()
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := c.MarshalXML(enc, xml.StartElement{}); err != nil {
		t.Fatal(err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestCoreProperties_RoundTrip(t *testing.T) {
	c := parse(t, wordCore)
	roundtrip.Verify(t, c, ns.Qn("cp:coreProperties"))
	roundtrip.VerifyBytes[*docprops.CT_CoreProperties](t, []byte(wordCore[strings.Index(wordCore, "<cp:"):]), ns.Qn("cp:coreProperties"))
}

func TestCoreProperties_Read(t *testing.T) {
	c := parse(t, wordCore)
	for name, want := range map[xml.Name]string{
		docprops.Title:          "Gap assessment & DPIA",
		docprops.Creator:        "A. Author",
		docprops.Keywords:       "privacy, dpdpa",
		docprops.LastModifiedBy: "B. Editor",
		docprops.Revision:       "3",
		docprops.Subject:        "",
	} {
		got, ok := c.Text(name)
		if !ok || got != want {
			t.Errorf("%s = %q, %v; want %q", name.Local, got, ok, want)
		}
	}
	if _, ok := c.Text(docprops.Category); ok {
		t.Error("Category reported present")
	}
	created, ok := c.Time(docprops.Created)
	if !ok || !created.Equal(time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("Created = %v, %v", created, ok)
	}
}

// Output uses canonical prefixes declared once on the root, keeps the
// dcterms prefix the xsi:type value refers to, and keeps unknown elements.
func TestCoreProperties_MarshalPrefixes(t *testing.T) {
	c := parse(t, wordCore)
	c.SetTime(docprops.Modified, time.Date(2026, 11, 14, 8, 0, 0, 0, time.FixedZone("IST", 5*3600+1800)))
	c.SetText(docprops.Category, "Report")
	out := marshal(t, c)
	for _, want := range []string{
		`<cp:coreProperties xmlns:cp="` + ns.CP + `" xmlns:dc="` + ns.DC + `" xmlns:dcterms="` + ns.DCTerms + `"`,
		`xmlns:xsi="` + ns.XSI + `" xmlns:ns1="urn:example:ext">`,
		`<dc:title>Gap assessment &amp; DPIA</dc:title>`,
		`<dcterms:modified xsi:type="dcterms:W3CDTF">2026-11-14T02:30:00Z</dcterms:modified>`,
		`<ns1:reviewed ns1:by="QA">yes</ns1:reviewed><cp:category>Report</cp:category></cp:coreProperties>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %s\n%s", want, out)
		}
	}
	if strings.Count(out, "xmlns:dc=") != 1 {
		t.Errorf("namespaces declared more than once:\n%s", out)
	}
}

func TestCoreProperties_SetTimeAddsType(t *testing.T) {
	c := parse(t, `<cp:coreProperties xmlns:cp="`+ns.CP+`"/>`)
	c.SetTime(docprops.Created, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	c.SetTime(docprops.LastPrinted, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	out := marshal(t, c)
	if !strings.Contains(out, `<dcterms:created xsi:type="dcterms:W3CDTF">2026-01-02T03:04:05Z</dcterms:created>`) {
		t.Errorf("created: %s", out)
	}
	if !strings.Contains(out, `<cp:lastPrinted>2026-01-02T03:04:05Z</cp:lastPrinted>`) {
		t.Errorf("lastPrinted must not get xsi:type: %s", out)
	}
	// Setting twice keeps one xsi:type.
	c.SetTime(docprops.Created, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if out := marshal(t, c); strings.Count(out, "xsi:type") != 1 {
		t.Errorf("xsi:type duplicated: %s", out)
	}
}

func TestCoreProperties_Remove(t *testing.T) {
	c := parse(t, wordCore)
	c.Remove(docprops.Title)
	if _, ok := c.Text(docprops.Title); ok {
		t.Error("title still present")
	}
}

func TestParseW3CDTF(t *testing.T) {
	for in, want := range map[string]time.Time{
		"2026-10-01T09:30:00Z":          time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC),
		"2026-10-01T09:30:00.5Z":        time.Date(2026, 10, 1, 9, 30, 0, 5e8, time.UTC),
		"2026-10-01T15:00:00+05:30":     time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC),
		"2026-10-01T09:30Z":             time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC),
		"2026-10-01":                    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		"2026-10":                       time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		"2026":                          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"2026-10-01T09:30:00.123456Z":   time.Date(2026, 10, 1, 9, 30, 0, 123456000, time.UTC),
		"2026-10-01T09:30:00.000+00:00": time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC),
	} {
		got, ok := docprops.ParseW3CDTF(in)
		if !ok || !got.Equal(want) {
			t.Errorf("ParseW3CDTF(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "yesterday", "01/10/2026"} {
		if _, ok := docprops.ParseW3CDTF(bad); ok {
			t.Errorf("ParseW3CDTF(%q) accepted", bad)
		}
	}
}

func TestCoreProperties_RejectsOtherRoot(t *testing.T) {
	dec := xml.NewDecoder(strings.NewReader(`<w:settings xmlns:w="` + ns.W + `"/>`))
	tok, _ := dec.Token()
	c := new(docprops.CT_CoreProperties)
	if err := c.UnmarshalXML(dec, tok.(xml.StartElement)); err == nil {
		t.Error("accepted w:settings as core properties")
	}
}
