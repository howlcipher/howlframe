const fs = require("fs");

const edges = Object.fromEntries(fs.readFileSync("in/edges.txt", "utf8").split("\n").map((line) => line.split(">")));
const targets = new Set(Object.values(edges));
let node = Object.keys(edges).find((n) => !targets.has(n));
const chain = [node];
while (node in edges) {
  node = edges[node];
  chain.push(node);
}
fs.writeFileSync("out/result.txt", chain.join(">") + "\n");
