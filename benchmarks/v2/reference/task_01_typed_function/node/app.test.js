const assert = require('node:assert');
const test = require('node:test');
/** @param {number} a @param {number} b @returns {number} */
function add(a, b) { return a + b; }
test('add', () => { assert.strictEqual(add(2, 3), 5); });
