import os
import sys
import shutil
import tempfile
import subprocess
import threading
import time

HOWLFRAME = "/tmp/howlframe"

def compile_bc(src_path, bc_path):
    cmd = [HOWLFRAME, "-compile-bc", "-o", bc_path, src_path]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode != 0:
        raise RuntimeError(f"Compile failed: {res.stderr}\n{res.stdout}")

def run_bc(bc_path, caps, stdin_data=""):
    cmd = [HOWLFRAME, "-run-bc", "-allow-caps", caps, bc_path]
    return subprocess.run(cmd, input=stdin_data, capture_output=True, text=True)

def run_interp(src_path, caps, stdin_data=""):
    cmd = [HOWLFRAME, "-run", "-allow-caps", caps, src_path]
    return subprocess.run(cmd, input=stdin_data, capture_output=True, text=True)

def test_1_1_static_single_file_symlink():
    print("=== Probe 1.1: Static relative symlink at single-file grant ===")
    d = tempfile.mkdtemp(prefix="g1_p1_")
    try:
        secret = os.path.join(d, "secret.txt")
        with open(secret, "w") as f:
            f.write("SECRET_PASS")
        granted = os.path.join(d, "granted.txt")
        os.symlink("secret.txt", granted)

        # Howl code to read
        read_howl = os.path.join(d, "reader.howl")
        with open(read_howl, "w") as f:
            f.write(f'''
(cli_app
  (let (c (try_let (b (read_file "{granted}"))
            (catch e "DENIED")
            (bytes_to_string b)))
    (print c)))
''')
        read_bc = os.path.join(d, "reader.hfbc")
        compile_bc(read_howl, read_bc)

        # Test bytecode read
        res_bc = run_bc(read_bc, f"filesystem:read={granted}")
        if "SECRET_PASS" in res_bc.stdout:
            print("FAIL: Bytecode read accessed secret file through symlink!")
            return False
        if "DENIED" not in res_bc.stdout and res_bc.returncode == 0:
            print(f"FAIL: Bytecode read did not report denial: stdout={res_bc.stdout!r}, stderr={res_bc.stderr!r}")
            return False
        print("  [PASS] Bytecode read refused symlink to sibling")

        # Test interpreter read
        res_interp = run_interp(read_howl, f"filesystem:read={granted}")
        if "SECRET_PASS" in res_interp.stdout:
            print("FAIL: Interpreter read accessed secret file through symlink!")
            return False
        print("  [PASS] Interpreter read refused symlink to sibling")

        # Howl code to write
        write_howl = os.path.join(d, "writer.howl")
        with open(write_howl, "w") as f:
            f.write(f'''
(cli_app
  (let (res (try_let (w (do (write_file "{granted}" "CORRUPTED") 1))
              (catch e "DENIED")
              "WRITTEN"))
    (print res)))
''')
        write_bc = os.path.join(d, "writer.hfbc")
        compile_bc(write_howl, write_bc)

        # Test bytecode write
        res_bc_w = run_bc(write_bc, f"filesystem:write={granted}")
        with open(secret, "r") as f:
            sec_content = f.read()
        if sec_content != "SECRET_PASS":
            print(f"FAIL: Bytecode write corrupted secret file! Content: {sec_content}")
            return False
        if "WRITTEN" in res_bc_w.stdout:
            print("FAIL: Bytecode write succeeded when symlink was present!")
            return False
        print("  [PASS] Bytecode write refused symlink to sibling")

        # Test interpreter write
        res_in_w = run_interp(write_howl, f"filesystem:write={granted}")
        with open(secret, "r") as f:
            sec_content = f.read()
        if sec_content != "SECRET_PASS":
            print(f"FAIL: Interpreter write corrupted secret file! Content: {sec_content}")
            return False
        print("  [PASS] Interpreter write refused symlink to sibling")

        return True
    finally:
        shutil.rmtree(d, ignore_errors=True)

def test_1_2_single_file_race():
    print("=== Probe 1.2: Single-file grant concurrent symlink race ===")
    d = tempfile.mkdtemp(prefix="g1_p2_")
    try:
        secret = os.path.join(d, "secret.txt")
        with open(secret, "w") as f:
            f.write("SECRET_VAULT")
        granted = os.path.join(d, "granted.txt")
        with open(granted, "w") as f:
            f.write("LEGIT")

        write_howl = os.path.join(d, "writer.howl")
        with open(write_howl, "w") as f:
            f.write(f'''
(cli_app
  (let (res (try_let (w (do (write_file "{granted}" "MODIFIED") 1))
              (catch e "DENIED")
              "WRITTEN"))
    (print res)))
''')
        write_bc = os.path.join(d, "writer.hfbc")
        compile_bc(write_howl, write_bc)

        read_howl = os.path.join(d, "reader.howl")
        with open(read_howl, "w") as f:
            f.write(f'''
(cli_app
  (let (c (try_let (b (read_file "{granted}"))
            (catch e "DENIED")
            (bytes_to_string b)))
    (print c)))
''')
        read_bc = os.path.join(d, "reader.hfbc")
        compile_bc(read_howl, read_bc)

        stop = False
        def flipper():
            tmp = os.path.join(d, "flipper_tmp")
            i = 0
            while not stop:
                i += 1
                try:
                    if os.path.exists(tmp) or os.path.islink(tmp):
                        os.unlink(tmp)
                    if i % 2 == 0:
                        os.symlink("secret.txt", tmp)
                    else:
                        with open(tmp, "w") as f:
                            f.write("LEGIT")
                    os.replace(tmp, granted)
                except Exception:
                    pass

        t = threading.Thread(target=flipper)
        t.start()

        escaped_reads = 0
        escaped_writes = 0
        total_runs = 50

        for _ in range(total_runs):
            # run read
            res = run_bc(read_bc, f"filesystem:read={granted}")
            if "SECRET_VAULT" in res.stdout:
                escaped_reads += 1
            # run write
            res_w = run_bc(write_bc, f"filesystem:write={granted}")
            with open(secret, "r") as f:
                if f.read() != "SECRET_VAULT":
                    escaped_writes += 1
                    with open(secret, "w") as f_fix:
                        f_fix.write("SECRET_VAULT")

        stop = True
        t.join()

        print(f"  Runs: {total_runs}, Escaped reads: {escaped_reads}, Escaped writes: {escaped_writes}")
        if escaped_reads > 0 or escaped_writes > 0:
            print("FAIL: Escapes detected during single-file race!")
            return False
        print("  [PASS] Zero escapes in single-file race")
        return True
    finally:
        shutil.rmtree(d, ignore_errors=True)

