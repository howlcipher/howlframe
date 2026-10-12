const fs = require("fs");

const text = fs.readFileSync("in/brackets.txt", "utf8").trim();
const pairs = { ")": "(", "]": "[", "}": "{" };
const stack = [];
let result = null;
for (let i = 0; i < text.length && result === null; i++) {
  const ch = text[i];
  if ("([{".includes(ch)) {
    stack.push(ch);
  } else if (ch in pairs && stack.pop() !== pairs[ch]) {
    result = `error at ${i}`;
  }
}
if (result === null) {
  result = stack.length ? "unclosed" : "ok";
}
fs.writeFileSync("out/result.txt", result + "\n");
