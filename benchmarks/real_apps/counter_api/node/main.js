#!/usr/bin/env node
const http = require("http");

let n = 0;
const server = http.createServer((req, res) => {
  const send = (code, body) => {
    const data = JSON.stringify(body);
    res.writeHead(code, { "Content-Type": "application/json" });
    res.end(data);
  };
  if (req.url === "/count" && req.method === "GET") send(200, { n: String(n) });
  else if (req.url === "/inc" && req.method === "POST") {
    n += 1;
    send(200, { n: String(n) });
  } else if (req.url === "/reset" && req.method === "POST") {
    n = 0;
    send(200, { n: "0" });
  } else if ((req.url === "/inc" || req.url === "/reset") && req.method !== "POST") {
    send(405, { error: "method_not_allowed" });
  } else {
    res.writeHead(404);
    res.end();
  }
});

server.listen(8103);
