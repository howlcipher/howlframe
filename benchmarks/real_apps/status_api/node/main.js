#!/usr/bin/env node
const http = require("http");

const server = http.createServer((req, res) => {
  const send = (code, body) => {
    const data = JSON.stringify(body);
    res.writeHead(code, { "Content-Type": "application/json" });
    res.end(data);
  };
  if (req.url === "/health") send(200, { status: "ok" });
  else if (req.url === "/ready") {
    if ((process.env.REQUIRED_ENV || "") !== "") send(200, { status: "ready" });
    else send(503, { status: "not ready", reason: "missing REQUIRED_ENV" });
  } else if (req.url === "/version") send(200, { version: "1.0.0", app: "status_api" });
  else if (req.url === "/config") send(200, { public_config: process.env.PUBLIC_CONFIG || "" });
  else {
    res.writeHead(404);
    res.end();
  }
});

server.listen(8090);
