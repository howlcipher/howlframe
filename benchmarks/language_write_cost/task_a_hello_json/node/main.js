const http = require("http");

const server = http.createServer((req, res) => {
  if (req.url === "/json") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({
      status: "success",
      message: "Hello from HowlFrame JSON endpoint!",
    }));
  } else {
    res.writeHead(200, { "Content-Type": "text/plain" });
    res.end("Hello, World! HowlFrame language is alive!");
  }
});

server.listen(8080);
