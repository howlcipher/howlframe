package vm

import (
	"math"
	"testing"
)

// The VM writes and reads JSON text itself, as Go 1.25's encoding/json
// does, so a program's output does not change with the toolchain that
// built the VM (bugs.md #63). These are Go 1.25's texts.
func TestJSONTextIsGo125(t *testing.T) {
	for _, tc := range []struct {
		v    any
		want string
	}{
		{map[string]any{"b": int64(1), "a": map[string]any{"z": int64(1), "y": "<x>&"}}, `{"a":{"y":"\u003cx\u003e\u0026","z":1},"b":1}`},
		{"a\xffb\xe2\x82c", `"a\ufffdb\ufffd\ufffdc"`},
		{"\u2028\u2029", `"\u2028\u2029"`},
		{"\x01\b\f\n\r\t\"\\", `"\u0001\b\f\n\r\t\"\\"`},
		{[]any{1.5, math.Copysign(0, -1), 0.000001, 0.0000001, 1e21, 2.0, 123456789.125}, `[1.5,-0,0.000001,1e-7,1e+21,2,123456789.125]`},
		{[]any{nil, true, false, int64(-7), []any{}, map[string]any{}}, `[null,true,false,-7,[],{}]`},
	} {
		got, err := encodeJSONText(tc.v)
		if err != nil || string(got) != tc.want {
			t.Errorf("encode %#v = %s, %v; want %s", tc.v, got, err, tc.want)
		}
	}
	if _, err := encodeJSONText(math.NaN()); err == nil || err.Error() != "json: unsupported value: NaN" {
		t.Errorf("encode NaN: %v", err)
	}
	for text, want := range map[string]string{
		"":         "EOF",
		"   ":      "EOF",
		"{":        "unexpected EOF",
		"[1,]":     "invalid character ']' looking for beginning of value",
		`{"a" 1}`:  "invalid character '1' after object key",
		`{"a":1,}`: "invalid character '}' looking for beginning of object key string",
		"[1 2]":    "invalid character '2' after array element",
		"nulx":     "invalid character 'x' in literal null (expecting 'l')",
		"01":       "json: cannot unmarshal number into Go value of type struct {}",
		"1 [2]":    "json: cannot unmarshal array into Go value of type struct {}",
		"{}{}":     "multiple JSON values",
		"{} x":     "invalid character 'x' looking for beginning of value",
		"1.x":      "invalid character 'x' after decimal point in numeric literal",
		"1e+x":     "invalid character 'x' in exponent of numeric literal",
		"-x":       "invalid character 'x' in numeric literal",
		`"\q"`:     "invalid character 'q' in string escape code",
		`"\u12g4"`: `invalid character 'g' in \u hexadecimal character escape`,
		"\"a\tb\"": `invalid character '\t' in string literal`,
		"\x01":     `invalid character '\x01' looking for beginning of value`,
		"é":        "invalid character 'Ã' looking for beginning of value",
		"'":        `invalid character '\'' looking for beginning of value`,
		"1e400":    `strconv.ParseFloat: parsing "1e400": value out of range`,
	} {
		if _, err := decodeJSONText([]byte(text)); err == nil || err.Error() != want {
			t.Errorf("decode %q: %v, want %s", text, err, want)
		}
	}
	v, err := decodeJSONText([]byte(`{"a":1,"a":3,"b":[9223372036854775808,-0,-0.0],"s":"\ud83d\ude00\ud800x` + "\xff" + `y"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	b := m["b"].([]any)
	if m["a"] != int64(3) || b[0] != 9.223372036854776e18 || b[1] != int64(0) || !math.Signbit(b[2].(float64)) || m["s"] != "\U0001F600\uFFFDx\uFFFDy" {
		t.Errorf("decoded %#v", v)
	}
}
