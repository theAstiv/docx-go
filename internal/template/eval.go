package template

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// scope is the data visible at one point of the template: the root data and,
// inside {{#each}}, the current item.
type scope struct {
	root map[string]any
	// item is the current {{#each}} item; nil outside loops.
	item any
	// itemPath names the item in messages, e.g. "threats[3]".
	itemPath string
	inLoop   bool
}

// lookupResult is the outcome of resolving a path.
type lookupResult struct {
	value any
	found bool
	// detail explains a failed lookup, e.g. `"name" not found in client`.
	detail string
}

// lookup resolves a tag path. Paths starting with "." are relative to the
// current {{#each}} item; all others start at the root data.
func (s *scope) lookup(path string) lookupResult {
	var cur any
	var where string
	rest := path
	if strings.HasPrefix(path, ".") {
		if !s.inLoop {
			return lookupResult{detail: path + " is only valid inside {{#each}}"}
		}
		cur, where = s.item, s.itemPath
		rest = strings.TrimPrefix(path, ".")
		if rest == "" {
			return lookupResult{value: cur, found: true}
		}
	} else {
		cur, where = s.root, "the data"
	}
	for _, seg := range strings.Split(rest, ".") {
		next, ok, isMap := field(cur, seg)
		if !isMap {
			return lookupResult{detail: fmt.Sprintf("%s is not an object, cannot look up %q", where, seg)}
		}
		if !ok {
			return lookupResult{detail: fmt.Sprintf("%q not found in %s", seg, where)}
		}
		cur = next
		if where == "the data" {
			where = seg
		} else {
			where += "." + seg
		}
	}
	return lookupResult{value: cur, found: true}
}

// field returns m[key] for the map types JSON decoding and hand-built data
// produce. isMap is false when v is not a map at all.
func field(v any, key string) (val any, ok, isMap bool) {
	switch m := v.(type) {
	case map[string]any:
		val, ok = m[key]
		return val, ok, true
	case map[string]string:
		s, ok := m[key]
		return s, ok, true
	}
	return nil, false, false
}

// format renders a scalar value as text. ok is false for values that have
// no text form (maps, lists, unknown types).
func format(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", true
	case string:
		return x, true
	case bool:
		return strconv.FormatBool(x), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	case int:
		return strconv.Itoa(x), true
	case int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(x), true
	case json.Number:
		return x.String(), true
	case fmt.Stringer:
		return x.String(), true
	}
	return "", false
}
