package javascript

// richtextJSHelper returns the rich text runtime for a contenteditable
// element. Its model is a list of blocks:
//
//	{"type": "p" | "h2" | "h3", "content": runs}
//	{"type": "ul" | "ol", "items": [runs, ...]}
//
// where runs is a list of {"text": s} with optional "bold", "italic" (true)
// and "link" (an href string). richtext_set builds the DOM with createElement
// and text nodes only, never parsing HTML. richtext_get reads it back through
// an allowlist: b/strong, i/em and a[href] keep their meaning, p/div become
// paragraphs, h1 and h2 become h2 and smaller headings h3, ul/ol/li become
// lists, br becomes a line break, and any other element contributes only its
// text. Links are returned as written; checking them is the program's job.
// richtext_attach makes the element an accessible multiline textbox and
// pastes only plain text.
func richtextJSHelper() string {
	return `function howlFrameRichtextInvalid(why) { return new Error("RICHTEXT_INVALID: " + why); }
function howlFrameRichtextRuns(runs) {
  if (!Array.isArray(runs)) throw howlFrameRichtextInvalid("runs must be a list");
  return runs.map(function (r) {
    if (r === null || typeof r !== "object" || Array.isArray(r) || typeof r.text !== "string") throw howlFrameRichtextInvalid("a run needs text");
    if (r.link !== undefined && r.link !== null && typeof r.link !== "string") throw howlFrameRichtextInvalid("a link must be text");
    return r;
  });
}
// The DOM nodes for one run: its text (with br for line breaks) inside a, b
// and i elements as its format requires, built with appendChild only.
function howlFrameRichtextRunNodes(r) {
  var content = [];
  r.text.split("\n").forEach(function (part, i) {
    if (i > 0) content.push(document.createElement("br"));
    if (part !== "") content.push(document.createTextNode(part));
  });
  var chain = [];
  if (r.link) { var a = document.createElement("a"); a.setAttribute("href", r.link); chain.push(a); }
  if (r.bold) chain.push(document.createElement("b"));
  if (r.italic) chain.push(document.createElement("i"));
  if (chain.length === 0) return content;
  content.forEach(function (n) { chain[chain.length - 1].appendChild(n); });
  for (var k = chain.length - 1; k > 0; k--) chain[k - 1].appendChild(chain[k]);
  return [chain[0]];
}
function howlFrameRichtextAppendRuns(parent, runs) {
  howlFrameRichtextRuns(runs).forEach(function (r) {
    howlFrameRichtextRunNodes(r).forEach(function (n) { parent.appendChild(n); });
  });
}
function howlFrameRichtextSet(el, blocks) {
  if (el === null || el === undefined || typeof el !== "object") throw new Error("TYPE_ERROR: richtext_set expected an element");
  if (!Array.isArray(blocks)) throw howlFrameRichtextInvalid("the model must be a list of blocks");
  var built = blocks.map(function (b) {
    if (b === null || typeof b !== "object") throw howlFrameRichtextInvalid("a block must be an object");
    var node;
    if (b.type === "p" || b.type === "h2" || b.type === "h3") {
      node = document.createElement(b.type);
      howlFrameRichtextAppendRuns(node, b.content);
    } else if (b.type === "ul" || b.type === "ol") {
      if (!Array.isArray(b.items)) throw howlFrameRichtextInvalid("a list needs items");
      node = document.createElement(b.type);
      b.items.forEach(function (runs) {
        var li = document.createElement("li");
        howlFrameRichtextAppendRuns(li, runs);
        node.appendChild(li);
      });
    } else {
      throw howlFrameRichtextInvalid("unknown block type " + JSON.stringify(b.type));
    }
    return node;
  });
  el.textContent = "";
  built.forEach(function (n) { el.appendChild(n); });
}
function howlFrameRichtextCollect(node, fmt, out) {
  if (node.nodeType === 3) { out.push({ text: node.data, fmt: fmt }); return; }
  if (node.nodeType !== 1) return;
  var tag = node.tagName;
  if (tag === "BR") { out.push({ text: "\n", fmt: fmt }); return; }
  var f = { bold: fmt.bold, italic: fmt.italic, link: fmt.link };
  if (tag === "B" || tag === "STRONG") f.bold = true;
  if (tag === "I" || tag === "EM") f.italic = true;
  if (tag === "A" && node.getAttribute("href") !== null) f.link = node.getAttribute("href");
  for (var i = 0; i < node.childNodes.length; i++) howlFrameRichtextCollect(node.childNodes[i], f, out);
}
function howlFrameRichtextMerge(pieces) {
  var runs = [];
  pieces.forEach(function (p) {
    var last = runs[runs.length - 1];
    var same = last && !!last.bold === !!p.fmt.bold && !!last.italic === !!p.fmt.italic && (last.link || null) === (p.fmt.link || null);
    if (same) { last.text += p.text; return; }
    var r = { text: p.text };
    if (p.fmt.bold) r.bold = true;
    if (p.fmt.italic) r.italic = true;
    if (p.fmt.link) r.link = p.fmt.link;
    runs.push(r);
  });
  return runs.filter(function (r) { return r.text !== ""; });
}
function howlFrameRichtextInline(nodes) {
  var pieces = [];
  nodes.forEach(function (n) { howlFrameRichtextCollect(n, {}, pieces); });
  return howlFrameRichtextMerge(pieces);
}
function howlFrameRichtextHasText(runs) {
  return runs.some(function (r) { return r.text.trim() !== ""; });
}
function howlFrameRichtextBlockType(n) {
  var tag = n.nodeType === 1 ? n.tagName : "";
  if (tag === "P" || tag === "DIV") return "p";
  if (tag === "H1" || tag === "H2") return "h2";
  if (/^H[3-6]$/.test(tag)) return "h3";
  if (tag === "UL") return "ul";
  if (tag === "OL") return "ol";
  return "";
}
function howlFrameRichtextHasBlocks(n) {
  for (var i = 0; i < n.childNodes.length; i++) {
    if (howlFrameRichtextBlockType(n.childNodes[i]) !== "") return true;
  }
  return false;
}
// Reads the blocks of container into blocks. Inline nodes between blocks
// form paragraphs; a p or div that holds blocks is read as a container,
// because browsers nest blocks inside paragraphs while editing.
function howlFrameRichtextBlocks(container, blocks) {
  var loose = [];
  var flush = function () {
    var runs = howlFrameRichtextInline(loose);
    if (howlFrameRichtextHasText(runs)) blocks.push({ type: "p", content: runs });
    loose = [];
  };
  for (var i = 0; i < container.childNodes.length; i++) {
    var n = container.childNodes[i];
    var type = howlFrameRichtextBlockType(n);
    if (type === "") { loose.push(n); continue; }
    flush();
    if (type === "p" && howlFrameRichtextHasBlocks(n)) {
      howlFrameRichtextBlocks(n, blocks);
    } else if (type === "ul" || type === "ol") {
      var items = [];
      for (var j = 0; j < n.childNodes.length; j++) {
        var li = n.childNodes[j];
        var runs = howlFrameRichtextInline(li.nodeType === 1 && li.tagName === "LI" ? li.childNodes : [li]);
        if (howlFrameRichtextHasText(runs)) items.push(runs);
      }
      if (items.length) blocks.push({ type: type, items: items });
    } else {
      var content = howlFrameRichtextInline(n.childNodes);
      if (howlFrameRichtextHasText(content)) blocks.push({ type: type, content: content });
    }
  }
  flush();
}
function howlFrameRichtextGet(el) {
  if (el === null || el === undefined || typeof el !== "object") throw new Error("TYPE_ERROR: richtext_get expected an element");
  var blocks = [];
  howlFrameRichtextBlocks(el, blocks);
  return blocks;
}
function howlFrameRichtextAttach(el, label) {
  if (el === null || el === undefined || typeof el !== "object") throw new Error("TYPE_ERROR: richtext_attach expected an element");
  el.setAttribute("contenteditable", "true");
  el.setAttribute("role", "textbox");
  el.setAttribute("aria-multiline", "true");
  el.setAttribute("aria-label", String(label));
  el.addEventListener("paste", function (e) {
    e.preventDefault();
    var text = e.clipboardData ? e.clipboardData.getData("text/plain") : "";
    document.execCommand("insertText", false, text);
  });
  el.addEventListener("drop", function (e) { e.preventDefault(); });
}
var howlFrameRichtextCommands = {
  bold: ["bold", false], italic: ["italic", false], link: ["createLink", true], unlink: ["unlink", false],
  p: ["formatBlock", "<p>"], h2: ["formatBlock", "<h2>"], h3: ["formatBlock", "<h3>"],
  ul: ["insertUnorderedList", false], ol: ["insertOrderedList", false],
};
function howlFrameRichtextCommand(el, cmd, value) {
  if (el === null || el === undefined || typeof el !== "object") throw new Error("TYPE_ERROR: richtext_command expected an element");
  if (!Object.prototype.hasOwnProperty.call(howlFrameRichtextCommands, cmd)) throw new Error("RICHTEXT_UNKNOWN_COMMAND: " + String(cmd));
  var spec = howlFrameRichtextCommands[cmd];
  el.focus();
  if (spec[1] === true) {
    if (typeof value !== "string" || value === "") throw new Error("RICHTEXT_INVALID: link needs an address");
    document.execCommand(spec[0], false, value);
  } else if (typeof spec[1] === "string") {
    document.execCommand(spec[0], false, spec[1]);
  } else {
    document.execCommand(spec[0], false);
  }
}
`
}
