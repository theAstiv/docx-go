package template

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Without hooks (the engine on its own), an image tag cannot be filled.
func TestImage_NoHooks(t *testing.T) {
	_, _, err := executeBody(t, `<w:p><w:r><w:t>{{image p}}</w:t></w:r></w:p>`,
		map[string]any{"p": ImageValue{Data: []byte("x"), AltText: "x"}})
	if !errors.Is(err, ErrUnsupportedLocation) {
		t.Errorf("err = %v, want ErrUnsupportedLocation", err)
	}
}

// Image tags in content controls are not supported, whatever the data.
func TestImage_InContentControl(t *testing.T) {
	inner := `<w:sdt><w:sdtContent><w:p><w:r><w:t>{{image p}}</w:t></w:r></w:p></w:sdtContent></w:sdt>`
	for _, data := range []map[string]any{{}, {"p": ImageValue{Data: []byte("x"), AltText: "x"}}} {
		_, _, err := executeBody(t, inner, data)
		if !errors.Is(err, ErrUnsupportedLocation) {
			t.Errorf("data %v: err = %v, want ErrUnsupportedLocation", data, err)
		}
	}
}

// A hook error that does not wrap ErrUnsupportedValue is an internal
// failure: it aborts Execute instead of becoming an issue about the data.
func TestImage_HookErrors(t *testing.T) {
	var body wml.CT_Body
	src := `<w:body ` + testNS + `><w:p><w:r><w:t>{{image p}}</w:t></w:r></w:p></w:body>`
	for _, tt := range []struct {
		hookErr error
		issue   bool
	}{
		{fmt.Errorf("%w: bad png", ErrUnsupportedValue), true},
		{errors.New("disk on fire"), false},
	} {
		if err := xml.Unmarshal([]byte(src), &body); err != nil {
			t.Fatal(err)
		}
		hooks := Hooks{AddImage: func(string, ImageValue, int64) (*wml.CT_Drawing, error) { return nil, tt.hookErr }}
		_, err := Execute([]Story{{Part: "word/document.xml", Blocks: &body.Children}},
			map[string]any{"p": ImageValue{Data: []byte("x"), AltText: "x"}}, Options{Hooks: hooks})
		var te *Error
		if got := errors.As(err, &te); got != tt.issue {
			t.Errorf("%v: err = %v, *Error = %v, want %v", tt.hookErr, err, got, tt.issue)
		}
		if !errors.Is(err, tt.hookErr) && !strings.Contains(err.Error(), tt.hookErr.Error()) {
			t.Errorf("%v: err = %v does not carry the hook's error", tt.hookErr, err)
		}
	}
}
