package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"unicode/utf8"
)

// The JSON text of encode_json, res_json and parse_json, written and read as
// Go 1.25's encoding/json does, but by the VM itself: newer Go releases
// build encoding/json on json/v2, which words syntax errors differently and
// writes invalid UTF-8 as a raw U+FFFD, so a program's output depended on
// the toolchain that built the VM (bugs.md #63). The WebAssembly backend's
// std/json.howl implements the same rules, and its differential tests check
// both against each other.

// encodeJSONText writes v (as normalizeForJSON leaves it) as JSON: object
// keys in sorted byte order, no spaces, strings HTML-safe with invalid
// UTF-8 as the escape for U+FFFD, integers exactly and floats in encoding/json's layout.
func encodeJSONText(v any) ([]byte, error) {
	var b []byte
	return appendJSON(b, v)
}

func appendJSON(b []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(b, "null"...), nil
	case bool:
		return strconv.AppendBool(b, x), nil
	case int64:
		return strconv.AppendInt(b, x, 10), nil
	case int:
		return strconv.AppendInt(b, int64(x), 10), nil
	case float64:
		return appendJSONFloat(b, x)
	case string:
		return appendJSONString(b, x), nil
	case []any:
		b = append(b, '[')
		for i, e := range x {
			if i > 0 {
				b = append(b, ',')
			}
			var err error
			if b, err = appendJSON(b, e); err != nil {
				return nil, err
			}
		}
		return append(b, ']'), nil
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b = append(b, '{')
		for i, k := range keys {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSONString(b, k)
			b = append(b, ':')
			var err error
			if b, err = appendJSON(b, x[k]); err != nil {
				return nil, err
			}
		}
		return append(b, '}'), nil
	}
	// Anything else (no HowlFrame value) as encoding/json writes it.
	enc, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, enc...), nil
}

// appendJSONFloat is encoding/json's floatEncoder for float64.
func appendJSONFloat(b []byte, f float64) ([]byte, error) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, fmt.Errorf("json: unsupported value: %s", strconv.FormatFloat(f, 'g', -1, 64))
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	b = strconv.AppendFloat(b, f, format, -1, 64)
	if format == 'e' {
		// e-07 becomes e-7.
		if n := len(b); n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return b, nil
}

const jsonHex = "0123456789abcdef"

