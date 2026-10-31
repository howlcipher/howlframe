package javascript_test

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func openZipBase64(t *testing.T, encoded string) *zip.Reader {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("zip_build did not return base64: %v (%q)", err, encoded)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("archive/zip rejected zip_build output: %v", err)
	}
	return reader
}

func zipEntryText(t *testing.T, f *zip.File) string {
	t.Helper()
	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc) // archive/zip verifies the CRC-32 at EOF.
	if err != nil {
		t.Fatalf("%s: %v", f.Name, err)
	}
	return string(data)
}

func TestZipBuildProducesStandardArchive(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		out := runWebAppOutput(t, b, `(web_app
  (print (zip_build (list
    (dict ("path" "index.html") ("text" "<h1>Héllo ✓</h1>"))
    (dict ("path" "assets/logo.bin") ("base64" "AAEC/w==")))))
)`)
		reader := openZipBase64(t, out)
		if len(reader.File) != 2 {
			t.Fatalf("want 2 entries, got %d", len(reader.File))
		}
		if reader.File[0].Name != "index.html" || reader.File[1].Name != "assets/logo.bin" {
			t.Fatalf("entry order not preserved: %s, %s", reader.File[0].Name, reader.File[1].Name)
		}
		if got := zipEntryText(t, reader.File[0]); got != "<h1>Héllo ✓</h1>" {
			t.Fatalf("text entry must be UTF-8 encoded, got %q", got)
		}
		if got := zipEntryText(t, reader.File[1]); got != "\x00\x01\x02\xff" {
			t.Fatalf("base64 entry must be stored as raw bytes, got %q", got)
		}
		if reader.File[0].Flags&0x800 == 0 {
			t.Fatal("entries must set the UTF-8 file name flag")
		}
	})
}

func TestZipBuildIsDeterministic(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		src := `(web_app
  (let (entries (list (dict ("path" "a.txt") ("text" "same"))))
    (print (= (zip_build entries) (zip_build entries))))
)`
		if out := runWebAppOutput(t, b, src); out != "true" {
			t.Fatalf("identical inputs must produce identical archives, got %q", out)
		}
		out := runWebAppOutput(t, b, `(web_app (print (zip_build (list (dict ("path" "a.txt") ("text" "x"))))))`)
		reader := openZipBase64(t, out)
		if mod := reader.File[0].Modified; mod.Year() != 1980 || mod.Month() != 1 || mod.Day() != 1 {
			t.Fatalf("entries must carry the fixed 1980-01-01 timestamp, got %v", mod)
		}
	})
}

func TestZipBuildEmptyListIsValidArchive(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		reader := openZipBase64(t, runWebAppOutput(t, b, `(web_app (print (zip_build (list))))`))
		if len(reader.File) != 0 {
			t.Fatalf("want empty archive, got %d entries", len(reader.File))
		}
	})
}

