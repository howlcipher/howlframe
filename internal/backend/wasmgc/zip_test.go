package wasmgc

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/binary"
	"math/rand"
	"os/exec"
	"strings"
	"testing"
)

// zipOf writes a ZIP with archive/zip: each file as (name, data, method).
func zipOf(t *testing.T, files ...zipFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: f.name, Method: f.method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type zipFile struct {
	name   string
	data   []byte
	method uint16
}

// rawZip writes a one-entry ZIP by hand, so a test can give it any
// method, flags, sizes, CRC and compressed bytes.
func rawZip(name string, method, flags uint16, crc, csize, usize uint32, data []byte) []byte {
	var b bytes.Buffer
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	le(uint32(0x04034b50))
	le(uint16(20))
	le(flags)
	le(method)
	le(uint32(0x00210000))
	le(crc)
	le(csize)
	le(usize)
	le(uint16(len(name)))
	le(uint16(0))
	b.WriteString(name)
	b.Write(data)
	cd := b.Len()
	le(uint32(0x02014b50))
	le(uint16(20))
	le(uint16(20))
	le(flags)
	le(method)
	le(uint32(0x00210000))
	le(crc)
	le(csize)
	le(usize)
	le(uint16(len(name)))
	le(uint16(0))
	le(uint16(0))
	le(uint16(0))
	le(uint16(0))
	le(uint32(0))
	le(uint32(0))
	b.WriteString(name)
	size := b.Len() - cd
	le(uint32(0x06054b50))
	le(uint16(0))
	le(uint16(0))
	le(uint16(1))
	le(uint16(1))
	le(uint32(size))
	le(uint32(cd))
	le(uint16(0))
	return b.Bytes()
}

func deflated(t *testing.T, data []byte, level int) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := flate.NewWriter(&b, level)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(data)
	w.Close()
	return b.Bytes()
}

func crcOf(data []byte) uint32 {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	fw, _ := w.CreateHeader(&zip.FileHeader{Name: "x", Method: zip.Store})
	fw.Write(data)
	w.Close()
	r, _ := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	return r.File[0].CRC32
}

// readZip is a program that prints what zip_read gives for an archive.
func readZip(archive []byte, limits string) string {
	return `(print (encode_json (zip_read "` + base64.StdEncoding.EncodeToString(archive) + `" ` + limits + `)))`
}

