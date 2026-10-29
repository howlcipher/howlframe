package javascript

// archiveJSHelper returns the browser archive runtime used by dom_text,
// zip_build, and download_file.
//
// zip_build writes a ZIP of stored (uncompressed) entries. Each entry is a
// dict with a "path" and exactly one of "text" (encoded as UTF-8) or "base64"
// (raw bytes). Output is deterministic: entries keep their list order, every
// entry carries the fixed MS-DOS timestamp 1980-01-01 00:00, and no extra
// fields are written. Paths are relative, "/"-separated, and limited to a
// conservative portable character set, so an archive can never name a file
// outside the directory it is extracted into. Paths that differ only in case
// are rejected because they collide on case-insensitive file systems.
//
// download_file hands bytes to the browser's download mechanism through a
// Blob object URL that is revoked after the click. It writes no file itself;
// the browser decides where the download is saved. Without a document it
// fails with UNSUPPORTED_HOST instead of silently doing nothing.
func archiveJSHelper() string {
	return `var howlFrameZipMaxEntries = 10000;
var howlFrameZipMaxBytes = 512 * 1024 * 1024;
var howlFrameZipCRCTable = null;
function howlFrameZipCRC32(bytes) {
  if (howlFrameZipCRCTable === null) {
    howlFrameZipCRCTable = new Uint32Array(256);
    for (var n = 0; n < 256; n++) {
      var c = n;
      for (var k = 0; k < 8; k++) c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1);
      howlFrameZipCRCTable[n] = c >>> 0;
    }
  }
  var crc = 0xFFFFFFFF;
  for (var i = 0; i < bytes.length; i++) crc = howlFrameZipCRCTable[(crc ^ bytes[i]) & 0xFF] ^ (crc >>> 8);
  return (crc ^ 0xFFFFFFFF) >>> 0;
}
function howlFrameUTF8(s) {
  var out = [];
  for (var i = 0; i < s.length; i++) {
    var cp = s.codePointAt(i);
    if (cp > 0xFFFF) i++;
    if (cp >= 0xD800 && cp <= 0xDFFF) cp = 0xFFFD;
    if (cp < 0x80) out.push(cp);
    else if (cp < 0x800) out.push(0xC0 | (cp >> 6), 0x80 | (cp & 63));
    else if (cp < 0x10000) out.push(0xE0 | (cp >> 12), 0x80 | ((cp >> 6) & 63), 0x80 | (cp & 63));
    else out.push(0xF0 | (cp >> 18), 0x80 | ((cp >> 12) & 63), 0x80 | ((cp >> 6) & 63), 0x80 | (cp & 63));
  }
  return Uint8Array.from(out);
}
function howlFrameBase64Bytes(code, v) {
  if (typeof v !== "string" || v.length % 4 !== 0 || !/^[A-Za-z0-9+/]*={0,2}$/.test(v)) {
    throw new Error(code + ": expected standard base64 data");
  }
  var bin = atob(v);
  var out = new Uint8Array(bin.length);
  for (var i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}
function howlFrameBytesBase64(bytes) {
  var s = "";
  for (var i = 0; i < bytes.length; i += 0x8000) {
    s += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
  }
  return btoa(s);
}
function howlFrameZipPathOK(p) {
  if (typeof p !== "string" || p.length === 0 || p.length > 255) return false;
  var parts = p.split("/");
  for (var i = 0; i < parts.length; i++) {
    var seg = parts[i];
    if (seg === "" || seg === "." || seg === ".." || !/^[A-Za-z0-9._~@+-]+$/.test(seg)) return false;
  }
  return true;
}
function howlFrameZipBuild(entries) {
  if (!Array.isArray(entries)) {
    throw new Error("TYPE_ERROR: zip_build expected list, got " + (entries === null ? "null" : typeof entries));
  }
  if (entries.length > howlFrameZipMaxEntries) {
    throw new Error("ZIP_LIMIT: more than " + howlFrameZipMaxEntries + " entries");
  }
  var files = [];
  var seen = Object.create(null);
  var total = 0;
  for (var i = 0; i < entries.length; i++) {
    var e = entries[i];
    if (e === null || typeof e !== "object" || Array.isArray(e)) {
      throw new Error("ZIP_INVALID_ENTRY: entry " + i + " is not a dict");
    }
    var path = Object.prototype.hasOwnProperty.call(e, "path") ? e.path : null;
    if (!howlFrameZipPathOK(path)) {
      throw new Error("ZIP_INVALID_PATH: entry " + i + " has an unsafe path");
    }
    var key = path.toLowerCase();
    if (seen[key]) throw new Error("ZIP_DUPLICATE_PATH: " + path);
    seen[key] = true;
    var hasText = Object.prototype.hasOwnProperty.call(e, "text");
    var hasB64 = Object.prototype.hasOwnProperty.call(e, "base64");
    if (hasText === hasB64) {
      throw new Error("ZIP_INVALID_ENTRY: " + path + " needs exactly one of text or base64");
    }
    var data;
    if (hasText) {
      if (typeof e.text !== "string") throw new Error("ZIP_INVALID_ENTRY: " + path + " text must be a string");
      data = howlFrameUTF8(e.text);
    } else {
      data = howlFrameBase64Bytes("ZIP_INVALID_ENTRY", e.base64);
    }
    total += data.length;
    if (total > howlFrameZipMaxBytes) throw new Error("ZIP_LIMIT: archive exceeds " + howlFrameZipMaxBytes + " bytes");
    files.push({ name: howlFrameUTF8(path), data: data, crc: howlFrameZipCRC32(data) });
  }
  var size = 22;
  for (var j = 0; j < files.length; j++) size += 30 + 46 + 2 * files[j].name.length + files[j].data.length;
  var buf = new Uint8Array(size);
  var dv = new DataView(buf.buffer);
  var off = 0;
  var DOS_DATE = 0x21; // 1980-01-01
  function header(f, central, localOffset) {
    dv.setUint32(off, central ? 0x02014b50 : 0x04034b50, true); off += 4;
    if (central) { dv.setUint16(off, 20, true); off += 2; }
    dv.setUint16(off, 20, true); off += 2;       // version needed
    dv.setUint16(off, 0x0800, true); off += 2;   // UTF-8 names
    dv.setUint16(off, 0, true); off += 2;        // stored
    dv.setUint16(off, 0, true); off += 2;        // time
    dv.setUint16(off, DOS_DATE, true); off += 2; // date
    dv.setUint32(off, f.crc, true); off += 4;
    dv.setUint32(off, f.data.length, true); off += 4;
    dv.setUint32(off, f.data.length, true); off += 4;
    dv.setUint16(off, f.name.length, true); off += 2;
    dv.setUint16(off, 0, true); off += 2;        // extra length
    if (central) {
      dv.setUint16(off, 0, true); off += 2;      // comment length
      dv.setUint16(off, 0, true); off += 2;      // disk
      dv.setUint16(off, 0, true); off += 2;      // internal attributes
      dv.setUint32(off, 0, true); off += 4;      // external attributes
      dv.setUint32(off, localOffset, true); off += 4;
    }
    buf.set(f.name, off); off += f.name.length;
  }
  var offsets = [];
  for (var a = 0; a < files.length; a++) {
    offsets.push(off);
    header(files[a], false, 0);
    buf.set(files[a].data, off); off += files[a].data.length;
  }
  var cdStart = off;
  for (var b = 0; b < files.length; b++) header(files[b], true, offsets[b]);
  var cdSize = off - cdStart;
  dv.setUint32(off, 0x06054b50, true); off += 4;
  dv.setUint16(off, 0, true); off += 2;
  dv.setUint16(off, 0, true); off += 2;
  dv.setUint16(off, files.length, true); off += 2;
  dv.setUint16(off, files.length, true); off += 2;
  dv.setUint32(off, cdSize, true); off += 4;
  dv.setUint32(off, cdStart, true); off += 4;
  dv.setUint16(off, 0, true); off += 2;
  return howlFrameBytesBase64(buf);
}
function howlFrameDownloadFile(name, mime, data) {
  if (typeof name !== "string" || !/^[A-Za-z0-9._ ()-]{1,128}$/.test(name) || /^[. ]/.test(name)) {
    throw new Error("DOWNLOAD_INVALID_NAME: file name must be a plain name without path separators");
  }
  if (typeof mime !== "string" || !/^[A-Za-z0-9.+-]+\/[A-Za-z0-9.+-]+$/.test(mime)) {
    throw new Error("DOWNLOAD_INVALID_TYPE: expected a type/subtype media type");
  }
  var bytes = howlFrameBase64Bytes("DOWNLOAD_INVALID_DATA", data);
  if (typeof document === "undefined" || typeof Blob === "undefined" || typeof URL === "undefined") {
    throw new Error("UNSUPPORTED_HOST: download_file needs a browser document");
  }
  var url = URL.createObjectURL(new Blob([bytes], { type: mime }));
  var a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.rel = "noopener";
  a.style.display = "none";
  document.body.appendChild(a);
  try {
    a.click();
  } finally {
    document.body.removeChild(a);
    setTimeout(function () { URL.revokeObjectURL(url); }, 0);
  }
}
function howlFrameDomText(el) {
  if (el === null || el === undefined || typeof el !== "object") {
    throw new Error("TYPE_ERROR: dom_text expected an element");
  }
  var t = el.textContent;
  return typeof t === "string" ? t : "";
}
`
}
