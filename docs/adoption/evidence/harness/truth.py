# Replays HowlBoard backend/ledger_import_test.go TestLedgerImportTruth fixture and store assertions against a tool.
import json, subprocess, sys, tempfile, os
BIN, ART, MODE = sys.argv[1:4]
stamp="2026-10-12T15:21:01.772056+00:00"; manual="manual coordinator projection"
E=[("repair","verification_completed","passed",stamp,{"scope":"actual check"}),
   ("repair","verification_completed","claimed",None,{"scope":"agent claim","exit_code":0}),
   ("repair","verification_completed","failed",None,{"scope":"failed check","exit_code":7}),
   ("repair","task_completed","accepted",None,None),("active","implementation_started","running",stamp,None),
   ("release","human_boundary_triggered","owner_action_required",None,None),("no-summary","verification_completed","passed",None,"DROP"),
   ("non-verification","task_completed","accepted",None,{"scope":"summary of execution"}),("empty-summary","verification_completed","passed",None,{})]
lines=[]
for i,(t,a,r,ts,vs) in enumerate(E):
    e={"schema":"ai.evidence_entry/v1","entry_id":f"e{i}","task_id":t,"agent_id":"test_coordinator","action":a,"result":r,
       "artifact":"local-proof.log","metadata":{"recording_mode":manual},"verification_summary":vs}
    if ts: e["timestamp"]=ts
    if vs=="DROP": del e["verification_summary"]
    lines.append(json.dumps(e))
led=tempfile.mktemp(suffix=".jsonl"); open(led,"w").write("\n".join(lines)+"\n")
d=tempfile.mkdtemp()
if MODE=="incumbent":
    subprocess.run([BIN,"-run-bc","-allow-caps","filesystem,database",ART,led,"20"],cwd=d,check=True,capture_output=True)
else:
    subprocess.run([BIN,"run","--allow-caps",f"filesystem:read={led},filesystem:read={d},filesystem:write={d},database",ART,led,d,"20"],check=True,capture_output=True)
s=json.load(open(os.path.join(d,"howlboard_missions.json")))
fails=[]
c=s["mission:repair"]["verification"]
if len(c)!=3: fails.append(f"repair has {len(c)} checks, want 3")
else:
    for i,st in enumerate(["passed","claimed","failed"]):
        if c[i]["status"]!=st: fails.append(f"status{i}")
        if not isinstance(c[i].get("summary"),dict): fails.append(f"check{i}: original summary not retained")
    if "exit_code" in c[0]: fails.append(f"missing exit code fabricated (exit_code={c[0]['exit_code']})")
    if c[1].get("exit_code")!=0 or c[2].get("exit_code")!=7: fails.append(f"explicit exit codes not preserved ({c[1].get('exit_code')},{c[2].get('exit_code')})")
for t in ["active","release","no-summary","non-verification"]:
    if s["mission:"+t]["verification"]: fails.append(f"{t}: fabricated verification")
a=s["mission:active"]
if a.get("recording_mode")!=manual or a.get("ledger_recorded_at")!=stamp: fails.append("ledger provenance/time not retained")
if a["execution"].get("start_recorded") is not True: fails.append("observed implementation start lost")
if s["mission:release"]["state"]!="AWAITING_APPROVAL" or a["state"]!="EXECUTING": fails.append("states drifted")
if a["authority"]["decision"]!="REQUIRE_APPROVAL": fails.append("approval invented")
print(MODE, "PASS" if not fails else "FAIL", fails)
