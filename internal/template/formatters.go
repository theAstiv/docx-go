package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// Formatter transforms a value in a tag's pipe: {{total | number 2}} calls
// the "number" formatter with the value of total and args [2]. Arguments
// are strings or float64s. The result is written as text, or passed to the
// next formatter.
type Formatter func(v any, args []any) (any, error)

// builtin is a built-in formatter with its argument check, which runs on
// every tag that uses it whether or not the tag is rendered, so a wrong
// argument is a template error, not a data error.
type builtin struct {
	fn Formatter
	// args returns a non-empty detail when the arguments are invalid.
	args func(args []any) string
}

// defaultName is the formatter that also accepts a missing value.
const defaultName = "default"

var builtins = map[string]builtin{
	"upper":     {fn: textFormatter(strings.ToUpper), args: argsNone("upper")},
	"lower":     {fn: textFormatter(strings.ToLower), args: argsNone("lower")},
	defaultName: {fn: formatDefault, args: argsDefault},
	"number":    {fn: formatNumber, args: argsNumber},
	"currency":  {fn: formatCurrency, args: argsCurrency},
	"date":      {fn: formatDate, args: argsDate},
}

// formatterSet resolves formatter names for one Execute: Options.Funcs
// first, then the built-ins.
type formatterSet struct {
	funcs map[string]Formatter
}

// lookup returns the formatter and, for a built-in, its argument check.
func (fs formatterSet) lookup(name string) (Formatter, func([]any) string, bool) {
	if f, ok := fs.funcs[name]; ok && f != nil {
		return f, nil, true
	}
	if b, ok := builtins[name]; ok {
		return b.fn, b.args, true
	}
	return nil, nil, false
}

// callFormatter calls f, turning a panic in a caller's formatter into an
// error so Execute stays atomic and reports it like any other failure.
func callFormatter(f Formatter, v any, args []any) (out any, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("panic: %v", r)
		}
	}()
	return f(v, args)
}

// errDefaultOverride is returned by Execute when Options.Funcs replaces
// "default", whose missing-key handling lives in the engine.
var errDefaultOverride = errors.New(`template: Options.Funcs cannot replace the built-in "default" formatter`)

// --- argument checks -------------------------------------------------------

func argsNone(name string) func([]any) string {
	return func(args []any) string {
		if len(args) != 0 {
			return name + " takes no arguments"
		}
		return ""
	}
}

func argsDefault(args []any) string {
	if len(args) != 1 {
		return `default takes one argument: default "text"`
	}
	return ""
}

// argsNumberTail checks "N ["sep" ["dec"]]" starting at args[0].
func argsNumberTail(name string, args []any) string {
	if len(args) < 1 || len(args) > 3 {
		return name + ` takes the number of decimals, then optional "thousands" and "decimal" separators`
	}
	n, ok := args[0].(float64)
	if !ok || n != math.Trunc(n) || n < 0 || n > 20 {
		return name + ": the number of decimals must be a whole number from 0 to 20"
	}
	for _, a := range args[1:] {
		if _, ok := a.(string); !ok {
			return name + ": separators must be quoted strings"
		}
	}
	return ""
}

func argsNumber(args []any) string { return argsNumberTail("number", args) }

func argsCurrency(args []any) string {
	if len(args) == 0 {
		return `currency takes a symbol, then the number of decimals: currency "€" 2`
	}
	if _, ok := args[0].(string); !ok {
		return "currency: the symbol must be a quoted string"
	}
	if len(args) == 1 {
		return "" // two decimals
	}
	return argsNumberTail("currency", args[1:])
}

func argsDate(args []any) string {
	if len(args) != 1 {
		return `date takes one argument, a Go layout: date "2 Jan 2006"`
	}
	if _, ok := args[0].(string); !ok {
		return "date: the layout must be a quoted string"
	}
	return ""
}

// --- formatters --------------------------------------------------------------

func textFormatter(f func(string) string) Formatter {
	return func(v any, _ []any) (any, error) {
		s, ok := format(v)
		if !ok {
			return nil, errors.New("a list or object has no text")
		}
		return f(s), nil
	}
}

// formatDefault replaces null and "" with its argument. A missing value is
// replaced by the engine before formatters run (see engine.value).
func formatDefault(v any, args []any) (any, error) {
	if v == nil {
		return args[0], nil
	}
	if s, ok := v.(string); ok && s == "" {
		return args[0], nil
	}
	return v, nil
}

