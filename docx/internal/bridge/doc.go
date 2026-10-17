// Package bridge connects docx/template to the internals of docx.Document
// without adding exported methods that return internal types.
//
// Package docx sets the function variables here in its init function;
// docx/template calls them. Being internal to docx/, only packages under
// docx/ can import it.
package bridge
