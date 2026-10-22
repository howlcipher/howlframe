import os
import sys
import time
import subprocess
import threading
import signal

HOWLFRAME = "/tmp/howlframe"
SH = os.path.realpath("/bin/sh")
LS = os.path.realpath("/bin/ls")
ECHO = os.path.realpath("/bin/echo")
PYTHON = os.path.realpath(sys.executable)

print(f"Canonical paths: SH={SH}, LS={LS}, ECHO={ECHO}, PYTHON={PYTHON}")

def compile_bc(src_path, bc_path):
    cmd = [HOWLFRAME, "-compile-bc", src_path, "-o", bc_path]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode != 0:
        raise RuntimeError(f"Compile failed: {res.stderr}\n{res.stdout}")

def run_bc(bc_path, caps, flags=None):
    cmd = [HOWLFRAME, "-run-bc", "-allow-caps", caps]
    if flags:
        cmd.extend(flags)
    cmd.append(bc_path)
    return subprocess.run(cmd, capture_output=True, text=True)

def test_2_1_early_close_stdout_stderr():
    print("=== Probe 2.1: Re-attack Defect 3 - closing stdout/stderr early ===")
    d = "/tmp/g2_p1"
    os.system(f"rm -rf {d}")
    os.makedirs(d, exist_ok=True)
    howl_src = os.path.join(d, "early_close.howl")
    bc_path = os.path.join(d, "early_close.hfbc")
    # A shell command that closes stdout and stderr, sleeps 0.6s, then exits with code 7
    with open(howl_src, "w") as f:
        f.write(f'''
(cli_app
  (let (r (try_let (res (exec "{SH}" (list "-c" "exec >/dev/null 2>&1; sleep 0.6; exit 7")))
            (catch e e)
            res))
    (print r)))
''')
    compile_bc(howl_src, bc_path)
    t0 = time.time()
    res = run_bc(bc_path, f"process,process:exec={SH} **")
    elapsed = time.time() - t0
    print(f"  Execution took: {elapsed:.3f}s, stdout: {res.stdout.strip()!r}, stderr: {res.stderr.strip()!r}, rc: {res.returncode}")
    if elapsed < 0.5:
        print("FAIL: Command was terminated prematurely!")
        return False
    if "exit status 7" not in res.stdout and "exit status 7" not in res.stderr and "7" not in res.stdout:
        print(f"FAIL: Expected exit code 7, got stdout: {res.stdout}, stderr: {res.stderr}")
        return False
    print("  [PASS] Program closing stdout/stderr ran to completion with real exit status 7")
    return True

def test_2_2_fd_leaks_to_child():
    print("=== Probe 2.2: Check for fd leaks from runner or supervisor into child ===")
    d = "/tmp/g2_p2"
    os.system(f"rm -rf {d}")
    os.makedirs(d, exist_ok=True)
    howl_src = os.path.join(d, "fd_check.howl")
    bc_path = os.path.join(d, "fd_check.hfbc")
    with open(howl_src, "w") as f:
        f.write(f'''
(cli_app
  (let (r (exec "{LS}" (list "-l" "/proc/self/fd")))
    (print (bytes_to_string r))))
''')
    compile_bc(howl_src, bc_path)
    res = run_bc(bc_path, f"process,process:exec={LS} **")
    print("  Child /proc/self/fd listing:")
    lines = res.stdout.strip().splitlines()
    for line in lines:
        print("   ", line)
    
    # Python probe to check open fds > 2
    py_check_src = os.path.join(d, "py_fd.howl")
    py_check_bc = os.path.join(d, "py_fd.hfbc")
    with open(py_check_src, "w") as f:
        f.write(f'''
(cli_app
  (let (r (exec "{PYTHON}" (list "-c" "import os; print([fd for fd in range(3, 30) if os.path.exists(f'/proc/self/fd/{{fd}}') and fd != 3])")))
    (print (bytes_to_string r))))
''')
    compile_bc(py_check_src, py_check_bc)
    res_py = run_bc(py_check_bc, f"process,process:exec={PYTHON} **")
    print("  Extra fds open in child (excluding dirfd 3):", res_py.stdout.strip())
    if res_py.stdout.strip() != "[]":
        print(f"FAIL: Leaked fds into child: {res_py.stdout.strip()}")
        return False
    print("  [PASS] Zero fd leaks into child")
    return True

