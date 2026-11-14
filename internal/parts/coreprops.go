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
// package relationships, or "" when there is none.
func (p *Package) corePropsPath() string {
	rels := p.pkg.Rels("").ByType(opc.RelTypeCoreProps)
	if len(rels) == 0 {
		return ""
	}
	return resolveRelPath("", rels[0].Target)
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
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parts: core properties: read root token: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			ct := new(docprops.CT_CoreProperties)
			if err := ct.UnmarshalXML(dec, start); err != nil {
				return nil, fmt.Errorf("parts: core properties: %w", err)
			}
			return ct, nil
		}
	}
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
// with its content-type override and package relationship. A part that
// exists but cannot be parsed is replaced by an empty one.
func (p *Package) EditCoreProps() *docprops.CT_CoreProperties {
	st := p.loadCoreProps()
	if st == nil {
		path := p.corePropsPath()
		if path == "" {
			path = PathCoreProps
			p.pkg.Rels("").Add(opc.RelTypeCoreProps, PathCoreProps, "")
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
