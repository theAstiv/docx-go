package template

import (
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// ImageValue is an image to place at an {{image path}} tag. Sizes are EMU;
// zero keeps the image's aspect ratio (both zero: its pixel size at 96 DPI).
// docx/template maps its public Image to this through Options.Adapt.
type ImageValue struct {
	Data          []byte
	Width, Height int64
	AltText       string
	Title         string
}

// Hooks are the package operations the engine cannot do itself: it sees
// only element trees (CLAUDE.md, templating D-calls). Nil hooks make the
// matching tags fail with ErrUnsupportedLocation.
type Hooks struct {
	// AddImage stores img in the package, with its relationship on the part
	// at path part, and returns the inline picture to place, using docPrID
	// as its wp:docPr id.
	AddImage func(part string, img ImageValue, docPrID int64) (*wml.CT_Drawing, error)
}

// adapt maps a looked-up value through Options.Adapt.
func (en *engine) adapt(v any) any {
	if en.opts.Adapt == nil {
		return v
	}
	return en.opts.Adapt(v)
}

// image resolves an {{image path}} tag to the picture to place. ok is false
// when nothing is placed: a reported problem, a missing key in Lenient mode,
// or the structure check.
func (en *engine) image(t tag, sc *scope, loc Location) (*wml.CT_Drawing, bool) {
	if en.inRaw {
		en.fail(loc, t, ErrUnsupportedLocation, "image tags are not supported "+rawLocation)
		return nil, false
	}
	if en.check {
		return nil, false
	}
	r := sc.lookup(t.path)
	if !r.found {
		en.missing(loc, t, r.detail)
		return nil, false
	}
	var img ImageValue
	switch v := en.adapt(r.value).(type) {
	case ImageValue:
		img = v
	case *ImageValue:
		if v == nil {
			en.fail(loc, t, ErrUnsupportedValue, "the image is nil")
			return nil, false
		}
		img = *v
	default:
		en.fail(loc, t, ErrUnsupportedValue, fmt.Sprintf("{{image}} needs a template.Image value, got %T", r.value))
		return nil, false
	}
	if img.AltText == "" {
		en.fail(loc, t, ErrUnsupportedValue, "an image needs AltText, which screen readers read out")
		return nil, false
	}
	if en.opts.Hooks.AddImage == nil {
		en.fail(loc, t, ErrUnsupportedLocation, "images can only be added to a document")
		return nil, false
	}
	d, err := en.opts.Hooks.AddImage(loc.Part, img, en.nextDocPr())
	if err != nil {
		en.fail(loc, t, ErrUnsupportedValue, err.Error())
		return nil, false
	}
	return d, true
}
