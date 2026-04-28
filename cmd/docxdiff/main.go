// Command docxdiff shows a structural diff between two .docx files.
//
// Both files are canonicalised via internal/canon before diffing so the
// output agrees exactly with what the round-trip test oracle considers equal.
//
// Usage:
//
//	docxdiff a.docx b.docx
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/theAstiv/docx-go/internal/canon"
	"github.com/theAstiv/docx-go/internal/opc"
)

// relLabel formats a relationship set source path for display.
// "" (package-level) is shown as "(package)".
func relLabel(partPath string) string {
	if partPath == "" {
		return "(package)"
	}
	return partPath
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: docxdiff a.docx b.docx")
		os.Exit(1)
	}

	aLines, err := packageLines(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "docxdiff: %v\n", err)
		os.Exit(1)
	}
	bLines, err := packageLines(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "docxdiff: %v\n", err)
		os.Exit(1)
	}

	diff := unifiedDiff(aLines, bLines, os.Args[1], os.Args[2])
	if diff == "" {
		fmt.Println("(no differences)")
		return
	}
	fmt.Print(diff)
	os.Exit(1)
}

// packageLines opens a .docx, canonicalises every XML part, and returns a
// stable text projection — one line per part entry — suitable for diffing.
func packageLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	var lines []string

	// Content-type entries.
	for _, d := range pkg.ContentTypes.Defaults {
		lines = append(lines, fmt.Sprintf("[CT:default] .%s → %s", d.Extension, d.ContentType))
	}
	for _, o := range pkg.ContentTypes.Overrides {
		lines = append(lines, fmt.Sprintf("[CT:override] %s → %s", o.PartName, o.ContentType))
	}

	// All relationship sets (package-level and per-part).
	relsParts := pkg.RelsParts()
	sort.Strings(relsParts)
	for _, partPath := range relsParts {
		label := relLabel(partPath)
		for _, rel := range pkg.Rels(partPath).All() {
			lines = append(lines, fmt.Sprintf("[rels:%s] %s %s → %s",
				label, rel.ID, shortType(rel.Type), rel.Target))
		}
	}

	// XML parts — canonicalised.
	for _, partPath := range pkg.PartPaths() {
		raw := pkg.Part(partPath)
		if isXMLPart(partPath) {
			can, err := canon.XML(raw)
			if err != nil {
				// Not valid XML — emit raw for diffing.
				can = raw
			}
			lines = append(lines, fmt.Sprintf("=== %s ===\n%s", partPath, can))
		} else {
			lines = append(lines, fmt.Sprintf("[binary] %s (%d bytes)", partPath, len(raw)))
		}
	}

	sort.Strings(lines)
	return lines, nil
}

func isXMLPart(path string) bool {
	return strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".rels")
}

func shortType(relType string) string {
	idx := strings.LastIndexByte(relType, '/')
	if idx < 0 {
		return relType
	}
	return relType[idx+1:]
}

// unifiedDiff produces a minimal unified diff between two string slices.
func unifiedDiff(a, b []string, nameA, nameB string) string {
	if slicesEqual(a, b) {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("--- " + nameA + "\n")
	sb.WriteString("+++ " + nameB + "\n")

	// Simple line-by-line diff (LCS not needed for this tool's purpose).
	aSet := make(map[string]bool, len(a))
	bSet := make(map[string]bool, len(b))
	for _, l := range a {
		aSet[l] = true
	}
	for _, l := range b {
		bSet[l] = true
	}
	for _, l := range a {
		if !bSet[l] {
			sb.WriteString("- " + l + "\n")
		}
	}
	for _, l := range b {
		if !aSet[l] {
			sb.WriteString("+ " + l + "\n")
		}
	}
	return sb.String()
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
