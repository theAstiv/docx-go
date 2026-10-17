package parts

import "github.com/theAstiv/docx-go/internal/opc"

// targetModeExternal is the OPC TargetMode value for links that leave the package.
const targetModeExternal = "External"

// Path returns the OPC path of the header part (e.g. "word/header1.xml").
func (e *HeaderEntry) Path() string { return e.path }

// Path returns the OPC path of the footer part (e.g. "word/footer1.xml").
func (e *FooterEntry) Path() string { return e.path }

// AddExternalHyperlink returns the relationship ID that links the part at
// source to the external URL, creating the relationship when the part has no
// identical one yet. The URL is stored exactly as given.
func (p *Package) AddExternalHyperlink(source, url string) string {
	rels := p.pkg.Rels(source)
	for _, r := range rels.ByType(opc.RelTypeHyperlink) {
		if r.TargetMode == targetModeExternal && r.Target == url {
			return r.ID
		}
	}
	return rels.Add(opc.RelTypeHyperlink, url, targetModeExternal)
}

// HyperlinkTarget resolves a hyperlink relationship of the part at source.
// It reports false when the ID is unknown or is not a hyperlink relationship.
func (p *Package) HyperlinkTarget(source, relID string) (string, bool) {
	r, ok := p.pkg.Rels(source).ByID(relID)
	if !ok || r.Type != opc.RelTypeHyperlink {
		return "", false
	}
	return r.Target, true
}
