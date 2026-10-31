package javascript_test

import (
	"strings"
	"testing"
)

// hostHarness runs code with browser stand-ins: an in-memory IndexedDB with
// the subset the storage helpers use (optionally failing puts with a quota
// error), a file input holding two files, and image decode and encode.
func hostHarness(t *testing.T, code program, quotaFull bool) string {
	t.Helper()
	quota := "false"
	if quotaFull {
		quota = "true"
	}
	wrapper := howlNormJS + `const vm = require("node:vm");
const out = [];
const dbs = {};
const quotaFull = ` + quota + `;
function req(fn) { const r = {}; setTimeout(() => { try { r.result = fn(); r.onsuccess && r.onsuccess({ target: r }); } catch (e) { r.error = e; r.onerror && r.onerror({ target: r }); } }, 0); return r; }
const indexedDB = {
  open(name, version) {
    const r = {};
    setTimeout(() => {
      const fresh = !dbs[name];
      const db = dbs[name] || (dbs[name] = { stores: {}, objectStoreNames: { contains(n) { return !!db.stores[n]; } } });
      db.createObjectStore = (n) => { db.stores[n] = new Map(); };
      db.close = () => {};
      db.transaction = (n, mode) => {
        const tx = { error: null };
        const store = db.stores[n];
        tx.objectStore = () => ({
          get: (k) => req(() => store.has(k) ? store.get(k) : undefined),
          put: (v, k) => { if (quotaFull) { const e = new Error("full"); e.name = "QuotaExceededError"; setTimeout(() => { tx.error = e; tx.onerror && tx.onerror({ target: tx }); }, 0); return {}; }
                           store.set(k, v); setTimeout(() => tx.oncomplete && tx.oncomplete(), 0); return {}; },
          delete: (k) => { store.delete(k); setTimeout(() => tx.oncomplete && tx.oncomplete(), 0); return {}; },
          getAllKeys: () => req(() => [...store.keys()]),
        });
        return tx;
      };
      r.result = db;
      if (fresh) r.onupgradeneeded && r.onupgradeneeded({ target: r });
      r.onsuccess && r.onsuccess({ target: r });
    }, 0);
    return r;
  },
};
const files = [new File([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2])], "photo.png", { type: "image/png" }),
               new File([Buffer.from("hello")], "notes.txt", { type: "text/plain" })];
const input = { files };
const bigInput = { files: [new File([Buffer.alloc(2048)], "big.png", { type: "image/png" })] };
async function createImageBitmap(blob) {
  const b = Buffer.from(await blob.arrayBuffer());
  if (b[0] !== 0x89) throw new Error("bad image");
  return { width: 4000, height: 3000, close() {} };
}
class OffscreenCanvas {
  constructor(w, h) { this.width = w; this.height = h; }
  getContext() { const c = this; return { drawImage(img, x, y, w, h) { c.drew = [w, h]; } }; }
  async convertToBlob(opts) { return new Blob([Buffer.from("re-encoded:" + this.width + "x" + this.height + ":" + opts.type)], { type: opts.type }); }
}
const browser = { console: { log: (v) => out.push(howlNorm(v)), error: () => {} }, indexedDB, Blob, createImageBitmap, OffscreenCanvas, atob, btoa,
  document: { querySelector: (s) => s === "#files" ? input : s === "#big" ? bigInput : null } };
Promise.resolve(` + code.inContext("browser") + `).then(
  () => setTimeout(() => console.log(JSON.stringify(out)), 20),
  (err) => console.log(JSON.stringify({ error: String(err.message), out })));`
	stdout, stderr, exit := runNode(t, wrapper)
	if exit != 0 {
		t.Fatalf("harness: %s %s", stdout, stderr)
	}
	return strings.TrimSpace(stdout)
}

