#!/usr/bin/env node
const http = require("http");
const https = require("https");
const { URL } = require("url");

const args = process.argv.slice(2);
if (args.length < 1 || args[0] === "") {
  console.log("Usage: fetch_cli <url> [method]");
  process.exit(2);
}
const method = args[1] && args[1] !== "" ? args[1] : "GET";
const u = new URL(args[0]);
const lib = u.protocol === "https:" ? https : http;
const req = lib.request(u, { method }, (res) => {
  let body = "";
  res.on("data", (c) => (body += c));
  res.on("end", () => {
    console.log(`status=${res.statusCode}`);
    process.stdout.write(body);
    process.exit(0);
  });
});
req.on("error", (e) => {
  console.log(String(e));
  process.exit(2);
});
req.end();