def test_1_3_store_single_file():
    print("=== Probe 1.3: Store open & sync under single-file grant ===")
    d = tempfile.mkdtemp(prefix="g1_p3_")
    try:
        secret = os.path.join(d, "secret.json")
        with open(secret, "w") as f:
            f.write('{"key": {"val": "TOP_SECRET"}}')
        store_file = os.path.join(d, "store.json")
        os.symlink("secret.json", store_file)

        # Try to open store when store_file is a symlink to secret.json
        howl_code = f'''
(cli_app
  (let (s (try_let (st (do (store_open kv "file://{store_file}") 1))
            (catch e "DENIED_STORE")
            "STORE_OPENED"))
    (print s)))
'''
        src = os.path.join(d, "s.howl")
        bc = os.path.join(d, "s.hfbc")
        with open(src, "w") as f:
            f.write(howl_code)
        compile_bc(src, bc)

        res = run_bc(bc, f"database,filesystem:read={store_file},filesystem:write={store_file}")
        if "STORE_OPENED" in res.stdout:
            print("FAIL: store_open followed symlink to secret.json!")
            return False
        print("  [PASS] store_open refused symlinked single-file target")
        return True
    finally:
        shutil.rmtree(d, ignore_errors=True)

def test_1_4_directory_symlink_escape():
    print("=== Probe 1.4: Directory grant symlink escape (mkdir & write) ===")
    d = tempfile.mkdtemp(prefix="g1_p4_")
    try:
        root_dir = os.path.join(d, "granted_root")
        os.makedirs(root_dir)
        outside_dir = os.path.join(d, "outside")
        os.makedirs(outside_dir)
        victim_file = os.path.join(outside_dir, "victim.txt")
        with open(victim_file, "w") as f:
            f.write("UNTOUCHED")

        # Symlink pointing outside inside the root
        escape_link = os.path.join(root_dir, "escape_link")
        os.symlink(outside_dir, escape_link)

        # Howl code attempting to write outside via symlink
        target_file = os.path.join(escape_link, "victim.txt")
        write_howl = os.path.join(d, "escape_writer.howl")
        with open(write_howl, "w") as f:
            f.write(f'''
(cli_app
  (let (res (try_let (w (do (write_file "{target_file}" "PWNED") 1))
              (catch e "DENIED")
              "WRITTEN"))
    (print res)))
''')
        write_bc = os.path.join(d, "escape_writer.hfbc")
        compile_bc(write_howl, write_bc)

        res_bc = run_bc(write_bc, f"filesystem:write={root_dir}")
        with open(victim_file, "r") as f:
            v_content = f.read()
        if v_content != "UNTOUCHED":
            print("FAIL: write_file escaped directory root through symlink!")
            return False
        if "WRITTEN" in res_bc.stdout:
            print("FAIL: write_file reported written across root boundary!")
            return False
        print("  [PASS] write_file refused escape across directory root")

        # Now test mkdir across root boundary
        target_dir = os.path.join(escape_link, "escaped_subdir")
        mkdir_howl = os.path.join(d, "escape_mkdir.howl")
        with open(mkdir_howl, "w") as f:
            f.write(f'''
(cli_app
  (let (res (try_let (w (do (mkdir "{target_dir}") 1))
              (catch e "DENIED")
              "CREATED"))
    (print res)))
''')
        mkdir_bc = os.path.join(d, "escape_mkdir.hfbc")
        compile_bc(mkdir_howl, mkdir_bc)

        res_mk = run_bc(mkdir_bc, f"filesystem:write={root_dir}")
        if os.path.exists(target_dir):
            print("FAIL: mkdir escaped directory root through symlink!")
            return False
        if "CREATED" in res_mk.stdout:
            print("FAIL: mkdir reported created across root boundary!")
            return False
        print("  [PASS] mkdir refused escape across directory root")

        return True
    finally:
        shutil.rmtree(d, ignore_errors=True)

def main():
    ok1 = test_1_1_static_single_file_symlink()
    ok2 = test_1_2_single_file_race()
    ok3 = test_1_3_store_single_file()
    ok4 = test_1_4_directory_symlink_escape()
    if ok1 and ok2 and ok3 and ok4:
        print("\nALL GATE 1 PROBES PASSED!")
        sys.exit(0)
    else:
        print("\nSOME GATE 1 PROBES FAILED!")
        sys.exit(1)

if __name__ == "__main__":
    main()
