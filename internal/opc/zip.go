package opc

import (
	"archive/zip"
	"fmt"
	"io"
	"sort"
	"time"
)

// epoch is the deterministic mtime stamped on every ZIP entry we write.
var epoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// zipEntry holds the name and raw bytes of a single ZIP part.
type zipEntry struct {
	Name string
	Data []byte
}

// readZip reads all entries from r into a name→data map.
func readZip(r io.ReaderAt, size int64) (map[string][]byte, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("opc/zip: open: %w", err)
	}
	parts := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opc/zip: open %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("opc/zip: read %s: %w", f.Name, err)
		}
		parts[f.Name] = data
	}
	return parts, nil
}

// writeZip writes entries to w in deterministic (sorted) order with epoch
// mtimes and no extra ZIP fields or ZIP64 extensions.
func writeZip(w io.Writer, entries []zipEntry) error {
	// Sort for deterministic output.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	zw := zip.NewWriter(w)
	for _, e := range entries {
		fh := &zip.FileHeader{
			Name:               e.Name,
			Method:             zip.Deflate,
			Modified:           epoch,
		}
		// Store very small entries uncompressed.
		if len(e.Data) < 128 {
			fh.Method = zip.Store
		}
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			return fmt.Errorf("opc/zip: create %s: %w", e.Name, err)
		}
		if _, err := fw.Write(e.Data); err != nil {
			return fmt.Errorf("opc/zip: write %s: %w", e.Name, err)
		}
	}
	return zw.Close()
}