func TestZipBuildRejectsUnsafeEntries(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		for _, tc := range []struct {
			name, entries, code string
		}{
			{"parent traversal", `(dict ("path" "../evil.txt") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"nested traversal", `(dict ("path" "a/../../evil.txt") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"absolute", `(dict ("path" "/etc/passwd") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"backslash", `(dict ("path" "a\\b.txt") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"drive letter", `(dict ("path" "C:/x.txt") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"empty segment", `(dict ("path" "a//b.txt") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"dot segment", `(dict ("path" "a/./b.txt") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"trailing slash", `(dict ("path" "dir/") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"empty path", `(dict ("path" "") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"control char", `(dict ("path" "a` + "\t" + `b") ("text" "x"))`, "ZIP_INVALID_PATH"},
			{"duplicate", `(dict ("path" "a.txt") ("text" "x")) (dict ("path" "a.txt") ("text" "y"))`, "ZIP_DUPLICATE_PATH"},
			{"case duplicate", `(dict ("path" "A.txt") ("text" "x")) (dict ("path" "a.txt") ("text" "y"))`, "ZIP_DUPLICATE_PATH"},
			{"no content", `(dict ("path" "a.txt"))`, "ZIP_INVALID_ENTRY"},
			{"both contents", `(dict ("path" "a.txt") ("text" "x") ("base64" "AA=="))`, "ZIP_INVALID_ENTRY"},
			{"bad base64", `(dict ("path" "a.bin") ("base64" "not base64!"))`, "ZIP_INVALID_ENTRY"},
			{"non-dict entry", `"a.txt"`, "ZIP_INVALID_ENTRY"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				src := `(web_app
  (try_let (z (zip_build (list ` + tc.entries + `)))
    (catch err (print (to_string err)))
    (print "BUILT")))`
				out := runWebAppOutput(t, b, src)
				if !strings.Contains(out, tc.code) {
					t.Fatalf("want %s, got %q", tc.code, out)
				}
			})
		}
	})
}

func TestZipBuildRejectsNonList(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		out := runWebAppOutput(t, b, `(web_app
  (try_let (z (zip_build "x"))
    (catch err (print (to_string err)))
    (print "BUILT")))`)
		if !strings.Contains(out, "TYPE_ERROR") {
			t.Fatalf("want TYPE_ERROR, got %q", out)
		}
	})
}

func TestZipBuildEnforcesEntryLimit(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		out := runWebAppOutput(t, b, `(web_app
  (let (entries (list))
    (let (i 0)
      (do
        (while (< i 10001)
          (do
            (append entries (dict ("path" (str_join (list "f" (to_string i)) "")) ("text" "")))
            (set i (+ i 1))))
        (try_let (z (zip_build entries))
          (catch err (print (to_string err)))
          (print "BUILT"))))))`)
		if !strings.Contains(out, "ZIP_LIMIT") {
			t.Fatalf("want ZIP_LIMIT for more than 10000 entries, got %q", out)
		}
	})
}

// browserDownloadHarness runs code in a vm context that records the anchor
// download a browser would perform.
func browserDownloadHarness(t *testing.T, code program, withDocument bool) map[string]any {
	t.Helper()
	docSetup := ""
	if withDocument {
		docSetup = `
browser.Blob = class { constructor(parts, opts) { this.parts = parts; this.type = opts.type; } };
browser.URL = { createObjectURL: (b) => { record.blob = {type: b.type, bytes: Array.from(b.parts[0])}; return "blob:test"; },
                revokeObjectURL: (u) => { record.revoked = u; } };
browser.document = {
  createElement: (tag) => ({ tag, style: {}, click() { record.clicked = {tag: this.tag, href: this.href, download: this.download, rel: this.rel}; } }),
  body: { appendChild: () => { record.appended = true; }, removeChild: () => { record.removed = true; } }
};
browser.setTimeout = (fn) => fn();`
	}
	wrapper := howlNormJS + `const vm = require("node:vm");
const record = {};
const browser = { console: { log: (v) => { record.log = String(v); }, error: () => {} }, atob, btoa, TextEncoder, Uint8Array };` + docSetup + `
Promise.resolve(` + code.inContext("browser") + `).then(
  () => setTimeout(() => console.log(JSON.stringify(record)), 10),
  (err) => console.log(JSON.stringify({ error: String(err.message) }))
);`
	stdout, stderr, exit := runNode(t, wrapper)
	if exit != 0 {
		t.Fatalf("harness failed: %s %s", stdout, stderr)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(stdout), &record); err != nil {
		t.Fatalf("bad harness output %q: %v", stdout, err)
	}
	return record
}

func TestDownloadFileUsesBlobAnchorAndRevokes(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app
  (do
    (download_file "site.zip" "application/zip" "AAEC/w==")
    (print "done")))`)
		record := browserDownloadHarness(t, code, true)
		clicked, ok := record["clicked"].(map[string]any)
		if !ok {
			t.Fatalf("anchor was not clicked: %v", record)
		}
		if clicked["tag"] != "a" || clicked["download"] != "site.zip" || clicked["href"] != "blob:test" {
			t.Fatalf("unexpected anchor: %v", clicked)
		}
		blob := record["blob"].(map[string]any)
		if blob["type"] != "application/zip" {
			t.Fatalf("blob type: %v", blob)
		}
		got, _ := json.Marshal(blob["bytes"])
		if string(got) != "[0,1,2,255]" {
			t.Fatalf("blob bytes must be decoded base64, got %s", got)
		}
		if record["revoked"] != "blob:test" || record["removed"] != true {
			t.Fatalf("object URL and anchor must be cleaned up: %v", record)
		}
		if record["log"] != "done" {
			t.Fatalf("download_file must complete before the next statement: %v", record)
		}
	})
}

func TestDownloadFileRejectsUnsafeNameAndBadInput(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		for _, tc := range []struct{ name, call, code string }{
			{"path separator", `(download_file "../x.zip" "application/zip" "AA==")`, "DOWNLOAD_INVALID_NAME"},
			{"empty name", `(download_file "" "application/zip" "AA==")`, "DOWNLOAD_INVALID_NAME"},
			{"bad mime", `(download_file "x.zip" "text/html\r\nX: y" "AA==")`, "DOWNLOAD_INVALID_TYPE"},
			{"bad base64", `(download_file "x.zip" "application/zip" "%%%")`, "DOWNLOAD_INVALID_DATA"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				code := build(t, b, `(web_app (try_let (r `+tc.call+`) (catch err (print (to_string err))) (print "OK")))`)
				record := browserDownloadHarness(t, code, true)
				if record["clicked"] != nil {
					t.Fatalf("rejected download must not click: %v", record)
				}
				if log, _ := record["log"].(string); !strings.Contains(log, tc.code) {
					t.Fatalf("want %s, got %v", tc.code, record)
				}
			})
		}
	})
}