func TestBrowserStorageRoundTrip(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (do
  (print (is_nil (storage_get "drafts" "site")))
  (storage_put "drafts" "site" "{\"title\":\"é\"}")
  (storage_put "drafts" "other" "x")
  (print (storage_get "drafts" "site"))
  (print (encode_json (storage_keys "drafts")))
  (storage_delete "drafts" "other")
  (print (encode_json (storage_keys "drafts")))
  (print (encode_json (storage_keys "elsewhere")))))`)
		want := `["true","{\"title\":\"é\"}","[\"other\",\"site\"]","[\"site\"]","[]"]`
		if got := hostHarness(t, code, false); got != want {
			t.Fatalf("got %s\nwant %s", got, want)
		}
	})
}

func TestBrowserStorageFailuresAreReported(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (do
  (try_let (r (storage_put "drafts" "site" "x")) (catch err (print (to_string err))) (print "stored"))
  (try_let (r (storage_put "Bad NS" "k" "x")) (catch err (print (to_string err))) (print "stored"))
  (try_let (r (storage_put "drafts" "k" 5)) (catch err (print (to_string err))) (print "stored"))))`)
		got := hostHarness(t, code, true)
		for _, want := range []string{"STORAGE_QUOTA", "STORAGE_INVALID_NAME", "TYPE_ERROR"} {
			if !strings.Contains(got, want) {
				t.Fatalf("want %s in %s", want, got)
			}
		}
		if strings.Contains(got, "stored") {
			t.Fatalf("a failed put must not report success: %s", got)
		}
	})
}

func TestBrowserStorageWithoutIndexedDBFailsClosed(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (try_let (v (storage_get "drafts" "k")) (catch err (print (to_string err))) (print "read")))`)
		stdout, _, _ := runNode(t, code.script())
		if !strings.Contains(stdout, "STORAGE_UNAVAILABLE") {
			t.Fatalf("want STORAGE_UNAVAILABLE without indexedDB, got %q", stdout)
		}
	})
}

func TestFileReadInputAndSniffing(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (do
  (for f (file_read_input (dom_query "#files") 1024)
    (print (str_join (list (map_get f "name") (map_get f "type") (to_string (map_get f "size")) (sniff_media_type (map_get f "base64"))) "|")))
  (try_let (r (file_read_input (dom_query "#big") 1024)) (catch err (print (to_string err))) (print "read big"))
  (print (sniff_media_type "/9j/4AAQ"))
  (print (sniff_media_type "R0lGODlh"))
  (print (sniff_media_type "UklGRiQAAABXRUJQVlA4"))
  (print (sniff_media_type "PHN2Zz4="))))`)
		got := hostHarness(t, code, false)
		want := `["photo.png|image/png|10|image/png","notes.txt|text/plain|5|","FILE_TOO_LARGE: big.png is larger than 1024 bytes","image/jpeg","image/gif","image/webp",""]`
		if got != want {
			t.Fatalf("got %s\nwant %s", got, want)
		}
	})
}

func TestImageDecodeAndReencode(t *testing.T) {
	forBackends(t, func(t *testing.T, b string) {
		code := build(t, b, `(web_app (let (png "iVBORw0KGgoBAg==") (do
  (let (info (image_decode png "image/png"))
    (print (str_join (list (to_string (map_get info "width")) (to_string (map_get info "height"))) "x")))
  (let (out (image_encode png "image/png" 1600 "image/webp" 0.82))
    (do
      (print (str_join (list (to_string (map_get out "width")) (to_string (map_get out "height")) (map_get out "type")) " "))))
  (try_let (r (image_decode "aGVsbG8=" "image/png")) (catch err (print (to_string err))) (print "decoded"))
  (try_let (r (image_encode png "image/png" 0 "image/webp" 0.8)) (catch err (print (to_string err))) (print "encoded")))))`)
		got := hostHarness(t, code, false)
		for _, want := range []string{`"4000x3000"`, `"1600 1200 image/webp"`, "IMAGE_INVALID", "IMAGE_INVALID_OPTIONS"} {
			if !strings.Contains(got, want) {
				t.Fatalf("want %s in %s", want, got)
			}
		}
	})
}
