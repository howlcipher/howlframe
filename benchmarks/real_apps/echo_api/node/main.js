#!/usr/bin/env node
const http = require("http");
const { URL } = require("url");

function send(res, code, obj) {
  const body = JSON.stringify(obj);
  res.writeHead(code, { "Content-Type": "application/json", "Content-Length": Buffer.byteLength(body) });
  res.end(body);
}

http.createServer((req, res) => {
  const u = new URL(req.url, "http://127.0.0.1");
  if (u.pathname === "/health") return send(res, 200, { status: "ok" });
  if (u.pathname === "/echo") return send(res, 200, { method: req.method, q: u.searchParams.get("q") || "" });
  if (u.pathname.startsWith("/echo/")) {
    return send(res, 200, { name: u.pathname.slice("/echo/".length), method: req.method });
  }
  send(res, 404, { error: "not_found" });
}).listen(8106, "127.0.0.1");
