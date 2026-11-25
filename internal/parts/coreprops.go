package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/opc"
	"github.com/theAstiv/docx-go/internal/oxml/docprops"
)

const (
	// ContentTypeCoreProps is the MIME type for docProps/core.xml.
	ContentTypeCoreProps = "application/vnd.openxmlformats-package.core-properties+xml"
	// PathCoreProps is the canonical OPC path for the core properties part.
	PathCoreProps = "docProps/core.xml"
)

// corePropsState is the parsed core properties part. Unlike the other typed
// parts it is read lazily and written back only after a change, so a
// document whose properties are never touched keeps docProps/core.xml
// byte-identical, and templating can keep filling the part's bytes directly.
type corePropsState struct {
	path  string
	ct    *docprops.CT_CoreProperties
	dirty bool
}

// corePropsPath returns the path of the core properties part named by the
// package relationships: the stored name when the part exists (OPC names
// compare ignoring case), else the resolved name. It returns "" when there
// is no such relationship or its target names no part inside the package.
func (p *Package) corePropsPath() string {
	rels := p.pkg.Rels("").ByType(opc.RelTypeCoreProps)
	if len(rels) == 0 {
		return ""
	}
	name, _, ok := p.locate("", rels[0])
	if !ok {
		return ""
	}
	return name
}

// load parses the core properties part on first use. It returns nil when
// the package has no such part or it cannot be parsed.
func (p *Package) loadCoreProps() *corePropsState {
	if p.core != nil {
		return p.core
	}
	path := p.corePropsPath()
	if path == "" {
		return nil
	}
	data := p.pkg.Part(path)
	if data == nil {
		return nil
	}
	ct, err := parseCoreProps(data)
	if err != nil {
		return nil
	}
	p.core = &corePropsState{path: path, ct: ct, dirty: false}
	return p.core
}

func parseCoreProps(data []byte) (*docprops.CT_CoreProperties, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	start, err := opc.RootElement(dec)
	if err != nil {
		return nil, fmt.Errorf("parts: core properties: %w", err)
	}
	ct := new(docprops.CT_CoreProperties)
	if err := ct.UnmarshalXML(dec, start); err != nil {
		return nil, fmt.Errorf("parts: core properties: %w", err)
	}
	return ct, nil
}

// CoreProps returns the core document properties for reading, or nil when
// the package has none (or they cannot be parsed). Callers must not modify
// the result; use EditCoreProps.
func (p *Package) CoreProps() *docprops.CT_CoreProperties {
	if st := p.loadCoreProps(); st != nil {
		return st.ct
	}
	return nil
}

// EditCoreProps returns the core document properties for modification and
// marks them to be written on Save. A package without the part gets one,
// with its content-type override and package relationship, under the first
// of docProps/core.xml, docProps/core2.xml, … no part uses; a relationship
// whose target names no part inside the package is replaced. A part that
// exists but cannot be parsed is replaced by an empty one.
func (p *Package) EditCoreProps() *docprops.CT_CoreProperties {
	st := p.loadCoreProps()
	if st == nil {
		path := p.corePropsPath()
		if path == "" {
			rels := p.pkg.Rels("")
			for _, r := range rels.ByType(opc.RelTypeCoreProps) {
				rels.Remove(r.ID)
			}
			path = PathCoreProps
			for n := 2; p.taken(path); n++ {
				path = fmt.Sprintf("docProps/core%d.xml", n)
			}
			rels.Add(opc.RelTypeCoreProps, path, "")
		}
		p.pkg.ContentTypes.SetOverride("/"+path, ContentTypeCoreProps)
		st = &corePropsState{path: path, ct: new(docprops.CT_CoreProperties), dirty: false}
		p.core = st
	}
	st.dirty = true
	return st.ct
}

// flushCoreProps writes changed core properties into the package bytes.
func (p *Package) flushCoreProps() error {
	if p.core == nil || !p.core.dirty {
		return nil
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := p.core.ct.MarshalXML(enc, xml.StartElement{}); err != nil {
		return fmt.Errorf("parts: core properties: marshal: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return fmt.Errorf("parts: core properties: flush: %w", err)
	}
	p.pkg.SetPart(p.core.path, buf.Bytes())
	p.core.dirty = false
	return nil
}
