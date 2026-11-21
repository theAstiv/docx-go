package opc

import (
	"archive/zip"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
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
func readZip(r io.ReaderAt, size int64, lim limits) (map[string][]byte, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("opc/zip: open: %w", err)
	}
	if len(zr.File) > lim.entries {
		return nil, fmt.Errorf("%w: %d ZIP entries (max %d)", ErrLimitExceeded, len(zr.File), lim.entries)
	}
	// Check the declared sizes before reading anything. archive/zip refuses
	// to read past an entry's declared size, so these bound the reads too;
	// the LimitReader below is a second guard.
	var total int64
	for _, f := range zr.File {
		if f.UncompressedSize64 > math.MaxInt64 {
			return nil, fmt.Errorf("%w: %s declares %d bytes uncompressed", ErrLimitExceeded, f.Name, f.UncompressedSize64)
		}
		size := int64(f.UncompressedSize64)
		if size > lim.entry {
			return nil, fmt.Errorf("%w: %s is %d bytes uncompressed (max %d)",
				ErrLimitExceeded, f.Name, size, lim.entry)
		}
		// total <= lim.total and size <= lim.entry here, so the sum cannot
		// overflow.
		total += size
		if total > lim.total {
			return nil, fmt.Errorf("%w: entries total more than %d bytes uncompressed", ErrLimitExceeded, lim.total)
		}
	}
	parts := make(map[string][]byte, len(zr.File))
	folded := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		// A folder entry ("word/") is not a part; some ZIP tools add them.
		if strings.HasSuffix(f.Name, "/") && f.UncompressedSize64 == 0 {
			continue
		}
		if err := checkPartName(f.Name); err != nil {
			return nil, err
		}
		// Part names compare ignoring ASCII case; two entries for one part
		// would leave which one wins to map order.
		key := strings.ToLower(f.Name)
		if prev, dup := folded[key]; dup {
			return nil, fmt.Errorf("%w: %q and %q name the same part", ErrPartName, prev, f.Name)
		}
		folded[key] = f.Name
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opc/zip: open %s: %w", f.Name, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, lim.entry+1))
		if readErr == nil && int64(len(data)) > lim.entry {
			readErr = fmt.Errorf("%w: %s is larger than %d bytes uncompressed", ErrLimitExceeded, f.Name, lim.entry)
		}
		closeErr := rc.Close()
		if readErr != nil {
			return nil, fmt.Errorf("opc/zip: read %s: %w", f.Name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("opc/zip: close %s: %w", f.Name, closeErr)
		}
		// Every part: see checkXMLDepth for why no part may be skipped.
		if err := checkXMLDepth(f.Name, data, lim.depth); err != nil {
			return nil, err
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
			Name:     e.Name,
			Method:   zip.Deflate,
			Modified: epoch,
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