func formatNumber(v any, args []any) (any, error) {
	neg, digits, err := roundDecimal(v, int(args[0].(float64)))
	if err != nil {
		return nil, err
	}
	s := groupDigits(digits, int(args[0].(float64)), sepArgs(args[1:]))
	if neg {
		s = "-" + s
	}
	return s, nil
}

func formatCurrency(v any, args []any) (any, error) {
	sym := args[0].(string)
	decimals := 2
	var seps []any
	if len(args) > 1 {
		decimals = int(args[1].(float64))
		seps = args[2:]
	}
	neg, digits, err := roundDecimal(v, decimals)
	if err != nil {
		return nil, err
	}
	s := sym + groupDigits(digits, decimals, sepArgs(seps))
	if neg {
		s = "-" + s
	}
	return s, nil
}

// separators are the thousands and decimal separators of number output.
type separators struct{ thousands, decimal string }

func sepArgs(args []any) separators {
	s := separators{thousands: ",", decimal: "."}
	if len(args) > 0 {
		s.thousands = args[0].(string)
	}
	if len(args) > 1 {
		s.decimal = args[1].(string)
	}
	return s
}

// roundDecimal rounds v to the given number of decimals, half away from
// zero, on its decimal value: a JSON number or numeric string is read
// exactly, and a float64 through its shortest decimal form, so 2.675 rounds
// to 2.68 (the float64 nearest 2.675 is just below it). It returns the sign
// and the digits of |v| * 10^decimals, without a leading zero beyond one;
// neg is false when the rounded value is zero.
func roundDecimal(v any, decimals int) (neg bool, digits string, err error) {
	s, err := decimalString(v)
	if err != nil {
		return false, "", err
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return false, "", fmt.Errorf("%q is not a number", s)
	}
	neg = r.Sign() < 0
	r.Abs(r)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	// q = floor((2·num·scale + den) / (2·den)) rounds half up on |v|·scale.
	num := new(big.Int).Mul(r.Num(), scale)
	num.Mul(num, big.NewInt(2))
	num.Add(num, r.Denom())
	den := new(big.Int).Mul(r.Denom(), big.NewInt(2))
	q := new(big.Int).Quo(num, den)
	if q.Sign() == 0 {
		neg = false
	}
	return neg, q.String(), nil
}

// decimalString returns v's decimal text.
func decimalString(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", errors.New("no value (pipe through default first)")
	case json.Number:
		return x.String(), nil
	case string:
		s := strings.TrimSpace(x)
		if _, err := strconv.ParseFloat(s, 64); err != nil || strings.ContainsAny(s, "xXnNiIpP_") {
			return "", fmt.Errorf("%q is not a number", x)
		}
		return s, nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "", fmt.Errorf("%v is not a finite number", x)
		}
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case float32:
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return "", fmt.Errorf("%v is not a finite number", x)
		}
		return strconv.FormatFloat(float64(x), 'g', -1, 32), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(x), nil
	}
	return "", fmt.Errorf("a %T is not a number", v)
}

// groupDigits formats the digits of |v|·10^decimals with separators.
func groupDigits(digits string, decimals int, sep separators) string {
	if len(digits) <= decimals {
		digits = strings.Repeat("0", decimals-len(digits)+1) + digits
	}
	intPart, frac := digits[:len(digits)-decimals], digits[len(digits)-decimals:]
	var sb strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			sb.WriteString(sep.thousands)
		}
		sb.WriteRune(c)
	}
	if decimals > 0 {
		sb.WriteString(sep.decimal)
		sb.WriteString(frac)
	}
	return sb.String()
}

// dateLayouts are the input forms date accepts.
var dateLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"}

func formatDate(v any, args []any) (any, error) {
	layout := args[0].(string)
	switch x := v.(type) {
	case time.Time:
		return x.Format(layout), nil
	case string:
		for _, in := range dateLayouts {
			if t, err := time.Parse(in, strings.TrimSpace(x)); err == nil {
				return t.Format(layout), nil
			}
		}
		return nil, fmt.Errorf("%q is not an RFC 3339 date or time (2006-01-02 or 2006-01-02T15:04:05Z07:00)", x)
	}
	return nil, fmt.Errorf("a %T is not a date; pass an RFC 3339 string", v)
}
