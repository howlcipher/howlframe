package javascript_test

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

type zipFixture struct {
	name   string
	data   []byte
	method uint16
}

func makeZip(t *testing.T, files ...zipFixture) []byte {
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

// zipReadProgram reads an archive and prints one "path=base64" line per
// entry, or the error.
func zipReadProgram(archive []byte, maxEntries, maxBytes int) string {
	return fmt.Sprintf(`(web_app
  (try_let (entries (zip_read %q %d %d))
    (catch err (print (to_string err)))
    (for e entries (print (+ (map_get e "path") (+ "=" (map_get e "base64")))))))`,
		base64.StdEncoding.EncodeToString(archive), maxEntries, maxBytes)
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestZipReadReadsStoredAndDeflatedEntries(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		big := strings.Repeat("compressible text ", 500)
		archive := makeZip(t,
			zipFixture{"project.json", []byte(`{"a":"é"}`), zip.Store},
			zipFixture{"assets/", nil, zip.Store},
			zipFixture{"assets/notes.txt", []byte(big), zip.Deflate},
			zipFixture{"assets/empty.bin", nil, zip.Deflate},
		)
		got := runWebAppOutput(t, b, zipReadProgram(archive, 10, 1<<20))
		want := strings.Join([]string{
			"project.json=" + b64(`{"a":"é"}`),
			"assets/notes.txt=" + b64(big),
			"assets/empty.bin=",
		}, "\n")
		if got != want {
			t.Fatalf("directories are skipped and entries keep their order:\nwant %q\ngot  %q", want, got)
		}
	})
}

func TestZipReadRoundTripsZipBuild(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		got := runWebAppOutput(t, b, `(web_app
  (let (z (zip_build (list (dict ("path" "a.txt") ("text" "Héllo")) (dict ("path" "b/c.bin") ("base64" "AAEC/w==")))))
    (for e (zip_read z 10 1000) (print (+ (map_get e "path") (+ "=" (map_get e "base64")))))))`)
		if want := "a.txt=" + b64("Héllo") + "\nb/c.bin=AAEC/w=="; got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	})
}

func TestZipReadRejectsUnsafePaths(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		for _, name := range []string{"../evil.txt", "a/../../evil.txt", "/etc/passwd", "a//b", "a\\..\\b", "C:/x", "a/./b", "bad\x01name"} {
			got := runWebAppOutput(t, b, zipReadProgram(makeZip(t, zipFixture{name, []byte("x"), zip.Store}), 10, 100))
			if !strings.HasPrefix(got, "ZIP_INVALID_PATH") {
				t.Errorf("%q: want ZIP_INVALID_PATH, got %q", name, got)
			}
		}
		got := runWebAppOutput(t, b, zipReadProgram(makeZip(t, zipFixture{"My Photos/été 1.png", []byte("x"), zip.Store}), 10, 100))
		if got != "My Photos/été 1.png="+b64("x") {
			t.Fatalf("an ordinary name with spaces and accents must be readable, got %q", got)
		}
	})
}

func TestZipReadRejectsDuplicatePaths(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		archive := makeZip(t, zipFixture{"a.txt", []byte("1"), zip.Store}, zipFixture{"A.TXT", []byte("2"), zip.Store})
		if got := runWebAppOutput(t, b, zipReadProgram(archive, 10, 100)); !strings.HasPrefix(got, "ZIP_DUPLICATE_PATH") {
			t.Fatalf("want ZIP_DUPLICATE_PATH, got %q", got)
		}
	})
}

func TestZipReadEnforcesLimits(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		three := makeZip(t, zipFixture{"a", []byte("1"), zip.Store}, zipFixture{"b", []byte("2"), zip.Store}, zipFixture{"c", []byte("3"), zip.Store})
		if got := runWebAppOutput(t, b, zipReadProgram(three, 2, 100)); !strings.HasPrefix(got, "ZIP_LIMIT") {
			t.Fatalf("too many entries: want ZIP_LIMIT, got %q", got)
		}
		big := makeZip(t, zipFixture{"zeros", make([]byte, 5000), zip.Deflate})
		if got := runWebAppOutput(t, b, zipReadProgram(big, 10, 4999)); !strings.HasPrefix(got, "ZIP_LIMIT") {
			t.Fatalf("too many bytes: want ZIP_LIMIT, got %q", got)
		}
		if got := runWebAppOutput(t, b, zipReadProgram(big, 0, 100)); !strings.HasPrefix(got, "TYPE_ERROR") {
			t.Fatalf("a non-positive limit is a TYPE_ERROR, got %q", got)
		}
	})
}

