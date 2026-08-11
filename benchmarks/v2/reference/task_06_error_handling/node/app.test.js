const assert = require('node:assert');
const test = require('node:test');
function divide(a, b) {
    if (b === 0) throw new Error("Division by zero");
    return a / b;
}
test('divide', () => {
    assert.throws(() => divide(10, 0), /Division by zero/);
});
