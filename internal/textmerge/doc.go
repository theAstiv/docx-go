// Package textmerge finds and rewrites text that Word has split across runs.
//
// Word starts a new run whenever formatting, spell-check state or revision
// ids change, so text the author typed in one go often arrives in several
// runs. Flatten turns a paragraph's inline content into one string plus the
// pieces it came from; Collapse moves a byte range of that string into a
// single w:t so callers can edit it in place. The template engine uses it to
// merge split {{tags}}, and docx.Replace uses it to find text across runs.
//
// The package works on wml trees only. It sits beside the oxml layer and must
// not import parts, opc, docx or internal/template (enforced by depguard).
package textmerge
