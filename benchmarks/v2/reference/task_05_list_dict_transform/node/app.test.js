const assert = require('node:assert');
const test = require('node:test');
function processScores(items) {
    return items.filter(i => i.Score >= 50).sort((a, b) => b.Score - a.Score).map(i => i.Name);
}
test('processScores', () => {
    const res = processScores([{Name: "Alice", Score: 40}, {Name: "Bob", Score: 80}, {Name: "Charlie", Score: 90}]);
    assert.deepStrictEqual(res, ["Charlie", "Bob"]);
});
