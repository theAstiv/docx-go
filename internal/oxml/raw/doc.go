// Package raw provides RawElement — a token-stream capture for unknown XML
// elements.  Containers store unrecognised children as []*RawElement so that
// an open-save cycle never silently drops content.
package raw
