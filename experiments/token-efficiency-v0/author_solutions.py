"""Expand task-specific reference templates; never read oracle contents."""
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
CORPUS = ROOT.parent / 'falsifiable-v0'
manifest = json.loads((CORPUS / 'tasks/manifest.json').read_text())
for task in manifest['tasks'] + manifest['adversarial']:
    task_id = task['id']
    batch = task_id[1:]
    field, value = ('status', 'PASS') if task['shape']['loop'] else ('name', 'unit')
    conditional = task['shape']['conditional']
    approval = task['mutation'] == 'request_approval'
    template = 't02' if conditional else 't03' if approval else 't01'
    howl = (CORPUS / f'reference/arm_a/{template}.howl').read_text()
    howl = howl.replace(f'"{template[1:]}"', f'"{batch}"')
    if field == 'name':
        howl = howl.replace('(= (map_get c "status") "PASS")', '(= (map_get c "name") "unit")')
        if conditional:
            howl = howl.replace('(let (selected (list))', '(let (selected (list))\n     (let (failed (list))')
            howl = howl.replace('(let (record', '(for c checks (if (!= (map_get c "status") "PASS") (append failed c)))\n      (let (record')
            howl = howl.replace('(= (list_len selected) (list_len checks))', '(= (list_len failed) 0)')
            howl = howl.rstrip() + ')\n'
    existing = CORPUS / f'reference/arm_a/{task_id}.howl'
    if existing.exists():
        howl = existing.read_text()
    (ROOT / f'solutions/howl/{task_id}.howl').write_text(howl)

    python = f'''import json
from pathlib import Path

checks = json.loads(Path("evidence/checks.json").read_text())
version_text = Path("evidence/version.kv").read_text()
Path("evidence/note.txt").read_text()
selected = [check for check in checks if check["{field}"] == "{value}"]
record = {{"batch": "{batch}", "selected": selected}}
'''
    if conditional:
        python += '''versions = dict(line.split("=", 1) for line in version_text.splitlines())
current = tuple(map(int, versions["current"].split(".")))
proposed = tuple(map(int, versions["proposed"].split(".")))
if all(check["status"] == "PASS" for check in checks) and proposed > current:
    path = "out/release.json"
else:
    path = "out/approval_request.json"
    record = {"reason": "blocked", "summary": {}}
'''
    elif approval:
        python += 'path = "out/approval_request.json"\nrecord = {"reason": "release review", "summary": record}\n'
    else:
        python += 'path = "out/release.json"\n'
    python += 'Path(path).write_text(json.dumps(record, indent=2) + "\\n")\n'
    (ROOT / f'solutions/python/{task_id}.py').write_text(python)

    js = f'''const fs = require("node:fs");

const checks = JSON.parse(fs.readFileSync("evidence/checks.json", "utf8"));
const versionText = fs.readFileSync("evidence/version.kv", "utf8");
fs.readFileSync("evidence/note.txt", "utf8");
const selected = checks.filter(check => check.{field} === "{value}");
let record = {{ batch: "{batch}", selected }};
'''
    if conditional:
        js += '''const versions = Object.fromEntries(versionText.trim().split("\\n").map(line => line.split("=")));
const current = versions.current.split(".").map(Number);
const proposed = versions.proposed.split(".").map(Number);
let newer = false;
for (let i = 0; i < 3; i++) {
    if (proposed[i] !== current[i]) {
        newer = proposed[i] > current[i];
        break;
    }
}
let path;
if (checks.every(check => check.status === "PASS") && newer) {
    path = "out/release.json";
} else {
    path = "out/approval_request.json";
    record = { reason: "blocked", summary: {} };
}
'''
    elif approval:
        js += 'const path = "out/approval_request.json";\nrecord = { reason: "release review", summary: record };\n'
    else:
        js += 'const path = "out/release.json";\n'
    js += 'fs.writeFileSync(path, JSON.stringify(record, null, 2) + "\\n");\n'
    (ROOT / f'solutions/js/{task_id}.js').write_text(js)

    go = '''//go:build ignore

package main

import (
    "encoding/json"
    "os"
'''
    if conditional:
        go += '    "strconv"\n    "strings"\n'
    go += ''')

func mustRead(path string) []byte {
    data, err := os.ReadFile(path)
    if err != nil { panic(err) }
    return data
}
'''
    if conditional:
        go += '''
func version(text string) [3]int {
    var result [3]int
    for i, part := range strings.Split(text, ".") {
        n, err := strconv.Atoi(part)
        if err != nil { panic(err) }
        result[i] = n
    }
    return result
}
'''
    go += '''
func main() {
    var checks []map[string]interface{}
    if err := json.Unmarshal(mustRead("evidence/checks.json"), &checks); err != nil { panic(err) }
'''
    go += ('    versionText := mustRead("evidence/version.kv")\n' if conditional else '    mustRead("evidence/version.kv")\n')
    go += '    mustRead("evidence/note.txt")\n    selected := make([]map[string]interface{}, 0)\n'
    if conditional:
        go += '    allPassed := true\n'
    go += f'''    for _, check := range checks {{
        if check["{field}"] == "{value}" {{ selected = append(selected, check) }}
'''
    if conditional:
        go += '        if check["status"] != "PASS" { allPassed = false }\n'
    go += f'''    }}
    record := map[string]interface{{}}{{"batch": "{batch}", "selected": selected}}
'''
    if conditional:
        go += '''    versions := make(map[string]string)
    for _, line := range strings.Fields(string(versionText)) {
        key, value, _ := strings.Cut(line, "=")
        versions[key] = value
    }
    current, proposed := version(versions["current"]), version(versions["proposed"])
    newer := false
    for i := 0; i < 3; i++ {
        if proposed[i] != current[i] {
            newer = proposed[i] > current[i]
            break
        }
    }
    path := "out/release.json"
    if !allPassed || !newer {
        path = "out/approval_request.json"
        record = map[string]interface{}{"reason": "blocked", "summary": map[string]interface{}{}}
    }
'''
    elif approval:
        go += '    path := "out/approval_request.json"\n    record = map[string]interface{}{"reason": "release review", "summary": record}\n'
    else:
        go += '    path := "out/release.json"\n'
    go += '''    data, err := json.MarshalIndent(record, "", "  ")
    if err != nil { panic(err) }
    if err := os.WriteFile(path, append(data, '\\n'), 0600); err != nil { panic(err) }
}
'''
    (ROOT / f'solutions/go/{task_id}.go').write_text(go)
