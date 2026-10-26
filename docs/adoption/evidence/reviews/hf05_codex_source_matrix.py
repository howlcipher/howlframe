"""Bounded independent HF-05 probes; all effects stay in this workspace."""

import json
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[1]
WRITE = '(write_file "fx.txt" "x")'
CASES = {
    "try_binding": f'(try_let (r {WRITE}) (catch e (print "c")) (print "b"))',
    "dict_value": f'(print (encode_json (dict ("k" {WRITE}))))',
    "return_value": f'(do (defun f () (return {WRITE})) (print (call f)))',
    "read_path": f'(print (read_file {WRITE}))',
    "write_data": f'(write_file "outer.txt" {WRITE})',
    "mkdir_path": f'(mkdir {WRITE})',
    "sleep_duration": f'(sleep {WRITE})',
    "stderr_value": f'(stderr {WRITE})',
    "encode_value": f'(print (encode_json {WRITE}))',
    "convert_value": f'(print (to_string {WRITE}))',
    "for_iterable": f'(for i {WRITE} (print i))',
    "match_subject": f'(match {WRITE} (default (print "default")))',
    "if_partial": f'(do {WRITE} (let (r (if false 1)) (print r)))',
    "match_partial": f'(do {WRITE} (print (match 2 (1 42))))',
    "nested_do": f'(print (do (do {WRITE})))',
    "dict_nested_try": f'(print (dict ("k" (try_let (r {WRITE}) (catch e 0) 1))))',
    "if_void": f'(print (if true {WRITE} {WRITE}))',
    "for_result": f'(print (for i (list 1) {WRITE}))',
    "map_set_result": '(let (m (dict)) (print (map_set m "k" 1)))',
    "append_result": '(let (l (list)) (print (append l 1)))',
    "map_set_key": f'(let (m (dict)) (map_set m {WRITE} 1))',
    "store_put_result": '(do (store_open s "memory://hf05") (print (store_put s "k" (dict ("v" 1)))))',
    "spawn_result": '(print (spawn (lambda () (print "effect"))))',
    "void_call_dict": f'(do (defun f () void {WRITE}) (print (dict ("k" (call f)))))',
    "void_call_read": f'(do (defun f () void {WRITE}) (print (read_file (call f))))',
    "valid_statements": f'(do {WRITE} (if true (write_file "fx.txt" "y") (do)) (print "ok"))',
    "valid_void": f'(do (defun save () void {WRITE}) (call save) (print "ok"))',
    "valid_do": f'(try_let (r (do {WRITE} 1)) (catch e (print "caught")) (print r))',
}


def run(args, directory):
    env = dict(os.environ, GOPROXY="off", HOWL_FORBID_LOCAL_INFERENCE="1")
    env["HOWLFRAME_ALLOW_CAPS"] = f"filesystem:write={directory}"
    try:
        result = subprocess.run(
            [str(arg) for arg in args], cwd=directory, env=env,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
            text=True, timeout=30,
        )
        return {"rc": result.returncode, "out": result.stdout}
    except subprocess.TimeoutExpired:
        return {"rc": -999, "out": "TIMEOUT"}


def main():
    records = []
    for version in ("base", "pr"):
        binary = ROOT / "evidence" / f"{version}-howlframe"
        selected = CASES if version == "pr" else {
            key: CASES[key] for key in
            ("try_binding", "dict_value", "return_value")
        }
        for name, body in selected.items():
            for mode in ("check", "build", "bc", "hfir", "direct", "go", "js"):
                directory = ROOT / "probes" / "runs" / version / name / mode
                directory.mkdir(parents=True, exist_ok=True)
                source = directory / "p.howl"
                root = "web_app" if mode == "js" else "cli_app"
                source.write_text(f"({root} {body})\n")
                artifact = directory / "p.hfbc"
                commands = {
                    "check": [binary, "check", source],
                    "build": [binary, "build", source, "-o", artifact],
                    "bc": [binary, "-compile-bc", source, "-o", artifact],
                    "hfir": [binary, "-compile-hfir-bc", source, "-o", artifact],
                    "direct": [binary, "-run", "-allow-caps",
                               f"filesystem:write={directory}", source],
                    "go": [binary, "build", "--target", "go", source,
                           "-o", directory / "server.go"],
                    "js": [binary, "build", "--target", "js", source,
                           "-o", directory / "app.js"],
                }
                result = run(commands[mode], directory)
                record = {"version": version, "case": name, "mode": mode,
                          "command": [str(v) for v in commands[mode]],
                          "compile": result}
                if result["rc"] == 0 and mode in ("build", "bc", "hfir"):
                    record["run"] = run(
                        [binary, "-run-bc", "-allow-caps",
                         f"filesystem:write={directory},database", artifact],
                        directory,
                    )
                record["effects"] = {
                    path.name: path.read_text() for path in
                    (directory / "fx.txt", directory / "outer.txt")
                    if path.is_file()
                }
                records.append(record)
                status = record.get("run", result)
                first_line = status["out"].strip().splitlines()
                print(version, name, mode, "compile", result["rc"],
                      "run", status["rc"], "effects", record["effects"],
                      first_line[0] if first_line else "", flush=True)
    (ROOT / "evidence" / "source-matrix.json").write_text(
        json.dumps(records, indent=2) + "\n"
    )


if __name__ == "__main__":
    main()
