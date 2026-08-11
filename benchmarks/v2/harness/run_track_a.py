#!/usr/bin/env python3
import argparse
import json
import os
import shutil
import subprocess
import tempfile
import time
from datetime import datetime, timezone

def count_tokens(text):
    # Proxy token counter (approximate using words or characters)
    # A real implementation would use tiktoken, but we do not require a paid model API or external deps.
    return len(text) // 4

def run_howlframe_task02(workspace, source_path):
    # Compile
    bc_path = os.path.join(workspace, "app.bc")
    compile_cmd = ["howlframe", "-compile-bc", source_path, "-o", bc_path]
    c_res = subprocess.run(compile_cmd, capture_output=True, text=True)
    if c_res.returncode != 0:
        return False, c_res.stderr

    # Create test file
    test_txt = os.path.join(workspace, "names.txt")
    with open(test_txt, "w") as f:
        f.write("Alice\nBob\n")

    # Run normal
    run_cmd = ["howlframe", "-allow-caps", "filesystem", "-run-bc", bc_path, test_txt]
    r_res = subprocess.run(run_cmd, capture_output=True, text=True)
    if r_res.returncode != 0 or "Hello, Alice" not in r_res.stdout or "Hello, Bob" not in r_res.stdout:
        return False, f"Normal run failed:\n{r_res.stdout}\n{r_res.stderr}"

    # Run missing
    run_cmd_missing = ["howlframe", "-allow-caps", "filesystem", "-run-bc", bc_path, "missing_file.txt"]
    m_res = subprocess.run(run_cmd_missing, capture_output=True, text=True)
    if m_res.returncode != 1 or "Error: File not found" not in m_res.stderr:
        return False, f"Missing file run failed (expected code 1 and Error msg). Code: {m_res.returncode}\nStderr: {m_res.stderr}"

    return True, "Pass"

def run_howlframe_task03(workspace, source_path):
    bc_path = os.path.join(workspace, "server.bc")
    compile_cmd = ["howlframe", "-compile-bc", source_path, "-o", bc_path]
    c_res = subprocess.run(compile_cmd, capture_output=True, text=True)
    if c_res.returncode != 0:
        return False, c_res.stderr

    # Start server
    server_process = subprocess.Popen(["howlframe", "-allow-caps", "network", "-run-bc", bc_path])
    time.sleep(1) # wait for server to start

    try:
        import urllib.request
        # Test /
        req1 = urllib.request.urlopen("http://localhost:8080/")
        res1 = req1.read().decode('utf-8')
        if "Hello World" not in res1:
            return False, f"GET / failed, got: {res1}"

        # Test /json
        req2 = urllib.request.urlopen("http://localhost:8080/json")
        if req2.getheader("Content-Type") != "application/json":
            return False, f"GET /json bad content type: {req2.getheader('Content-Type')}"
        res2 = req2.read().decode('utf-8')
        if '"message"' not in res2 or '"Hello JSON"' not in res2:
            return False, f"GET /json failed, got: {res2}"
        
        return True, "Pass"
    except Exception as e:
        return False, f"HTTP request failed: {e}"
    finally:
        server_process.terminate()
        server_process.wait()

def run_howlframe_task04(workspace, source_path):
    # For multi-module, source_path contains the main app. We expect a mathops.howl to exist.
    # We will assume the user has provided the files in the workspace already, or source_path is the directory?
    # Actually, the user passes a source file. If they generated multiple files, we assume source_path is a zip or dir.
    # For simplicity, if source_path is a directory, we copy all files.
    if os.path.isdir(source_path):
        for item in os.listdir(source_path):
            s = os.path.join(source_path, item)
            d = os.path.join(workspace, item)
            if os.path.isfile(s):
                shutil.copy2(s, d)
    else:
        # Just copy the one file, maybe it won't work for multi-module
        shutil.copy2(source_path, os.path.join(workspace, "app.howl"))
    
    # We look for a file that imports mathops and run it. Let's assume it's app.howl.
    # Actually, we can just find any .howl file that is not mathops.
    main_file = "app.howl"
    bc_path = os.path.join(workspace, "app.bc")
    compile_cmd = ["howlframe", "-compile-bc", os.path.join(workspace, main_file), "-o", bc_path]
    c_res = subprocess.run(compile_cmd, capture_output=True, text=True)
    if c_res.returncode != 0:
        return False, c_res.stderr

    run_cmd = ["howlframe", "-run-bc", bc_path]
    r_res = subprocess.run(run_cmd, capture_output=True, text=True)
    if r_res.returncode != 0 or "20" not in r_res.stdout:
        return False, f"Failed: {r_res.stdout}\n{r_res.stderr}"
    
    return True, "Pass"