func TestDownloadFileWithoutDocumentFailsClosed(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (try_let (r (download_file "x.zip" "application/zip" "AA==")) (catch err (print (to_string err))) (print "OK")))`)
		record := browserDownloadHarness(t, code, false)
		if log, _ := record["log"].(string); !strings.Contains(log, "UNSUPPORTED_HOST") {
			t.Fatalf("download without a document must fail with UNSUPPORTED_HOST, got %v", record)
		}
	})
}

func TestDomTextReadsTextContent(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (print (dom_text (dom_query "#tpl"))))`)
		wrapper := howlNormJS + `const vm = require("node:vm");
const out = [];
const browser = { console: { log: (v) => out.push(howlNorm(v)), error: () => {} },
  document: { querySelector: (s) => s === "#tpl" ? { textContent: "<b>raw</b> text" } : null } };
Promise.resolve(` + code.inContext("browser") + `).then(() => console.log(out.join("|")));`
		stdout, stderr, exit := runNode(t, wrapper)
		if exit != 0 || strings.TrimSpace(stdout) != "<b>raw</b> text" {
			t.Fatalf("dom_text must return textContent unparsed: %q %q", stdout, stderr)
		}
	})
}

func TestDomTextMissingElementIsTypeError(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (try_let (v (dom_text (dom_query "#none"))) (catch err (print (to_string err))) (print v)))`)
		wrapper := howlNormJS + `const vm = require("node:vm");
const out = [];
const browser = { console: { log: (v) => out.push(howlNorm(v)), error: () => {} }, document: { querySelector: () => null } };
Promise.resolve(` + code.inContext("browser") + `).then(() => console.log(out.join("|")));`
		stdout, _, _ := runNode(t, wrapper)
		if !strings.Contains(stdout, "TYPE_ERROR") {
			t.Fatalf("dom_text of a missing element must be TYPE_ERROR, got %q", stdout)
		}
	})
}

// A try_let whose catch variable is named e must still see the error. The
// generated catch clause bound its own parameter as e, so assigning e = e
// wrote to that parameter and the outer variable stayed null: the error was
// dropped and the success branch ran with an undefined value.
func TestTryLetCatchVariableNamedEReceivesError(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		for _, name := range []string{"e", "err", "error"} {
			out := runWebAppOutput(t, b, `(web_app
  (let (raw "{not json")
    (try_let (v (parse_json Any raw))
      (catch `+name+` (print "caught"))
      (print "missed"))))`)
			if out != "caught" {
				t.Fatalf("catch variable %s: want caught, got %q", name, out)
			}
		}
	})
}

// Dict keys are data. In JavaScript, assigning obj["__proto__"] or writing a
// literal {"__proto__": v} replaces the object's prototype instead of adding
// a key, so the key vanished from map_keys and inherited values appeared.
func TestDictProtoKeyIsAnOrdinaryKey(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		out := runWebAppOutput(t, b, `(web_app
  (let (d (dict))
    (let (lit (dict ("__proto__" (dict ("polluted" "yes")))))
      (let (raw "{\"__proto__\":{\"polluted\":\"yes\"}}")
        (try_let (parsed (parse_json Any raw))
          (catch err (print "parse failed"))
          (do
            (map_set d "__proto__" (dict ("polluted" "yes")))
            (print (encode_json (map_keys d)))
            (print (is_nil (map_get d "polluted")))
            (print (encode_json (map_keys lit)))
            (print (is_nil (map_get lit "polluted")))
            (map_set parsed "__proto__" (dict ("polluted" "again")))
            (print (encode_json (map_keys parsed)))
            (print (is_nil (map_get parsed "polluted")))
            (map_delete d "__proto__")
            (print (encode_json (map_keys d)))))))))`)
		want := "[\"__proto__\"]\ntrue\n[\"__proto__\"]\ntrue\n[\"__proto__\"]\ntrue\n[]"
		if out != want {
			t.Fatalf("got:\n%s\nwant:\n%s", out, want)
		}
	})
}
