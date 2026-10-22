import json, subprocess, sys, tempfile, os
BIN, INC, PROJ, LEDGER = sys.argv[1:5]
di = tempfile.mkdtemp(); dp = tempfile.mkdtemp()
subprocess.run([BIN,"-run-bc","-allow-caps","filesystem,database","-max-instructions","200000000",INC,LEDGER,"2000"],cwd=di,check=True,capture_output=True)
subprocess.run([BIN,"run","--allow-caps",f"filesystem:read={LEDGER},filesystem:read={dp},filesystem:write={dp},database","--max-instructions","10000000",PROJ,LEDGER,dp,"1000"],check=True,capture_output=True)
L = lambda d,n: json.load(open(os.path.join(d,n)))
mi, mp = L(di,"howlboard_missions.json"), L(dp,"howlboard_missions.json")
ti, tp = L(di,"howlboard_timeline.json")["feed"]["events"], L(dp,"howlboard_timeline.json")["feed"]["events"]
print("mission key order equal:", list(mi)==list(mp))
only_i, only_p, diff = {}, {}, {}
for k in mi:
    a, b = mi[k], mp[k]
    for f in set(a)-set(b): only_i[f]=only_i.get(f,0)+1
    for f in set(b)-set(a): only_p[f]=only_p.get(f,0)+1
    for f in set(a)&set(b):
        if a[f]!=b[f]: diff[f]=diff.get(f,0)+1
print("fields only in incumbent missions:", only_i)
print("fields only in projector missions:", only_p)
print("shared fields that differ (missions affected):", diff)
print("timeline order (mission_id,action) equal:", [(e["mission_id"],e["action"]) for e in ti]==[(e["mission_id"],e["action"]) for e in tp])
print("incumbent timeline keys:", sorted(ti[0]))
print("projector timeline keys:", sorted(tp[0])[:40])
print("projector timeline timestamp values:", {e.get("timestamp") for e in tp})
vi=[c for m in mi.values() for c in m.get("verification",[])]; vp=[c for m in mp.values() for c in m.get("verification",[])]
print("verification checks incumbent/projector:", len(vi), len(vp))
if vp: print("projector check sample:", vp[0])
if vi: print("incumbent check sample keys:", sorted(vi[0]))
print("authority decisions incumbent:", sorted({m["authority"]["decision"] for m in mi.values()}), "projector:", sorted({m["authority"]["decision"] for m in mp.values()}))
print("states differ:", {k:(mi[k]["state"],mp[k]["state"]) for k in mi if mi[k]["state"]!=mp[k]["state"]})