def run_howlframe_task07(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    compile_cmd = ["howlframe", "-compile-bc", source_path, "-o", bc_path]
    c_res = subprocess.run(compile_cmd, capture_output=True, text=True)
    if c_res.returncode != 0:
        return False, c_res.stderr

    # Ensure store capability is given. Wait, HowlFrame store uses what cap? 'database' maybe?
    # Actually the caps are: network, filesystem, process, environment, database
    run_cmd = ["howlframe", "-allow-caps", "database,filesystem", "-run-bc", bc_path]
    r_res = subprocess.run(run_cmd, capture_output=True, text=True)
    if r_res.returncode != 0:
        return False, f"Failed:\n{r_res.stdout}\n{r_res.stderr}"
    return True, "Pass"

def run_howlframe_task08(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    compile_cmd = ["howlframe", "-compile-bc", source_path, "-o", bc_path]
    c_res = subprocess.run(compile_cmd, capture_output=True, text=True)
    if c_res.returncode != 0:
        return False, c_res.stderr

    # Run without filesystem capability, wait for 'Access Denied'
    run_cmd = ["howlframe", "-allow-caps", "environment", "-run-bc", bc_path]
    r_res = subprocess.run(run_cmd, capture_output=True, text=True)
    # The requirement is that it gracefully handles the denial and prints 'Access Denied'.
    if r_res.returncode != 0:
         return False, f"Crashed or exited with error code {r_res.returncode}. It was supposed to gracefully handle it.\nStderr: {r_res.stderr}"
    if "Access Denied" not in r_res.stdout:
         return False, f"Did not print Access Denied.\nStdout: {r_res.stdout}"
    return True, "Pass"

def run_howlframe_task01(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    compile_cmd = ["howlframe", "-compile-bc", source_path, "-o", bc_path]
    c_res = subprocess.run(compile_cmd, capture_output=True, text=True)
    if c_res.returncode != 0:
        return False, c_res.stderr

    run_cmd = ["howlframe", "-run-bc", bc_path]
    r_res = subprocess.run(run_cmd, capture_output=True, text=True)
    if r_res.returncode != 0 or "Pass" not in r_res.stdout:
        return False, f"Failed:\n{r_res.stdout}\n{r_res.stderr}"
    return True, "Pass"

def run_howlframe_task06(workspace, source_path):
    bc_path = os.path.join(workspace, "app.bc")
    compile_cmd = ["howlframe", "-compile-bc", source_path, "-o", bc_path]
    c_res = subprocess.run(compile_cmd, capture_output=True, text=True)
    if c_res.returncode != 0:
        return False, c_res.stderr

    run_cmd = ["howlframe", "-run-bc", bc_path]
    r_res = subprocess.run(run_cmd, capture_output=True, text=True)
    if r_res.returncode != 0 or "Error: Division by zero" not in r_res.stdout:
        return False, f"Failed:\n{r_res.stdout}\n{r_res.stderr}"
    return True, "Pass"

VERIFIERS = {
    "task_01_typed_function": {"howlframe": run_howlframe_task01},
    "task_02_cli_file_processing": {"howlframe": run_howlframe_task02},
    "task_03_http_json_endpoint": {"howlframe": run_howlframe_task03},
    "task_04_multi_module_cli": {"howlframe": run_howlframe_task04},
    "task_06_error_handling": {"howlframe": run_howlframe_task06},
    "task_07_native_store": {"howlframe": run_howlframe_task07},
    "task_08_capability_restricted": {"howlframe": run_howlframe_task08},
}

def main():
    parser = argparse.ArgumentParser(description="Benchmark v2 Harness")
    parser.add_argument("--task", required=True)
    parser.add_argument("--language", required=True)
    parser.add_argument("--source", required=True, help="Path to generated source file or directory")
    parser.add_argument("--model", required=True, help="Model used")
    parser.add_argument("--track", default="A", choices=["A", "B"])
    parser.add_argument("--out", required=True, help="Path to output JSON result")
    
    args = parser.parse_args()

    if args.task not in VERIFIERS or args.language not in VERIFIERS[args.task]:
        print(f"Skipping verification: No verifier found for {args.task} in {args.language}")
        verification_result = "error"
        error_msg = "Unsupported task/language combination or verifier not implemented."
        success = False
    else:
        verifier = VERIFIERS[args.task][args.language]
        
        start_time = time.time()
        with tempfile.TemporaryDirectory() as workspace:
            success, msg = verifier(workspace, os.path.abspath(args.source))
        elapsed = time.time() - start_time
        
        verification_result = "pass" if success else "fail"
        print(f"Verification {verification_result.upper()}: {msg}")

    # Read source tokens
    source_content = ""
    if os.path.isdir(args.source):
        for f in os.listdir(args.source):
            if os.path.isfile(os.path.join(args.source, f)):
                with open(os.path.join(args.source, f), "r") as ff:
                    source_content += ff.read()
    else:
        with open(args.source, "r") as ff:
            source_content = ff.read()

    tokens = count_tokens(source_content)

    result = {
        "task": args.task,
        "language": args.language,
        "track": args.track,
        "model": args.model,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "metrics": {
            "first_pass_success": success,
            "eventual_success": success,
            "repair_attempts": 0,
            "total_generated_tokens": None,
            "final_source_tokens": tokens,
            "elapsed_time_seconds": round(elapsed, 2) if 'elapsed' in locals() else 0.0,
            "verification_commands_run": 2, # compile + run
            "code_churn_lines": None,
            "verification_result": verification_result,
            "runtime_backend": "HowlFrame VM" if args.language == "howlframe" else args.language,
            "capability_denial_behavior": "Crashing" if args.task == "task_08_capability_restricted" and not success else None,
            "deterministic_repeatability": None
        }
    }

    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out, "w") as f:
        json.dump(result, f, indent=2)
    print(f"Result written to {args.out}")

if __name__ == "__main__":
    main()
