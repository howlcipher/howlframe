import json,glob,os
rows=[]
for f in glob.glob(os.path.expanduser('~/.codex/sessions/**/*.jsonl'),recursive=True):
    last=None; calls=0; model=None
    for line in open(f,errors='ignore'):
        if '"token_count"' in line or '"function_call"' in line or '"turn_context"' in line:
            try: d=json.loads(line)
            except Exception: continue
            p=d.get('payload') or {}
            if p.get('type')=='token_count' and p.get('info'): last=p['info'].get('total_token_usage')
            if p.get('type') in ('function_call','custom_tool_call','local_shell_call'): calls+=1
            if d.get('type')=='turn_context': model=p.get('model') or model
    if last and 20<=calls<=200: rows.append((calls,last,model))
print('sessions',len(rows))
def pct(v,p): v=sorted(v); return v[min(len(v)-1,int(p*len(v)))]
for k in ('input_tokens','cached_input_tokens','output_tokens','reasoning_output_tokens'):
    v=[r[1].get(k,0) for r in rows]; print(f"{k:24} p50={pct(v,.5):>12,} p90={pct(v,.9):>12,} max={max(v):>12,}")
print('calls p50',pct([r[0] for r in rows],.5),'p90',pct([r[0] for r in rows],.9))
def cost(u):
    i=u.get('input_tokens',0); c=u.get('cached_input_tokens',0); o=u.get('output_tokens',0)
    return ((i-c)*2+c*0.10+o*10)/1e6
c=[cost(r[1]) for r in rows]; print('gpt-6.1-sol short-ctx-priced $/session p50 %.2f p90 %.2f max %.2f'%(pct(c,.5),pct(c,.9),max(c)))
c=[cost(r[1])/r[0]*150 for r in rows]; print('scaled to 150 calls p50 %.2f p90 %.2f max %.2f'%(pct(c,.5),pct(c,.9),max(c)))
from collections import Counter; print(Counter(r[2] for r in rows).most_common(5))
