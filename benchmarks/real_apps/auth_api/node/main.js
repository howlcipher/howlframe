#!/usr/bin/env node
const http = require("http");
const { URL } = require("url");

const server = http.createServer((req, res) => {
  const send = (code, body) => {
    const data = JSON.stringify(body);
    res.writeHead(code, { "Content-Type": "application/json" });
    res.end(data);
  };
  const u = new URL(req.url, "http://127.0.0.1");
  const auth = req.headers.authorization || "";
  if (u.pathname === "/health") send(200, { status: "ok" });
  else if (u.pathname === "/secret") {
    if (auth === "Bearer secret") send(200, { ok: "1", scope: "secret" });
    else send(401, { error: "unauthorized" });
  } else if (u.pathname.startsWith("/echo/")) {
    if (req.method !== "GET") {
      send(405, { error: "method_not_allowed" });
      return;
    }
    if (auth !== "Bearer secret") {
      send(401, { error: "unauthorized" });
      return;
    }
    send(200, { name: u.pathname.slice("/echo/".length), q: u.searchParams.get("q") || "" });
  } else {
    res.writeHead(404);
    res.end();
  }
});

server.listen(8102);
