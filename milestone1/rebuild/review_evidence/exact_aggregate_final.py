"""Reviewer-owned valid aggregate boundary, original failure retained separately."""
import json
from pathlib import Path
import subprocess
import time

out={}
for kind,rows in [('positive',[99999999999999]*92234),('mixed_negative',[-99999999999999]*92234+[99999999999999])]:
    source='date,description,amount\n'+''.join('2026-01-01,x,'+('-' if c<0 else '')+str(abs(c)//100)+'.'+str(abs(c)%100).zfill(2)+'\n' for c in rows)
    begin=time.monotonic()
    p=subprocess.run(['/tmp/m1-independent-hf','run','--max-instructions','4000000000','--max-memory-bytes','1099511627776','--max-call-depth','10000','/tmp/m1-independent-ledgerlens.hfbc','summary','--format','json','-'],input=source,text=True,capture_output=True,timeout=240)
    def money(c):return ('-' if c<0 else '')+str(abs(c)//100)+'.'+str(abs(c)%100).zfill(2)
    expected=dict(income=money(sum(c for c in rows if c>0)),expenses=money(-sum(c for c in rows if c<0)),net=money(sum(rows)),count=len(rows))
    actual=json.loads(p.stdout)['total'] if p.returncode==0 else None
    out[kind]=dict(code=p.returncode,stderr=p.stderr,expected=expected,actual=actual,seconds=round(time.monotonic()-begin,3),passed=p.returncode==0 and not p.stderr and actual==expected)
    Path(__file__).with_suffix('.json').write_text(json.dumps(out,indent=2)+'\n')
    assert out[kind]['passed'],out[kind]
print(json.dumps(out,indent=2))