// zip_build and zip_read are browser host operations of the JavaScript
// backend with no VM form, so it is their reference; this backend has
// them in std/zip.howl, inflate included.
func TestZipLikeJavaScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	text := []byte(strings.Repeat("the quick brown fox jumps over the lazy dog. ", 400))
	rng := rand.New(rand.NewSource(7))
	noise := make([]byte, 3000)
	rng.Read(noise)
	mixed := append(append([]byte{}, text[:5000]...), noise...)
	good := deflated(t, text, flate.BestCompression)
	set := func(b []byte, at int, v ...byte) []byte {
		c := append([]byte{}, b...)
		copy(c[at:], v)
		return c
	}
	two := zipOf(t, zipFile{"a.txt", []byte("hello"), zip.Store}, zipFile{"b/c.txt", text, zip.Deflate})

	for name, body := range map[string]string{
		// zip_build
		"build":            `(print (zip_build (list (dict ("path" "a.txt") ("text" "hello é ✓")) (dict ("path" "img/b.bin") ("base64" "AAEC/w==")) (dict ("path" "empty") ("text" "")))))`,
		"build empty":      `(print (zip_build (list)))`,
		"build round trip": `(print (encode_json (zip_read (zip_build (list (dict ("path" "x/y.json") ("text" "{\"a\":1}")) (dict ("path" "A~b@c+d-e_f.g") ("base64" "/w==")))) 10 1000)))`,
		"build not a list": `(let (d (dict ("s" "x") ("n" 1) ("f" 1.5) ("b" true) ("d" (dict)))) (print (try_let (z (zip_build (map_get d "s"))) (catch e "string") z) (try_let (z (zip_build (map_get d "zz"))) (catch e "nil") z)))`,
		"build string":     `(let (d (dict ("s" "x") ("n" 1))) (print (zip_build (map_get d "s"))))`,
		"build nil":        `(let (d (dict ("s" "x") ("n" 1))) (print (zip_build (map_get d "zz"))))`,
		"build number":     `(let (d (dict ("s" "x") ("n" 1))) (print (zip_build (map_get d "n"))))`,
		"build dict":       `(let (d (dict ("s" "x") ("d" (dict)))) (print (zip_build (map_get d "d"))))`,
		"build bool":       `(let (d (dict ("s" "x") ("b" true))) (print (zip_build (map_get d "b"))))`,
		"entry not a dict": `(let (d (dict ("l" (list "x")) ("n" 1))) (print (zip_build (map_get d "l"))))`,
		"absolute path":    `(print (zip_build (list (dict ("path" "/a") ("text" "")))))`,
		"dot segment":      `(print (zip_build (list (dict ("path" "a/./b") ("text" "")))))`,
		"dot dot":          `(print (zip_build (list (dict ("path" "../b") ("text" "")))))`,
		"empty segment":    `(print (zip_build (list (dict ("path" "a//b") ("text" "")))))`,
		"trailing slash":   `(print (zip_build (list (dict ("path" "a/") ("text" "")))))`,
		"space in path":    `(print (zip_build (list (dict ("path" "a b") ("text" "")))))`,
		"accent in path":   `(print (zip_build (list (dict ("path" "é") ("text" "")))))`,
		"no path":          `(print (zip_build (list (dict ("text" "")))))`,
		"long path":        `(print (zip_build (list (dict ("path" "` + strings.Repeat("a", 256) + `") ("text" "")))))`,
		"longest path":     `(print (zip_build (list (dict ("path" "` + strings.Repeat("a", 255) + `") ("text" "")))))`,
		"three dots":       `(print (zip_build (list (dict ("path" "a/.../b") ("text" "")))))`,
		"case duplicate":   `(print (zip_build (list (dict ("path" "Read.me") ("text" "")) (dict ("path" "rEAD.ME") ("text" "")))))`,
		"both kinds":       `(print (zip_build (list (dict ("path" "a") ("text" "") ("base64" "")))))`,
		"neither kind":     `(print (zip_build (list (dict ("path" "a")))))`,
		"text not string":  `(print (zip_build (list (dict ("path" "a") ("text" 1)))))`,
		"bad base64":       `(print (zip_build (list (dict ("path" "a") ("base64" "abc")))))`,
		"caught":           `(print (try_let (z (zip_build (list (dict ("path" "/") ("text" ""))))) (catch e "caught") z))`,
		// zip_read
		"read stored and deflated": readZip(two, `10 1000000`),
		"read fixed huffman":       readZip(zipOf(t, zipFile{"s.txt", []byte("abcabcabcabc"), zip.Deflate}), `10 1000`),
		"read mixed":               readZip(zipOf(t, zipFile{"m.bin", mixed, zip.Deflate}), `10 100000`),
		"read stored blocks":       readZip(zipOf(t, zipFile{"r.bin", noise, zip.Deflate}), `10 100000`),
		"read level zero":          readZip(rawZip("z.txt", 8, 0, crcOf(text), uint32(len(deflated(t, text, 0))), uint32(len(text)), deflated(t, text, 0)), `10 100000`),
		"read empty file":          readZip(zipOf(t, zipFile{"e", nil, zip.Deflate}, zipFile{"f", nil, zip.Store}), `10 10`),
		"read directories":         readZip(zipOf(t, zipFile{"d/", nil, zip.Store}, zipFile{"d/x", []byte("x"), zip.Store}), `10 10`),
		"read float limits":        readZip(two, `2.0 100000.0`),
		"too many entries":         readZip(two, `1 1000000`),
		"too many bytes":           readZip(two, `10 100`),
		"limit zero":               readZip(two, `0 10`),
		"limit fraction":           readZip(two, `10 1.5`),
		"limit text":               `(let (d (dict ("s" "x") ("n" 1))) (print (zip_read "" (map_get d "s") 10)))`,
		"not base64":               `(print (zip_read "abc" 10 10))`,
		"not a zip":                `(print (zip_read "aGVsbG8gd29ybGQgdGhpcyBpcyBub3QgYSB6aXA=" 10 10))`,
		"empty data":               `(print (zip_read "" 10 10))`,
		"bad crc":                  readZip(rawZip("a", 0, 0, 1234, 3, 3, []byte("abc")), `10 10`),
		"inconsistent sizes":       readZip(rawZip("a", 0, 0, crcOf([]byte("abc")), 3, 2, []byte("abc")), `10 10`),
		"encrypted":                readZip(rawZip("a", 0, 1, 0, 3, 3, []byte("abc")), `10 10`),
		"method 12":                readZip(rawZip("a", 12, 0, 0, 3, 3, []byte("abc")), `10 10`),
		"zip64 sizes":              readZip(rawZip("a", 0, 0, 0, 0xFFFFFFFF, 3, []byte("abc")), `10 10`),
		"absolute name":            readZip(rawZip("/etc/x", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"dot dot name":             readZip(rawZip("a/../b", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"backslash name":           readZip(rawZip("a\\b", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"colon name":               readZip(rawZip("c:x", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"control name":             readZip(rawZip("a\tb\x01\"q\"", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"long bad name":            readZip(rawZip(strings.Repeat("é", 78)+"😀😀/../x", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"split surrogate name":     readZip(rawZip(strings.Repeat("a", 79)+"😀/..", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"not utf-8 name":           readZip(rawZip("a\xffb", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"bom name":                 readZip(rawZip("\xef\xbb\xbfname.txt", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"unicode name":             readZip(rawZip("dé/ñ ✓.txt", 0, 0, crcOf([]byte("abc")), 3, 3, []byte("abc")), `10 10`),
		"duplicate names":          readZip(zipOf(t, zipFile{"A.txt", []byte("1"), zip.Store}, zipFile{"a.TXT", []byte("2"), zip.Store}), `10 10`),
		"truncated deflate":        readZip(rawZip("t", 8, 0, crcOf(text), uint32(len(good)-5), uint32(len(text)), good[:len(good)-5]), `10 100000`),
		"deflate junk":             readZip(rawZip("j", 8, 0, crcOf(text), uint32(len(good)+3), uint32(len(text)), append(append([]byte{}, good...), 1, 2, 3)), `10 100000`),
		"expands past":             readZip(rawZip("p", 8, 0, crcOf(text), uint32(len(good)), 100, good), `10 100000`),
		"shorter":                  readZip(rawZip("s", 8, 0, crcOf(text), uint32(len(good)), uint32(len(text)+1), good), `10 100000`),
		"invalid block type":       readZip(rawZip("b", 8, 0, 0, 1, 1, []byte{0x07}), `10 10`),
		"bad stored length":        readZip(rawZip("n", 8, 0, 0, 7, 2, []byte{0x01, 0x02, 0x00, 0x00, 0x00, 'a', 'b'}), `10 10`),
		"distance too far":         readZip(rawZip("d", 8, 0, 0, 3, 5, []byte{0x63, 0x00, 0x02}), `10 10`),
		"empty deflate":            readZip(rawZip("e", 8, 0, 0, 0, 1, nil), `10 10`),
		"damaged deflate":          readZip(rawZip("x", 8, 0, crcOf(text), uint32(len(good)), uint32(len(text)), set(good, 40, 0xFF, 0xFF, 0xFF, 0xFF)), `10 100000`),
		"central crc":              readZip(set(two, len(two)-22-40, 0, 0, 0, 0), `10 1000000`),
		"damaged central":          readZip(set(two, int(binary.LittleEndian.Uint32(two[len(two)-22+16:])), 0, 0, 0, 0), `10 1000000`),
		"damaged local header":     readZip(set(two, 0, 0, 0, 0, 0), `10 1000000`),
		"multi-part":               readZip(set(two, len(two)-22+4, 1), `10 1000000`),
		"central out of range":     readZip(set(two, len(two)-22+16, 0xFF, 0xFF, 0, 0), `10 1000000`),
		"caught read":              `(print (try_let (z (zip_read "abc" 10 10)) (catch e "caught") (list_len z)))`,
	} {
		t.Run(name, func(t *testing.T) { sameAsJS(t, "(cli_app "+body+")") })
	}
}

// Every embedded standard-library module parses as one module form; text
// after it (a stray parenthesis ending the module early) is an error, not
// silently dropped definitions.
func TestStdModulesParse(t *testing.T) {
	entries, err := stdFiles.ReadDir("std")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".howl")
		if _, err := stdDefuns(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	_, err = stdModule("broken", []byte("(module\n  (defun a () (return 1)))\n  (defun b () (return 2)))"))
	if err == nil || !strings.Contains(err.Error(), "has text after its module form (line 3)") {
		t.Fatalf("want the trailing text refused, got %v", err)
	}
}