// appendJSONString is encoding/json's string encoding with HTML escaping.
func appendJSONString(b []byte, s string) []byte {
	b = append(b, '"')
	start := 0
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
				i++
				continue
			}
			b = append(b, s[start:i]...)
			switch c {
			case '"', '\\':
				b = append(b, '\\', c)
			case '\b':
				b = append(b, '\\', 'b')
			case '\f':
				b = append(b, '\\', 'f')
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				b = append(b, '\\', 'u', '0', '0', jsonHex[c>>4], jsonHex[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', 'f', 'f', 'f', 'd')
			i += size
			start = i
			continue
		}
		if r == ' ' || r == ' ' {
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '2', '0', '2', jsonHex[r&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}

// decodeJSONText reads JSON text as json.Decoder with UseNumber does, then
// checks for a second value: integers that fit int64 are int64 and other
// numbers float64, the last of duplicate keys wins, invalid UTF-8 and
// unpaired surrogates in strings become U+FFFD, and the errors are Go
// 1.25's (a number out of range is reported after the second-value check).
func decodeJSONText(data []byte) (any, error) {
	d := &jsonReader{s: data}
	d.skipSpace()
	if d.i >= len(d.s) {
		return nil, errors.New("EOF")
	}
	v, err := d.value()
	if err != nil {
		return nil, err
	}
	d.skipSpace()
	if d.i < len(d.s) {
		kind := d.s[d.i]
		if _, err := d.value(); err != nil {
			return nil, err
		}
		switch kind {
		case '{', 'n':
			return nil, errors.New("multiple JSON values")
		case '[':
			return nil, errors.New("json: cannot unmarshal array into Go value of type struct {}")
		case '"':
			return nil, errors.New("json: cannot unmarshal string into Go value of type struct {}")
		case 't', 'f':
			return nil, errors.New("json: cannot unmarshal bool into Go value of type struct {}")
		default:
			return nil, errors.New("json: cannot unmarshal number into Go value of type struct {}")
		}
	}
	if d.pending != nil {
		return nil, d.pending
	}
	return v, nil
}

type jsonReader struct {
	s       []byte
	i       int
	pending error // the first number out of range
}

var errUnexpectedEOF = errors.New("unexpected EOF")

// quoteChar quotes a byte as encoding/json's scanner does in messages.
func quoteChar(c byte) string {
	if c == '\'' {
		return `'\''`
	}
	if c == '"' {
		return `'"'`
	}
	s := strconv.Quote(string(rune(c)))
	return "'" + s[1:len(s)-1] + "'"
}

func syntaxError(c byte, context string) error {
	return errors.New("invalid character " + quoteChar(c) + " " + context)
}

func (d *jsonReader) skipSpace() {
	for d.i < len(d.s) {
		switch d.s[d.i] {
		case ' ', '\t', '\n', '\r':
			d.i++
		default:
			return
		}
	}
}

// need is the byte at the position, or unexpected EOF.
func (d *jsonReader) need() (byte, error) {
	if d.i >= len(d.s) {
		return 0, errUnexpectedEOF
	}
	return d.s[d.i], nil
}

func (d *jsonReader) value() (any, error) {
	d.skipSpace()
	c, err := d.need()
	if err != nil {
		return nil, err
	}
	switch {
	case c == '{':
		return d.object()
	case c == '[':
		return d.array()
	case c == '"':
		return d.str()
	case c == '-' || c >= '0' && c <= '9':
		return d.number()
	case c == 't':
		return true, d.literal("true")
	case c == 'f':
		return false, d.literal("false")
	case c == 'n':
		return nil, d.literal("null")
	}
	return nil, syntaxError(c, "looking for beginning of value")
}

func (d *jsonReader) literal(word string) error {
	for k := 0; k < len(word); k++ {
		c, err := d.need()
		if err != nil {
			return err
		}
		if c != word[k] {
			return syntaxError(c, "in literal "+word+" (expecting "+quoteChar(word[k])+")")
		}
		d.i++
	}
	return nil
}

func (d *jsonReader) object() (any, error) {
	out := map[string]any{}
	d.i++
	d.skipSpace()
	if c, err := d.need(); err != nil {
		return nil, err
	} else if c == '}' {
		d.i++
		return out, nil
	}
	for {
		d.skipSpace()
		c, err := d.need()
		if err != nil {
			return nil, err
		}
		if c != '"' {
			return nil, syntaxError(c, "looking for beginning of object key string")
		}
		k, err := d.str()
		if err != nil {
			return nil, err
		}
		d.skipSpace()
		if c, err = d.need(); err != nil {
			return nil, err
		}
		if c != ':' {
			return nil, syntaxError(c, "after object key")
		}
		d.i++
		v, err := d.value()
		if err != nil {
			return nil, err
		}
		out[k.(string)] = v
		d.skipSpace()
		if c, err = d.need(); err != nil {
			return nil, err
		}
		switch c {
		case ',':
			d.i++
		case '}':
			d.i++
			return out, nil
		default:
			return nil, syntaxError(c, "after object key:value pair")
		}
	}
}

func (d *jsonReader) array() (any, error) {
	out := []any{}
	d.i++
	d.skipSpace()
	if c, err := d.need(); err != nil {
		return nil, err
	} else if c == ']' {
		d.i++
		return out, nil
	}
	for {
		v, err := d.value()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		d.skipSpace()
		c, err := d.need()
		if err != nil {
			return nil, err
		}
		switch c {
		case ',':
			d.i++
		case ']':
			d.i++
			return out, nil
		default:
			return nil, syntaxError(c, "after array element")
		}
	}
}

func (d *jsonReader) str() (any, error) {
	d.i++
	var out []byte
	for {
		c, err := d.need()
		if err != nil {
			return nil, err
		}
		switch {
		case c == '"':
			d.i++
			return string(out), nil
		case c == '\\':
			d.i++
			if out, err = d.unescape(out); err != nil {
				return nil, err
			}
		case c < 0x20:
			return nil, syntaxError(c, "in string literal")
		case c < utf8.RuneSelf:
			out = append(out, c)
			d.i++
		default:
			r, size := utf8.DecodeRune(d.s[d.i:])
			out = utf8.AppendRune(out, r) // U+FFFD for an invalid byte
			d.i += size
		}
	}
}

func (d *jsonReader) unescape(out []byte) ([]byte, error) {
	c, err := d.need()
	if err != nil {
		return nil, err
	}
	d.i++
	switch c {
	case '"', '\\', '/':
		return append(out, c), nil
	case 'b':
		return append(out, '\b'), nil
	case 'f':
		return append(out, '\f'), nil
	case 'n':
		return append(out, '\n'), nil
	case 'r':
		return append(out, '\r'), nil
	case 't':
		return append(out, '\t'), nil
	case 'u':
	default:
		return nil, syntaxError(c, "in string escape code")
	}
	r, err := d.hex4()
	if err != nil {
		return nil, err
	}
	if r >= 0xD800 && r <= 0xDBFF {
		// A high surrogate pairs only with a following low one.
		if d.i+6 <= len(d.s) && d.s[d.i] == '\\' && d.s[d.i+1] == 'u' {
			save := d.i
			d.i += 2
			lo, err := d.hex4()
			if err != nil {
				return nil, err
			}
			if lo >= 0xDC00 && lo <= 0xDFFF {
				return utf8.AppendRune(out, 0x10000+(r-0xD800)<<10+(lo-0xDC00)), nil
			}
			d.i = save
		}
		return utf8.AppendRune(out, utf8.RuneError), nil
	}
	if r >= 0xDC00 && r <= 0xDFFF {
		return utf8.AppendRune(out, utf8.RuneError), nil
	}
	return utf8.AppendRune(out, r), nil
}

func (d *jsonReader) hex4() (rune, error) {
	var r rune
	for k := 0; k < 4; k++ {
		c, err := d.need()
		if err != nil {
			return 0, err
		}
		switch {
		case c >= '0' && c <= '9':
			r = r*16 + rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r*16 + rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r*16 + rune(c-'A'+10)
		default:
			return 0, syntaxError(c, `in \u hexadecimal character escape`)
		}
		d.i++
	}
	return r, nil
}

func (d *jsonReader) digits() {
	for d.i < len(d.s) && d.s[d.i] >= '0' && d.s[d.i] <= '9' {
		d.i++
	}
}

// number: -?(0|[1-9][0-9]*)(.[0-9]+)?([eE][+-]?[0-9]+)?
func (d *jsonReader) number() (any, error) {
	start := d.i
	whole := true
	if d.s[d.i] == '-' {
		d.i++
	}
	c, err := d.need()
	if err != nil {
		return nil, err
	}
	switch {
	case c == '0':
		d.i++
	case c >= '1' && c <= '9':
		d.digits()
	default:
		return nil, syntaxError(c, "in numeric literal")
	}
	if d.i < len(d.s) && d.s[d.i] == '.' {
		whole = false
		d.i++
		if c, err = d.need(); err != nil {
			return nil, err
		}
		if c < '0' || c > '9' {
			return nil, syntaxError(c, "after decimal point in numeric literal")
		}
		d.digits()
	}
	if d.i < len(d.s) && (d.s[d.i] == 'e' || d.s[d.i] == 'E') {
		whole = false
		d.i++
		if c, err = d.need(); err != nil {
			return nil, err
		}
		if c == '+' || c == '-' {
			d.i++
		}
		if c, err = d.need(); err != nil {
			return nil, err
		}
		if c < '0' || c > '9' {
			return nil, syntaxError(c, "in exponent of numeric literal")
		}
		d.digits()
	}
	text := string(d.s[start:d.i])
	if whole {
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			return n, nil
		}
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		if d.pending == nil {
			d.pending = err
		}
		return 0.0, nil
	}
	return f, nil
}
