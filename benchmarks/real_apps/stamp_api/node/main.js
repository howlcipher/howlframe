#!/usr/bin/env node
const http = require("http");

const server = http.createServer((req, res) => {
  const send = (code, body) => {
    const data = JSON.stringify(body);
    res.writeHead(code, { "Content-Type": "application/json" });
    res.end(data);
  };
  if (req.url === "/health") send(200, { status: "ok" });
  else if (req.url === "/now") send(200, { unix: String(Math.floor(Date.now() / 1000)) });
  else if (req.url.startsWith("/echo/")) {
    if (req.method !== "GET") {
      send(405, { error: "method_not_allowed" });
      return;
    }
    send(200, {
      name: req.url.slice("/echo/".length),
      unix: String(Math.floor(Date.now() / 1000)),
    });
  } else {
    res.writeHead(404);
    res.end();
  }
});

server.listen(8104);
