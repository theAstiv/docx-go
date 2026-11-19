package template

import (
	"errors"
	"testing"
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
