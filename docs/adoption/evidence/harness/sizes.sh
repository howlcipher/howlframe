#!/usr/bin/env bash
# Old vs new HowlBoard importer at realistic slice sizes (records cloned from the
# sanitized fixture with fresh entry/task IDs; median ~855 bytes per record).
# usage: sizes.sh HOWLFRAME_BIN OLD.hfbc NEW.hfbc FIXTURE
set -u
bin=$1 old=$2 new=$3 fixture=$4 work=$(mktemp -d)
python3 -I - "$fixture" "$work" <<'PY'
import json, sys
rows = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
for n in (1000, 2000, 3000, 3500, 5000):
    with open(f"{sys.argv[2]}/syn{n}.jsonl", "w") as f:
        for i in range(n):
            r = dict(rows[i % len(rows)]); r["entry_id"] = f"syn-{i:06d}"; r["task_id"] = f"{r['task_id']}-{i // 40}"
            f.write(json.dumps(r) + "\n")
PY
for mem in default 268435456; do
  for n in 1000 2000 3000 3500 5000; do
    for v in old new; do
      art=$old; [[ $v == new ]] && art=$new
      flags=(-run-bc -allow-caps filesystem,database -max-instructions 200000000)
      [[ $mem != default ]] && flags+=(-max-memory-bytes "$mem")
      d=$(mktemp -d)
      out=$(cd "$d" && /usr/bin/time -f "%es rss=%MKB" "$bin" "${flags[@]}" "$art" "$work/syn$n.jsonl" 5000 2>&1 | tr '\n' ' ' | sed 's/{"phase".*"message":"\([^"]*\)".*}/RAW \1/' | cut -c1-120)
      echo "memory=$mem records=$n bytes=$(stat -c %s "$work/syn$n.jsonl") $v: $out"
    done
  done
done
