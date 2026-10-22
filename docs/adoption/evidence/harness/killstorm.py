"""SIGKILL the importer at spread-out times over a prior import; classify the stores left behind."""
import json, os, random, shutil, subprocess, sys, tempfile, time, collections
BIN, ART, LEDGER, PRIOR_LEDGER, ROUNDS = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], int(sys.argv[5])
STORES = ("howlboard_missions.json", "howlboard_timeline.json")
flags = ["-run-bc", "-allow-caps", "filesystem,database", "-max-instructions", "200000000", "-max-memory-bytes", "268435456", ART]
prior = tempfile.mkdtemp(); subprocess.run([BIN]+flags+[PRIOR_LEDGER, "10"], cwd=prior, check=True, capture_output=True)
t0 = time.time(); full = tempfile.mkdtemp(); subprocess.run([BIN]+flags+[LEDGER, "5000"], cwd=full, check=True, capture_output=True); dur = time.time()-t0
ref = {n: open(os.path.join(full, n)).read() for n in STORES}; pri = {n: open(os.path.join(prior, n)).read() for n in STORES}
random.seed(7); outcome = collections.Counter(); torn = 0
for i in range(ROUNDS):
    d = tempfile.mkdtemp()
    for n in STORES: shutil.copy(os.path.join(prior, n), d)
    p = subprocess.Popen([BIN]+flags+[LEDGER, "5000"], cwd=d, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(random.uniform(0, dur*1.05)); p.kill(); p.wait()
    state = []
    for n in STORES:
        txt = open(os.path.join(d, n)).read()
        try: json.loads(txt)
        except Exception: torn += 1; state.append("TORN"); continue
        state.append("prior" if txt == pri[n] else "complete" if txt == ref[n] else "partial")
    outcome[tuple(state)] += 1
    leftovers = [f for f in os.listdir(d) if f not in STORES]
    if leftovers: outcome[("leftover_files",)] += 1
print(json.dumps({"rounds": ROUNDS, "full_run_seconds": round(dur, 2), "torn_store_files": torn,
                  "outcomes(missions,timeline)": {",".join(k): v for k, v in outcome.items()}}))
