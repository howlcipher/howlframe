package javascript

// browserIOJSHelper returns the storage, file and image runtime.
//
// Storage is IndexedDB: one database per namespace ("howlframe:" + name)
// with one object store of string values. Names and keys are checked, a full
// disk is STORAGE_QUOTA, and a host without IndexedDB (or a browser that
// refuses it, as private modes may) is STORAGE_UNAVAILABLE: a failed write is
// never reported as stored. Storage is origin-scoped browser state and needs
// no capability grant; it reaches no file system or network.
//
// file_read_input reads the files chosen in an <input type="file"> as
// base64, refusing any file over max_bytes before reading it. sniff_media_type
// names PNG, JPEG, GIF or WebP data from its leading bytes, so a file's
// declared type need not be trusted. image_decode reports dimensions;
// image_encode redraws the image (dropping metadata such as location),
// scaled to fit max_side, in the requested type.
func browserIOJSHelper() string {
	return `function howlFrameStorageName(kind, v, max) {
  if (typeof v !== "string") throw new Error("TYPE_ERROR: " + kind + " must be a string");
  if (v.length === 0 || v.length > max || (kind === "namespace" && !/^[a-z][a-z0-9-]*$/.test(v))) {
    throw new Error("STORAGE_INVALID_NAME: " + kind + " " + JSON.stringify(v.slice(0, 64)));
  }
  return v;
}
function howlFrameStorageError(err) {
  var name = err && err.name ? err.name : "";
  if (name === "QuotaExceededError" || /quota/i.test(String(err && err.message))) return new Error("STORAGE_QUOTA: browser storage is full");
  return new Error("STORAGE_UNAVAILABLE: " + (err && err.message ? err.message : "browser storage refused the request"));
}
function howlFrameStorageOpen(ns) {
  ns = howlFrameStorageName("namespace", ns, 40);
  var idb = typeof indexedDB !== "undefined" ? indexedDB : null;
  if (!idb) return Promise.reject(new Error("STORAGE_UNAVAILABLE: this host has no IndexedDB"));
  return new Promise(function (resolve, reject) {
    var r;
    try { r = idb.open("howlframe:" + ns, 1); } catch (e) { reject(howlFrameStorageError(e)); return; }
    r.onupgradeneeded = function () { var db = r.result; if (!db.objectStoreNames.contains("kv")) db.createObjectStore("kv"); };
    r.onsuccess = function () { resolve(r.result); };
    r.onerror = function () { reject(howlFrameStorageError(r.error)); };
    r.onblocked = function () { reject(new Error("STORAGE_UNAVAILABLE: storage is blocked by another tab")); };
  });
}
function howlFrameStorageRequest(ns, mode, fn) {
  return howlFrameStorageOpen(ns).then(function (db) {
    return new Promise(function (resolve, reject) {
      var tx, req, done = false;
      try { tx = db.transaction("kv", mode); req = fn(tx.objectStore("kv")); } catch (e) { db.close(); reject(howlFrameStorageError(e)); return; }
      var finish = function (v) { if (!done) { done = true; db.close(); resolve(v); } };
      var fail = function (e) { if (!done) { done = true; db.close(); reject(howlFrameStorageError(e)); } };
      if (mode === "readonly") {
        req.onsuccess = function () { finish(req.result); };
        req.onerror = function () { fail(req.error); };
      } else {
        tx.oncomplete = function () { finish(null); };
      }
      tx.onerror = function () { fail(tx.error); };
      tx.onabort = function () { fail(tx.error); };
    });
  });
}
function howlFrameStorageGet(ns, key) {
  key = howlFrameStorageName("key", key, 512);
  return howlFrameStorageRequest(ns, "readonly", function (s) { return s.get(key); }).then(function (v) { return typeof v === "string" ? v : null; });
}
function howlFrameStoragePut(ns, key, value) {
  key = howlFrameStorageName("key", key, 512);
  if (typeof value !== "string") throw new Error("TYPE_ERROR: storage_put value must be a string");
  return howlFrameStorageRequest(ns, "readwrite", function (s) { return s.put(value, key); });
}
function howlFrameStorageDelete(ns, key) {
  key = howlFrameStorageName("key", key, 512);
  return howlFrameStorageRequest(ns, "readwrite", function (s) { return s.delete(key); });
}
function howlFrameStorageKeys(ns) {
  return howlFrameStorageRequest(ns, "readonly", function (s) { return s.getAllKeys(); }).then(function (keys) {
    return (keys || []).filter(function (k) { return typeof k === "string"; }).sort();
  });
}
async function howlFrameFileReadInput(el, maxBytes) {
  if (el === null || el === undefined || typeof el !== "object") throw new Error("TYPE_ERROR: file_read_input expected an element");
  if (typeof maxBytes !== "number" || !(maxBytes > 0)) throw new Error("TYPE_ERROR: file_read_input max_bytes must be a positive number");
  var files = el.files ? Array.prototype.slice.call(el.files) : [];
  for (var i = 0; i < files.length; i++) {
    if (files[i].size > maxBytes) throw new Error("FILE_TOO_LARGE: " + files[i].name + " is larger than " + maxBytes + " bytes");
  }
  var out = [];
  for (var j = 0; j < files.length; j++) {
    var f = files[j];
    var bytes = new Uint8Array(await f.arrayBuffer());
    out.push({ name: String(f.name), type: String(f.type || ""), size: bytes.length, base64: howlFrameBytesBase64(bytes) });
  }
  return out;
}
function howlFrameSniffMediaType(b64) {
  if (typeof b64 !== "string") return "";
  var b;
  // The first 16 base64 characters are 12 bytes, enough for every signature.
  try { b = howlFrameBase64Bytes("TYPE_ERROR", b64.length > 16 ? b64.slice(0, 16) : b64); } catch (e) { return ""; }
  var at = function (i, v) { return b.length > i && b[i] === v; };
  if (at(0, 0x89) && at(1, 0x50) && at(2, 0x4e) && at(3, 0x47) && at(4, 0x0d) && at(5, 0x0a) && at(6, 0x1a) && at(7, 0x0a)) return "image/png";
  if (at(0, 0xff) && at(1, 0xd8) && at(2, 0xff)) return "image/jpeg";
  if (at(0, 0x47) && at(1, 0x49) && at(2, 0x46) && at(3, 0x38)) return "image/gif";
  if (at(0, 0x52) && at(1, 0x49) && at(2, 0x46) && at(3, 0x46) && at(8, 0x57) && at(9, 0x45) && at(10, 0x42) && at(11, 0x50)) return "image/webp";
  return "";
}
async function howlFrameImageBitmap(b64, mime) {
  if (typeof createImageBitmap === "undefined" || typeof Blob === "undefined") throw new Error("UNSUPPORTED_HOST: image operations need a browser");
  var bytes = howlFrameBase64Bytes("IMAGE_INVALID", b64);
  try {
    return await createImageBitmap(new Blob([bytes], { type: String(mime) }));
  } catch (e) {
    throw new Error("IMAGE_INVALID: the data is not an image this browser can decode");
  }
}
async function howlFrameImageDecode(b64, mime) {
  var bmp = await howlFrameImageBitmap(b64, mime);
  var info = { width: bmp.width, height: bmp.height };
  if (bmp.close) bmp.close();
  return info;
}
async function howlFrameImageEncode(b64, mime, maxSide, outMime, quality) {
  if (typeof maxSide !== "number" || maxSide < 1 || maxSide > 16384 || typeof quality !== "number" || quality <= 0 || quality > 1 ||
      ["image/png", "image/jpeg", "image/webp"].indexOf(outMime) < 0) {
    throw new Error("IMAGE_INVALID_OPTIONS: max_side 1..16384, quality (0, 1], type png, jpeg or webp");
  }
  var bmp = await howlFrameImageBitmap(b64, mime);
  var scale = Math.min(1, maxSide / Math.max(bmp.width, bmp.height));
  var w = Math.max(1, Math.round(bmp.width * scale)), h = Math.max(1, Math.round(bmp.height * scale));
  var blob;
  if (typeof OffscreenCanvas !== "undefined") {
    var oc = new OffscreenCanvas(w, h);
    oc.getContext("2d").drawImage(bmp, 0, 0, w, h);
    blob = await oc.convertToBlob({ type: outMime, quality: quality });
  } else if (typeof document !== "undefined") {
    var c = document.createElement("canvas");
    c.width = w; c.height = h;
    c.getContext("2d").drawImage(bmp, 0, 0, w, h);
    blob = await new Promise(function (resolve) { c.toBlob(resolve, outMime, quality); });
  } else {
    throw new Error("UNSUPPORTED_HOST: image operations need a browser");
  }
  if (bmp.close) bmp.close();
  if (!blob) throw new Error("IMAGE_INVALID: the image could not be encoded");
  return { base64: howlFrameBytesBase64(new Uint8Array(await blob.arrayBuffer())), width: w, height: h, type: blob.type || outMime };
}
`
}
