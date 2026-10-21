"""Reproduce a 'confirmed deletion lost' failure through the real run loop:
the first records() call (the post-kill durable check) re-adds a confirmed id."""
import sys, importlib.util
path, art = sys.argv[1], sys.argv[2]
spec = importlib.util.spec_from_file_location('storm', path); k = importlib.util.module_from_spec(spec); spec.loader.exec_module(k)
orig, calls = k.records, [0]
def records(state):
    calls[0] += 1
    r = orig(state)
    if calls[0] == 1:
        r['t1'] = {'id': 't1', 'status': 'open'}  # pretend confirmed t1 survived
    return r
k.records = records
sys.argv = [path, '--runs', '1', '--artifacts', art]
try:
    k.main()
except BaseException as e:
    print('main ended:', type(e).__name__, e)