def test_2_3_deadline_during_exec():
    print("=== Probe 2.3: Deadline during exec kills entire tree (no survivors, no zombies) ===")
    d = "/tmp/g2_p3"
    os.system(f"rm -rf {d}")
    os.makedirs(d, exist_ok=True)
    howl_src = os.path.join(d, "deadline.howl")
    bc_path = os.path.join(d, "deadline.hfbc")
    marker = f"SURVIVOR_CHECK_{os.getpid()}"
    with open(howl_src, "w") as f:
        f.write(f'''
(cli_app
  (let (r (exec "{SH}" (list "-c" "sleep 100 & (setsid sleep 100 {marker} &) ; sleep 100")))
    (print "DONE")))
''')
    compile_bc(howl_src, bc_path)
    t0 = time.time()
    res = run_bc(bc_path, f"process,process:exec={SH} **", flags=["-deadline", "1s"])
    elapsed = time.time() - t0
    print(f"  Deadline triggered after {elapsed:.3f}s. Return code: {res.returncode}")
    time.sleep(0.5)
    # Check if marker process is still alive in /proc
    survivors = []
    for pid in os.listdir("/proc"):
        if pid.isdigit():
            try:
                cmdline = open(f"/proc/{pid}/cmdline", "rb").read().decode("latin1", errors="replace")
                if marker in cmdline:
                    survivors.append(pid)
            except Exception:
                pass
    if survivors:
        print(f"FAIL: Descendant survived deadline! PIDs: {survivors}")
        for p in survivors:
            try:
                os.kill(int(p), signal.SIGKILL)
            except Exception:
                pass
        return False
    print("  [PASS] All descendants terminated on deadline, zero survivors")
    return True

def test_2_4_output_limit_during_exec():
    print("=== Probe 2.4: Output limit during exec kills entire tree ===")
    d = "/tmp/g2_p4"
    os.system(f"rm -rf {d}")
    os.makedirs(d, exist_ok=True)
    howl_src = os.path.join(d, "output_cap.howl")
    bc_path = os.path.join(d, "output_cap.hfbc")
    marker = f"SURVIVOR_CAP_{os.getpid()}"
    with open(howl_src, "w") as f:
        f.write(f'''
(cli_app
  (let (r (exec "{SH}" (list "-c" "(setsid sleep 100 {marker} &) ; while true; do echo 'SPAM SPAM SPAM'; done")))
    (print "DONE")))
''')
    compile_bc(howl_src, bc_path)
    res = run_bc(bc_path, f"process,process:exec={SH} **", flags=["-max-exec-output-bytes", "1024"])
    print(f"  Output cap triggered. Return code: {res.returncode}")
    time.sleep(0.5)
    survivors = []
    for pid in os.listdir("/proc"):
        if pid.isdigit():
            try:
                cmdline = open(f"/proc/{pid}/cmdline", "rb").read().decode("latin1", errors="replace")
                if marker in cmdline:
                    survivors.append(pid)
            except Exception:
                pass
    if survivors:
        print(f"FAIL: Descendant survived output cap! PIDs: {survivors}")
        for p in survivors:
            try:
                os.kill(int(p), signal.SIGKILL)
            except Exception:
                pass
        return False
    print("  [PASS] All descendants terminated on output cap, zero survivors")
    return True

def test_2_5_concurrent_execs():
    print("=== Probe 2.5: High concurrency execs (20 threads) ===")
    d = "/tmp/g2_p5"
    os.system(f"rm -rf {d}")
    os.makedirs(d, exist_ok=True)
    howl_src = os.path.join(d, "conc.howl")
    bc_path = os.path.join(d, "conc.hfbc")
    with open(howl_src, "w") as f:
        f.write(f'''
(cli_app
  (let (r (exec "{ECHO}" (list "HELLO_CONCURRENCY")))
    (print (bytes_to_string r))))
''')
    compile_bc(howl_src, bc_path)

    results = []
    def worker():
        r = run_bc(bc_path, f"process,process:exec={ECHO} **")
        results.append(r.returncode == 0 and "HELLO_CONCURRENCY" in r.stdout)

    threads = [threading.Thread(target=worker) for _ in range(20)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    successes = sum(results)
    print(f"  20 concurrent execs: {successes}/20 succeeded")
    if successes != 20:
        print("FAIL: Some concurrent execs failed!")
        return False
    print("  [PASS] All concurrent execs succeeded without race/hang")
    return True

def main():
    ok1 = test_2_1_early_close_stdout_stderr()
    ok2 = test_2_2_fd_leaks_to_child()
    ok3 = test_2_3_deadline_during_exec()
    ok4 = test_2_4_output_limit_during_exec()
    ok5 = test_2_5_concurrent_execs()
    if ok1 and ok2 and ok3 and ok4 and ok5:
        print("\nALL GATE 2 PROBES PASSED!")
        sys.exit(0)
    else:
        print("\nSOME GATE 2 PROBES FAILED!")
        sys.exit(1)

if __name__ == "__main__":
    main()
