package template

import "testing"

// A paragraph is a block tag only if its whole visible text is the tag,
// counting text in tracked insertions and content controls.
func TestMarker_RawInlineText(t *testing.T) {
	cases := []struct {
		name, in string
		want     bool
	}{
		{"plain", `<w:r><w:t>{{#if x}}</w:t></w:r>`, true},
		{"typed with track changes on",
			`<w:ins w:id="1" w:author="a"><w:r><w:t>{{#if x}}</w:t></w:r></w:ins>`, true},
		{"tag plus inserted text",
			`<w:r><w:t>{{/if}}</w:t></w:r><w:ins w:id="1" w:author="a"><w:r><w:t>extra</w:t></w:r></w:ins>`, false},
		{"tag plus deleted text",
			`<w:r><w:t>{{/if}}</w:t></w:r><w:del w:id="1" w:author="a"><w:r><w:delText>gone</w:delText></w:r></w:del>`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, got := marker(parseP(t, c.in)); got != c.want {
				t.Errorf("marker = %v, want %v", got, c.want)
			}
		})
	}
}
