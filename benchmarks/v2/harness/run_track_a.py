#!/usr/bin/env python3
import argparse
import json
import os
import shutil
import subprocess
import tempfile
import time
from datetime import datetime, timezone

try:
    import tiktoken
    HAS_TIKTOKEN = True
except ImportError:
    HAS_TIKTOKEN = False

def count_tokens(text):
    if HAS_TIKTOKEN:
        try:
            enc = tiktoken.get_encoding("cl100k_base")
            return len(enc.encode(text)), "tiktoken(cl100k_base)"
        except Exception:
            pass
    return len(text) // 4, "character-estimate"

# -- HowlFrame Verifiers --
def run_howlframe_task01(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    r_c = subprocess.run(["howlframe", "-compile-bc", source_path, "-o", bc_path], capture_output=True, text=True)
    if r_c.returncode != 0: return False, r_c.stderr, 1
    r = subprocess.run(["howlframe", "-run-bc", bc_path], capture_output=True, text=True)
    if r.returncode != 0 or "Pass" not in r.stdout: return False, f"{r.stdout}\n{r.stderr}", 2
    return True, "Pass", 2

def run_howlframe_task02(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    r_c = subprocess.run(["howlframe", "-compile-bc", source_path, "-o", bc_path], capture_output=True, text=True)
    if r_c.returncode != 0: return False, r_c.stderr, 1
    test_txt = os.path.join(workspace, "names.txt")
    with open(test_txt, "w") as f: f.write("Alice\nBob\n")
    r1 = subprocess.run(["howlframe", "-allow-caps", "filesystem", "-run-bc", bc_path, test_txt], capture_output=True, text=True)
    if r1.returncode != 0 or "Hello, Alice" not in r1.stdout: return False, f"Failed normal run", 2
    r2 = subprocess.run(["howlframe", "-allow-caps", "filesystem", "-run-bc", bc_path, "missing_file.txt"], capture_output=True, text=True)
    if r2.returncode != 1 or "Error: File not found" not in r2.stderr: return False, "Failed missing run", 3
    return True, "Pass", 3

def run_howlframe_task03(workspace, source_path):
    bc_path = os.path.join(workspace, "server.bc")
    r_c = subprocess.run(["howlframe", "-compile-bc", source_path, "-o", bc_path], capture_output=True, text=True)
    if r_c.returncode != 0: return False, r_c.stderr, 1
    server = subprocess.Popen(["howlframe", "-allow-caps", "network", "-run-bc", bc_path])
    time.sleep(1)
    try:
        import urllib.request
        req1 = urllib.request.urlopen("http://localhost:8080/")
        res1 = req1.read().decode('utf-8')
        if "Hello World" not in res1: return False, "GET / failed", 1
        req2 = urllib.request.urlopen("http://localhost:8080/json")
        res2 = req2.read().decode('utf-8')
        if '"message"' not in res2 or req2.getheader("Content-Type") != "application/json": return False, "GET /json failed", 1
        return True, "Pass", 1
    except Exception as e: return False, str(e), 1
    finally:
        server.terminate(); server.wait()

def run_howlframe_task04(workspace, source_path):
    if os.path.isdir(source_path):
        for item in os.listdir(source_path): shutil.copy2(os.path.join(source_path, item), workspace)
    else: shutil.copy2(source_path, os.path.join(workspace, "app.howl"))
    bc_path = os.path.join(workspace, "app.bc")
    r_c = subprocess.run(["howlframe", "-compile-bc", os.path.join(workspace, "app.howl"), "-o", bc_path], capture_output=True, text=True)
    if r_c.returncode != 0: return False, r_c.stderr, 1
    r = subprocess.run(["howlframe", "-run-bc", bc_path], capture_output=True, text=True)
    if r.returncode != 0 or "20" not in r.stdout: return False, "Failed", 2
    return True, "Pass", 2

def run_howlframe_task06(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    r_c = subprocess.run(["howlframe", "-compile-bc", source_path, "-o", bc_path], capture_output=True, text=True)
    if r_c.returncode != 0: return False, r_c.stderr, 1
    r = subprocess.run(["howlframe", "-run-bc", bc_path], capture_output=True, text=True)
    if r.returncode != 0 or "Error: Division by zero" not in r.stdout: return False, "Failed", 2
    return True, "Pass", 2

def run_howlframe_task07(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    r_c = subprocess.run(["howlframe", "-compile-bc", source_path, "-o", bc_path], capture_output=True, text=True)
    if r_c.returncode != 0: return False, r_c.stderr, 1
    r = subprocess.run(["howlframe", "-allow-caps", "database,filesystem", "-run-bc", bc_path], capture_output=True, text=True)
    if r.returncode != 0: return False, "Failed", 2
    return True, "Pass", 2

def run_howlframe_task08(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    r_c = subprocess.run(["howlframe", "-compile-bc", source_path, "-o", bc_path], capture_output=True, text=True)
    if r_c.returncode != 0: return False, r_c.stderr, 1
    r = subprocess.run(["howlframe", "-allow-caps", "environment", "-run-bc", bc_path], capture_output=True, text=True)
    if r.returncode != 0 or "Access Denied" not in r.stdout: return False, "Failed", 2
    return True, "Pass", 2

# -- Go Verifiers --
def run_go_generic_test(workspace, source_path):
    # Copy files
    if os.path.isdir(source_path):
        for item in os.listdir(source_path): shutil.copy2(os.path.join(source_path, item), workspace)
    else: shutil.copy2(source_path, os.path.join(workspace, "main_test.go"))
    
    # Init module if needed
    if not os.path.exists(os.path.join(workspace, "go.mod")):
        subprocess.run(["go", "mod", "init", "test_app"], cwd=workspace, capture_output=True)
    
    # Run tests
    r = subprocess.run(["go", "test", "-v", "./..."], cwd=workspace, capture_output=True, text=True)
    if r.returncode != 0: return False, r.stderr + r.stdout, 1
    return True, "Pass", 1

# -- Python Verifiers --
def run_python_generic_test(workspace, source_path):
    if os.path.isdir(source_path):
        for item in os.listdir(source_path): shutil.copy2(os.path.join(source_path, item), workspace)
    else: shutil.copy2(source_path, os.path.join(workspace, "test_app.py"))
    
    r = subprocess.run(["python3", "-m", "unittest", "discover", "-s", workspace, "-p", "test_*.py"], capture_output=True, text=True)
    # Some people just write assert statements without unittest, so we also just run the file if unittest discovers nothing
    if r.returncode == 0 and "Ran 0 tests" in r.stderr:
        test_file = os.path.join(workspace, "test_app.py")
        if os.path.exists(test_file):
            r = subprocess.run(["python3", test_file], capture_output=True, text=True)
    
    if r.returncode != 0: return False, r.stderr + r.stdout, 1
    return True, "Pass", 1

# -- Node Verifiers --
def run_node_generic_test(workspace, source_path):
    if os.path.isdir(source_path):
        for item in os.listdir(source_path): shutil.copy2(os.path.join(source_path, item), workspace)
    else: shutil.copy2(source_path, os.path.join(workspace, "app.test.js"))
    
    r = subprocess.run(["node", "--test"], cwd=workspace, capture_output=True, text=True)
    if r.returncode != 0: return False, r.stderr + r.stdout, 1
    return True, "Pass", 1


VERIFIERS = {
    "task_01_typed_function": {
        "howlframe": run_howlframe_task01, "go": run_go_generic_test, "python": run_python_generic_test, "node": run_node_generic_test
    },
    "task_02_cli_file_processing": {
        "howlframe": run_howlframe_task02, "go": "unsupported", "python": "unsupported", "node": "unsupported" # Complex custom verifier needed, skip for now?
    },
    "task_03_http_json_endpoint": {
        "howlframe": run_howlframe_task03, "go": "unsupported", "python": "unsupported", "node": "unsupported"
    },
    "task_04_multi_module_cli": {
        "howlframe": run_howlframe_task04, "go": "unsupported", "python": "unsupported", "node": "unsupported"
    },
    "task_05_list_dict_transform": {
        "howlframe": "unsupported", "go": run_go_generic_test, "python": run_python_generic_test, "node": run_node_generic_test
    },
    "task_06_error_handling": {
        "howlframe": run_howlframe_task06, "go": run_go_generic_test, "python": run_python_generic_test, "node": run_node_generic_test
    },
    "task_07_native_store": {
        "howlframe": run_howlframe_task07, "go": "unsupported", "python": "unsupported", "node": "unsupported"
    },
    "task_08_capability_restricted": {
        "howlframe": run_howlframe_task08, "go": "unsupported", "python": "unsupported", "node": "unsupported"
    },
}

def main():
    parser = argparse.ArgumentParser(description="Benchmark v2 Harness")
    parser.add_argument("--task", required=True)
    parser.add_argument("--language", required=True)
    parser.add_argument("--sources", nargs="+", required=True, help="Path to generated source files/dirs in chronological order (attempt 1, attempt 2, ...)")
    parser.add_argument("--model", required=True, help="Model used")
    parser.add_argument("--track", default="A", choices=["A", "B"])
    parser.add_argument("--out", required=True, help="Path to output JSON result")
    args = parser.parse_args()

    if args.task not in VERIFIERS or args.language not in VERIFIERS[args.task]:
        verifier = "unsupported"
    else:
        verifier = VERIFIERS[args.task][args.language]

    if verifier == "unsupported":
        print(f"Skipping verification: {args.task} is explicitly unsupported for {args.language}")
        result_json = {
            "task": args.task, "language": args.language, "track": args.track, "model": args.model,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "notes": "unsupported",
            "metrics": {
                "first_pass_success": False, "eventual_success": False, "repair_attempts": 0,
                "total_generated_tokens": None, "final_source_tokens": 0, "elapsed_time_seconds": 0.0,
                "verification_commands_run": 0, "code_churn_lines": None, "verification_result": "error",
                "runtime_backend": args.language, "capability_denial_behavior": None, "deterministic_repeatability": None
            }
        }
        os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
        with open(args.out, "w") as f: json.dump(result_json, f, indent=2)
        return

    first_pass_success = None
    eventual_success = False
    repair_attempts = 0
    verification_commands_run = 0
    elapsed_time = 0.0
    final_source_tokens = 0
    token_method = ""
    verification_result = "fail"
    capability_behavior = None
    msg = ""

    for i, source in enumerate(args.sources):
        start_time = time.time()
        try:
            with tempfile.TemporaryDirectory() as workspace:
                success, msg, cmds = verifier(workspace, os.path.abspath(source))
        except Exception as e:
            success, msg, cmds = False, str(e), 1
        elapsed_time += time.time() - start_time
        verification_commands_run += cmds

        if i == 0:
            first_pass_success = success
        else:
            repair_attempts += 1
            
        if success:
            eventual_success = True
            verification_result = "pass"
            
            source_content = ""
            if os.path.isdir(source):
                for f in os.listdir(source):
                    if os.path.isfile(os.path.join(source, f)) and any(f.endswith(ext) for ext in [".howl", ".py", ".go", ".js", ".ts", ".txt"]):
                        with open(os.path.join(source, f), "r", encoding="utf-8") as ff: source_content += ff.read()
            else:
                with open(source, "r", encoding="utf-8") as ff: source_content = ff.read()
            
            final_source_tokens, token_method = count_tokens(source_content)
            
            if args.task == "task_08_capability_restricted":
                capability_behavior = "Gracefully caught Access Denied"
            break
        else:
            print(f"Attempt {i+1} failed: {msg}")

    if not eventual_success:
        if args.task == "task_08_capability_restricted":
            capability_behavior = "Failed to catch or output Access Denied"

    result = {
        "task": args.task,
        "language": args.language,
        "track": args.track,
        "model": args.model,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "notes": f"Tokens counted using {token_method}",
        "metrics": {
            "first_pass_success": first_pass_success,
            "eventual_success": eventual_success,
            "repair_attempts": repair_attempts,
            "total_generated_tokens": None,
            "final_source_tokens": final_source_tokens,
            "elapsed_time_seconds": round(elapsed_time, 2),
            "verification_commands_run": verification_commands_run,
            "code_churn_lines": None,
            "verification_result": verification_result,
            "runtime_backend": "HowlFrame VM" if args.language == "howlframe" else args.language,
            "capability_denial_behavior": capability_behavior,
            "deterministic_repeatability": None
        }
    }

    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out, "w") as f:
        json.dump(result, f, indent=2)
    print(f"Result written to {args.out}")

if __name__ == "__main__":
    main()