// setSizes rewrites the uncompressed size of the only entry in both headers.
func setSizes(archive []byte, size uint32) []byte {
	out := append([]byte(nil), archive...)
	local := bytes.Index(out, []byte("PK\x03\x04"))
	binary.LittleEndian.PutUint32(out[local+22:], size)
	central := bytes.Index(out, []byte("PK\x01\x02"))
	binary.LittleEndian.PutUint32(out[central+24:], size)
	return out
}

func TestZipReadStopsAnEntryThatExpandsPastItsDeclaredSize(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		// A small declared size passes the byte limit, but the data inflates to
		// 1 MiB: reading must stop at the declared size, not trust it.
		bomb := setSizes(makeZip(t, zipFixture{"zeros", make([]byte, 1<<20), zip.Deflate}), 100)
		if got := runWebAppOutput(t, b, zipReadProgram(bomb, 10, 1000)); got != "ZIP_CORRUPT: zeros expands past its declared size" {
			t.Fatalf("want ZIP_CORRUPT, got %q", got)
		}
	})
}

func TestZipReadRejectsDamagedArchives(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		good := makeZip(t, zipFixture{"a.txt", []byte("hello"), zip.Store})
		crc := append([]byte(nil), good...)
		crc[bytes.Index(crc, []byte("hello"))] = 'j'
		cases := map[string][]byte{
			"not a zip": []byte("just some text, not an archive"),
			"truncated": good[:len(good)-10],
			"empty":     {},
		}
		for name, data := range cases {
			if got := runWebAppOutput(t, b, zipReadProgram(data, 10, 100)); !strings.HasPrefix(got, "ZIP_INVALID") {
				t.Errorf("%s: want ZIP_INVALID, got %q", name, got)
			}
		}
		if got := runWebAppOutput(t, b, zipReadProgram(crc, 10, 100)); !strings.HasPrefix(got, "ZIP_CORRUPT") {
			t.Errorf("bad checksum: want ZIP_CORRUPT, got %q", got)
		}
		encrypted := append([]byte(nil), good...)
		for _, sig := range []string{"PK\x03\x04", "PK\x01\x02"} {
			at := bytes.Index(encrypted, []byte(sig))
			flagAt := at + 6
			if sig == "PK\x01\x02" {
				flagAt = at + 8
			}
			encrypted[flagAt] |= 1
		}
		if got := runWebAppOutput(t, b, zipReadProgram(encrypted, 10, 100)); !strings.HasPrefix(got, "ZIP_UNSUPPORTED") {
			t.Errorf("encrypted: want ZIP_UNSUPPORTED, got %q", got)
		}
		bzip := append([]byte(nil), good...)
		binary.LittleEndian.PutUint16(bzip[bytes.Index(bzip, []byte("PK\x01\x02"))+10:], 12)
		if got := runWebAppOutput(t, b, zipReadProgram(bzip, 10, 100)); !strings.HasPrefix(got, "ZIP_UNSUPPORTED") {
			t.Errorf("bzip2 method: want ZIP_UNSUPPORTED, got %q", got)
		}
	})
}

func TestBase64TextDecodesStrictUTF8(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		got := runWebAppOutput(t, b, fmt.Sprintf(`(web_app
  (print (base64_text %q))
  (print (base64_text ""))
  (try_let (s (base64_text "/w==")) (catch err (print (to_string err))) (print s))
  (try_let (s (base64_text "not base64!")) (catch err (print (to_string err))) (print s)))`, b64("Héllo ✓")))
		lines := strings.Split(got, "\n")
		if len(lines) != 4 || lines[0] != "Héllo ✓" || lines[1] != "" ||
			!strings.HasPrefix(lines[2], "TEXT_INVALID_UTF8") || !strings.HasPrefix(lines[3], "TEXT_INVALID_BASE64") {
			t.Fatalf("unexpected output %q", got)
		}
	})
}
