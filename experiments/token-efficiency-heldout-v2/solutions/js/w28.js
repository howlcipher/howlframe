const fs = require("fs");

const moves = { "closed open": "opened", "opened close": "closed", "closed lock": "locked", "locked unlock": "closed" };
let state = "closed";
let result = null;
const cmds = fs.readFileSync("in/cmds.txt", "utf8").split("\n");
for (let i = 0; i < cmds.length; i++) {
  const next = moves[`${state} ${cmds[i]}`];
  if (!next) {
    result = `error line ${i + 1}: cannot ${cmds[i]} when ${state}`;
    break;
  }
  state = next;
}
fs.writeFileSync("out/result.txt", (result || state) + "\n");
