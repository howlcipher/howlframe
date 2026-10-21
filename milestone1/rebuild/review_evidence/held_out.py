"""Independent evaluator probes, frozen contracts only, no implementation imports."""
import json
from pathlib import Path
import subprocess
import tempfile
import time

BIN = '/tmp/m1-independent-hf'
FLAGS = ['--max-instructions', '4000000000', '--max-memory-bytes', '4294967296', '--max-call-depth', '10000']

def invoke(tool, args, text='', grants=''):
    command = [BIN, 'run', *FLAGS]
    if grants:
        command += ['--allow-caps', grants]
    begin = time.monotonic()
    result = subprocess.run(command + ['/tmp/m1-independent-' + tool + '.hfbc', *args], input=text, text=True, capture_output=True, timeout=120)
    return {'code': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr, 'seconds': round(time.monotonic()-begin, 3)}

results = {}
# Smallest near-int64-boundary count using a permitted 12-digit amount.
# Exact income is 9,223,399,999,999,907,766 cents, greater than int64 max.
n = 92234
source = 'date,description,amount\n' + '2026-01-01,x,999999999999.99\n' * n
result = invoke('ledgerlens', ['summary', '--format', 'json', '-'], source)
expected = str((99999999999999 * n) // 100) + '.' + str((99999999999999 * n) % 100).zfill(2)
result['expected_income'] = expected
result['pass'] = result['code'] == 0 and json.loads(result['stdout'])['total']['income'] == expected
results['ledger_exact_aggregate'] = result
with tempfile.TemporaryDirectory(prefix='m1-independent-') as directory:
    root = Path(directory)
    # A missing writable history is pre-created, then report permission is denied.
    # R13 says denied output must create/modify no file.
    history = root / 'history.json'
    report = root / 'outside' / 'report.json'
    report.parent.mkdir()
    grants = f'filesystem:read={root},filesystem:write={history}'
    result = invoke('taskflow', ['run', '-f', '-', '--history', str(history), '--report', str(report)], 'task default\n', grants)
    result['history_created'] = history.exists()
    result['pass'] = result['code'] == 77 and not history.exists() and not report.exists()
    results['taskflow_multi_output_denial'] = result
# Unknown keys within JSON assertions are forbidden anywhere in config.
config = {'version':1, 'checks':[{'name':'a','url':'http://localhost','json_assert':[{'path':'a','equals':1,'zzz':2}]}]}
result = invoke('healthwatch', ['validate','-'], json.dumps(config))
result['pass'] = result['code'] == 65
results['healthwatch_unknown_assertion_key'] = result
# A variable declared later can be reached before its earlier cycle member.
result = invoke('taskflow', ['run','--dry-run','-f','-'], 'set A = ${B}\nset B = ${A}\ntask default\n  run /p ${B}\n')
result['pass'] = result['code'] == 65 and '-:1: variable cycle: A -> B -> A' in result['stderr']
results['taskflow_cycle_origin'] = result
print(json.dumps(results, indent=2))
