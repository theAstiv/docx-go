package parts

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/opc"
	"github.com/theAstiv/docx-go/internal/oxml/ns"
)

// Typed parts are found through relationships, and nothing but the
// relationship type says what a part holds. A relationship of one type
// naming a part of another (a header relationship to the main document)
// had that part parsed as the wrong type and written back over itself on
// Save, the last writer winning (v0.5.0 review H2). Two guards close this:
// each typed part's root element must be the one its role calls for, and no
// part may hold two roles.

// checkRoot returns ErrMalformedPart unless start is the WordprocessingML
// element local, the root element of the part's role.
func checkRoot(start xml.StartElement, local string) error {
	if start.Name.Space != ns.W || start.Name.Local != local {
		return fmt.Errorf("%w: root element is {%s}%s, want w:%s", opc.ErrMalformedPart, start.Name.Space, start.Name.Local, local)
	}
	return nil
}

// roleClaims records which typed role each part holds, by part name
// compared ignoring case, as OPC compares part names.
type roleClaims map[string]string

// claim records that the part at name holds role. Several relationships
// may name one part in the same role (two header references to one header
// part); a second role is ErrMalformedPart.
func (c roleClaims) claim(name, role string) error {
	k := strings.ToLower(name)
	if held, ok := c[k]; ok && held != role {
		return fmt.Errorf("%w: %q is both the %s part and the %s part", opc.ErrMalformedPart, name, held, role)
	}
	c[k] = role
	return nil
}
