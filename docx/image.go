package docx

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // register the GIF decoder for image.DecodeConfig
	_ "image/jpeg" // register the JPEG decoder for image.DecodeConfig
	_ "image/png"  // register the PNG decoder for image.DecodeConfig
	"io"
	"iter"
	"math"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// maxImageBytes bounds how much AddImage reads from its reader.
const maxImageBytes = 64 << 20

// emuPerPixel converts pixels to EMU at the default 96 DPI.
const emuPerPixel = 914400 / 96

// Errors returned by Run.AddImage. Test with errors.Is.
var (
	// ErrUnsupportedImage reports image data that is not PNG, JPEG or GIF.
	ErrUnsupportedImage = errors.New("docx: unsupported image format")
	// ErrInvalidImageSize reports a negative width or height.
	ErrInvalidImageSize = errors.New("docx: invalid image size")
	// ErrImageTooLarge reports image data above the 64 MiB limit.
	ErrImageTooLarge = errors.New("docx: image too large")
)

// ImageOptions configures Run.AddImage.
type ImageOptions struct {
	// Width and Height set the displayed size. If both are zero the image is
	// shown at its pixel size at 96 DPI. If only one is set the other follows
	// from the image's aspect ratio.
	Width, Height EMU
	// AltText describes the image for screen readers. It is written even when
	// empty (an empty description marks the image decorative); Validate flags
	// an empty value.
	AltText string
	// Title is an optional tooltip-style title.
	Title string
}

// Image is a proxy for an inline picture (w:drawing/wp:inline).
type Image struct {
	d  *wml.CT_Drawing
	st *story
}

// AddImage reads an image from r, stores it in the package and appends an
// inline picture of it to the run.
//
// PNG, JPEG and GIF are supported. SVG is not yet: Word needs a PNG fallback
// next to it, so SVG input returns ErrUnsupportedImage. Identical image bytes
// are stored once however many times they are added, and in headers and
// footers the image relationship lives on the header or footer part.
func (r *Run) AddImage(rd io.Reader, opts ImageOptions) (*Image, error) {
	if opts.Width < 0 || opts.Height < 0 {
		return nil, fmt.Errorf("%w: %d x %d EMU", ErrInvalidImageSize, opts.Width, opts.Height)
	}
	data, err := io.ReadAll(io.LimitReader(rd, maxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("docx: read image: %w", err)
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrImageTooLarge, maxImageBytes)
	}
	if looksLikeSVG(data) {
		return nil, fmt.Errorf("%w: SVG is not supported yet; convert it to PNG first", ErrUnsupportedImage)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupportedImage, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("%w: %d x %d pixels", ErrUnsupportedImage, cfg.Width, cfg.Height)
	}

	cx, cy, err := displaySize(int64(cfg.Width), int64(cfg.Height), opts)
	if err != nil {
		return nil, err
	}

	relID, fileName, err := r.st.pkg.AddImage(r.st.src, data, format)
	if err != nil {
		return nil, fmt.Errorf("docx: add image: %w", err)
	}

	d := wml.NewInlinePicture(wml.InlinePictureSpec{
		RID:      relID,
		FileName: fileName,
		CX:       cx,
		CY:       cy,
		DocPrID:  r.st.nextDocPrID(),
		AltText:  opts.AltText,
		Title:    opts.Title,
	})
	r.r.Children = append(r.r.Children, d)
	return &Image{d: d, st: r.st}, nil
}

// displaySize resolves the displayed size in EMU from the pixel size and the
// options.
func displaySize(pxW, pxH int64, opts ImageOptions) (cx, cy int64, err error) {
	w, h := int64(opts.Width), int64(opts.Height)
	switch {
	case w == 0 && h == 0:
		w, h = pxW*emuPerPixel, pxH*emuPerPixel
	case h == 0:
		h = scaleRound(w, pxH, pxW)
	case w == 0:
		w = scaleRound(h, pxW, pxH)
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("%w: %d x %d EMU", ErrInvalidImageSize, w, h)
	}
	return w, h, nil
}

// scaleRound returns v*num/den rounded to the nearest integer, saturating
// instead of overflowing.
func scaleRound(v, num, den int64) int64 {
	if den == 0 {
		return 0
	}
	f := math.Round(float64(v) * float64(num) / float64(den))
	if f > math.MaxInt64/2 {
		return math.MaxInt64 / 2
	}
	return int64(f)
}

// looksLikeSVG sniffs for an SVG document, which image.DecodeConfig cannot
// identify and which would otherwise produce a confusing error.
func looksLikeSVG(data []byte) bool {
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	head = bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
	head = bytes.TrimLeft(head, " \t\r\n")
	return bytes.HasPrefix(head, []byte("<svg")) ||
		(bytes.HasPrefix(head, []byte("<?xml")) && bytes.Contains(head, []byte("<svg")))
}

// Images iterates over the inline pictures in the run.
func (r *Run) Images() iter.Seq[*Image] {
	return func(yield func(*Image) bool) {
		for _, c := range r.r.Children {
			if d, ok := c.(*wml.CT_Drawing); ok && d.IsInline() {
				if !yield(&Image{d: d, st: r.st}) {
					return
				}
			}
		}
	}
}

// Size returns the displayed width and height.
func (i *Image) Size() (width, height EMU) {
	cx, cy, _ := i.d.ExtentEMU()
	return EMU(cx), EMU(cy)
}

// SetSize sets the displayed width and height. Both must be positive; other
// values are ignored.
func (i *Image) SetSize(width, height EMU) {
	if width <= 0 || height <= 0 {
		return
	}
	i.d.SetExtentEMU(int64(width), int64(height))
}

// AltText returns the alternative text.
func (i *Image) AltText() string { return i.d.AltText() }

// SetAltText sets the alternative text. An empty string marks the image
// decorative.
func (i *Image) SetAltText(s string) { i.d.SetAltText(s) }

// Title returns the title, or "".
func (i *Image) Title() string { return i.d.Title() }

// SetTitle sets the title. An empty string removes it.
func (i *Image) SetTitle(s string) { i.d.SetTitle(s) }

// Bytes returns the stored image file. It reports an error for a picture whose
// relationship is missing or external.
func (i *Image) Bytes() ([]byte, error) {
	rid := i.d.EmbedRID()
	data, ok := i.st.pkg.ImageData(i.st.src, rid)
	if !ok {
		return nil, fmt.Errorf("docx: image data for relationship %q not found", rid)
	}
	return append([]byte(nil), data...), nil
}
