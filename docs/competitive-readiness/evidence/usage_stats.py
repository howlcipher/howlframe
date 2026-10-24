import json, glob, os, statistics as st, sys
rows=[]
for f in glob.glob(os.path.expanduser('~/.claude/projects/*/*.jsonl')):
    per={}; turns=0; inp=cr=cw=out=0; maxctx=0; models=set()
    try:
        for line in open(f, errors='ignore'):
            if '"usage"' not in line: continue
            try: d=json.loads(line)
            except Exception: continue
            m=d.get('message') or {}
            u=m.get('usage'); mid=m.get('id')
            if not u or d.get('type')!='assistant': continue
            models.add(m.get('model'))
            per[mid]=tuple(max(x,y) for x,y in zip(per.get(mid,(0,0,0,0)),(u.get('input_tokens',0),u.get('cache_read_input_tokens',0),u.get('cache_creation_input_tokens',0),u.get('output_tokens',0))))
    except Exception: continue
    turns=len(per)
    for a,b,c,o in per.values():
        inp+=a; cr+=b; cw+=c; out+=o; maxctx=max(maxctx,a+b+c)
    if 20<=turns<=200 and any(x and 'opus' in x for x in models):
        rows.append((turns,inp,cr,cw,out,maxctx))
print('sessions',len(rows))
def pct(v,p): v=sorted(v); return v[min(len(v)-1,int(p*len(v)))]
for name,i in [('turns',0),('uncached_in',1),('cache_read',2),('cache_write',3),('output',4),('max_ctx',5)]:
    v=[r[i] for r in rows]; print(f"{name:12} p50={pct(v,.5):>12,} p90={pct(v,.9):>12,} max={max(v):>12,}")
per=[(r[2]+r[1]+r[3])/r[0] for r in rows]; print('avg ctx/turn p50', int(pct(per,.5)), 'p90', int(pct(per,.9)))
po=[r[4]/r[0] for r in rows]; print('out/turn p50', int(pct(po,.5)), 'p90', int(pct(po,.9)))
pw=[r[3]/r[0] for r in rows]; print('cache_write/turn p50', int(pct(pw,.5)), 'p90', int(pct(pw,.9)))
# Opus 5.5 cost per session
c=[(r[1]*4+r[2]*0.2+r[3]*5+r[4]*20)/1e6 for r in rows]; print('opus5.5-priced $/session p50 %.2f p90 %.2f max %.2f'%(pct(c,.5),pct(c,.9),max(c)))
c150=[(r[1]*4+r[2]*0.2+r[3]*5+r[4]*20)/1e6/r[0]*150 for r in rows]; print('scaled to 150 turns p50 %.2f p90 %.2f max %.2f'%(pct(c150,.5),pct(c150,.9),max(c150)))
