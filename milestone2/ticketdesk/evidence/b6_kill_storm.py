#!/usr/bin/env python3
"""Independent B6 probe. Stdlib only; builds committed HEAD in a temporary archive.
Run from repository root: python3 milestone2/ticketdesk/evidence/b6_kill_storm.py
Each trial restores the same API-seeded 1500-ticket snapshot, kills an active
DELETE loop, and verifies recovery against the durable records/index, including
all surviving records and all subsequent deletes. No runner ceilings overridden.

Round 5 instrumentation (diagnostics only; pass criteria unchanged): when a request
times out, or the delete worker does not finish, the script prints a DIAG block with
the in-flight request (method, path, ticket id, elapsed), the server process status
(alive/exit code, /proc state, CPU time, a short health probe), box load/memory, and
the server stdout/stderr tail (access log + fault lines).

Round 6 instrumentation (diagnostics only; pass criteria, seed, delays and the 10 s
request timeout unchanged):
  * every server's stdout/stderr is kept as <artifacts>/runNN_<phase>.log;
  * on a timeout, the live server gets SIGQUIT (Go goroutine dump into its log) before
    it is killed, and the state file plus any *.tmp-* files are copied to
    <artifacts>/runNN_failure/;
  * each run prints p99 and max request latency (storm and recovery phases).
Usage: b6_kill_storm.py [--runs N] [--artifacts DIR]   (defaults: 25 runs, $TMPDIR/b6-storm-<time>)
"""
import argparse
import shutil
import signal
import http.client
import json
import os
from pathlib import Path
import random
import socket
import subprocess
import tempfile
import threading
import time
import urllib.parse

TOKEN = 'independent-review-bootstrap-token'
TAIL_BYTES = 4096
REQUEST_TIMEOUT = 10  # frozen for Round 6


def box_status():
    out = []
    try:
        out.append('loadavg=' + ' '.join(f'{x:.2f}' for x in os.getloadavg()) + f' cpus={os.cpu_count()}')
    except OSError:
        pass
    try:
        mem = dict(l.split(':', 1) for l in Path('/proc/meminfo').read_text().splitlines() if ':' in l)
        out.append('mem_available=' + mem.get('MemAvailable', '?').strip())
    except OSError:
        pass
    try:
        psi = Path('/proc/pressure/io').read_text().splitlines()[0]
        out.append('io_pressure: ' + psi)
    except OSError:
        pass
    return '; '.join(out)


def require(ok, detail):
    if not ok:
        raise AssertionError(detail)


def pct(values, q):
    if not values:
        return 0.0
    v = sorted(values)
    return v[min(len(v) - 1, max(0, int(round(q * len(v) + 0.5)) - 1))]


class Server:
    def __init__(self, root, state, log_path):
        self.state = state
        self.log_path = log_path
        self.latencies = []
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            self.port = sock.getsockname()[1]
        grants = f'network,database,environment,filesystem:read={state.parent},filesystem:write={state.parent}'
        env = dict(os.environ, TICKETDESK_ADMIN_TOKEN=TOKEN,
                   TICKETDESK_DATA=str(state), TICKETDESK_PORT=str(self.port))
        self.log = open(log_path, 'w+b')
        self.inflight = None  # (method, path, start monotonic) of the request in progress
        self.last_done = None  # (method, path, status, elapsed) of the last completed request
        self.p = subprocess.Popen([str(root / 'howlframe'), 'run', '--allow-caps', grants,
                                   '--max-request-bytes', '1048576', '--access-log',
                                   str(root / 'out/ticketdesk.hfbc')], env=env,
                                  stdout=self.log, stderr=self.log)
        for _ in range(200):
            try:
                if self.req('GET', '/v1/health')[0] == 200:
                    return
            except OSError:
                pass
            require(self.p.poll() is None, 'server exited during startup')
            time.sleep(.025)
        self.diag('startup timeout')
        raise AssertionError('startup timeout')

    def req(self, method, path, body=None, timeout=REQUEST_TIMEOUT):
        c = http.client.HTTPConnection('127.0.0.1', self.port, timeout=timeout)
        headers = {'Authorization': 'Bearer ' + TOKEN}
        data = None
        if body is not None:
            data = json.dumps(body)
            headers['Content-Type'] = 'application/json'
        start = time.monotonic()
        self.inflight = (method, path, start)
        try:
            c.request(method, path, data, headers)
            r = c.getresponse()
            result = r.status, json.loads(r.read())
            elapsed = time.monotonic() - start
            self.latencies.append(elapsed)
            self.last_done = (method, path, result[0], elapsed)
            self.inflight = None
            return result
        finally:
            c.close()

    def latency(self):
        return f'n={len(self.latencies)} p99={pct(self.latencies, .99) * 1000:.1f}ms max={max(self.latencies, default=0) * 1000:.1f}ms'

    def save_state(self, dest):
        """Copy the state file and any *.tmp-* siblings (diagnostic only)."""
        dest.mkdir(parents=True, exist_ok=True)
        saved = []
        for f in sorted(self.state.parent.iterdir()):
            if f == self.state or '.tmp-' in f.name:
                shutil.copy2(f, dest / f.name)
                saved.append(f'{f.name}({f.stat().st_size}B)')
        return saved

    def diag(self, reason, dump_dir=None):
        """Print a diagnostic block. Never changes the verdict of a run."""
        lines = [f'DIAG reason={reason}']
        inflight = self.inflight
        if inflight:
            method, path, start = inflight
            tid = path.rsplit('/', 1)[-1] if path.startswith('/v1/tickets/') else '-'
            lines.append(f'DIAG inflight method={method} path={path} ticket={tid} elapsed={time.monotonic() - start:.3f}s')
        else:
            lines.append('DIAG inflight none')
        if self.last_done:
            m, pth, st, el = self.last_done
            lines.append(f'DIAG last_completed method={m} path={pth} status={st} elapsed={el:.3f}s')
        rc = self.p.poll()
        if rc is None:
            info = f'DIAG server pid={self.p.pid} alive'
            try:
                stat = Path(f'/proc/{self.p.pid}/stat').read_text().rsplit(')', 1)[1].split()
                tick = os.sysconf('SC_CLK_TCK')
                info += f' state={stat[0]} utime={int(stat[11]) / tick:.2f}s stime={int(stat[12]) / tick:.2f}s threads={stat[17]}'
            except (OSError, IndexError, ValueError):
                pass
            lines.append(info)
            saved = self.inflight
            try:
                t0 = time.monotonic()
                st = self.req('GET', '/v1/health', timeout=3)[0]
                lines.append(f'DIAG health_probe status={st} elapsed={time.monotonic() - t0:.3f}s')
            except Exception as e:  # diagnostic only
                lines.append(f'DIAG health_probe error={type(e).__name__}: {e}')
            self.inflight = saved
            # Goroutine dump: the Go runtime writes all stacks to stderr on SIGQUIT and exits.
            try:
                self.p.send_signal(signal.SIGQUIT)
                self.p.wait(5)
                lines.append(f'DIAG sigquit sent; server exited code={self.p.returncode}; goroutine dump in {self.log_path.name}')
            except Exception as e:  # diagnostic only
                lines.append(f'DIAG sigquit error={type(e).__name__}: {e}')
        else:
            lines.append(f'DIAG server pid={self.p.pid} exited code={rc}')
        lines.append('DIAG box ' + box_status())
        if dump_dir is not None:
            try:
                lines.append('DIAG saved state files: ' + ', '.join(self.save_state(dump_dir)) + f' -> {dump_dir.name}/')
            except OSError as e:
                lines.append(f'DIAG state save error: {e}')
        try:
            self.log.flush()
            size = self.log.seek(0, os.SEEK_END)
            self.log.seek(max(0, size - TAIL_BYTES))
            tail = self.log.read().decode('utf-8', 'replace')
            self.log.seek(0, os.SEEK_END)
            lines.append(f'DIAG server_output_tail ({min(size, TAIL_BYTES)} of {size} bytes):')
            lines.extend('DIAG | ' + l for l in tail.splitlines()[-40:])
        except (OSError, ValueError) as e:
            lines.append(f'DIAG server_output_tail unavailable: {e}')
        print('\n'.join(lines), flush=True)

    def stop(self, kill=False):
        if self.log.closed:
            return
        if self.p.poll() is None:
            self.p.kill() if kill else self.p.terminate()
        self.p.wait(10)
        self.log.close()

    def expect(self, method, path, status, body=None):
        got, result = self.req(method, path, body)
        require(got == status, f'{method} {path}: expected {status}, got {got}: {result}')
        return result

    def walk(self, limit, status=None):
        seen, cursors, cursor = [], set(), None
        while True:
            query = {'limit': limit}
            if status:
                query['status'] = status
            if cursor:
                query['cursor'] = cursor
            page = self.expect('GET', '/v1/tickets?' + urllib.parse.urlencode(query), 200)
            seen.extend(t['id'] for t in page['items'])
            cursor = page['next_cursor']
            if not cursor:
                break
            require(cursor not in cursors, 'cursor cycle')
            cursors.add(cursor)
        require(len(seen) == len(set(seen)), 'duplicate ids')
        return seen


def records(state):
    return {v['id']: v for k, v in state.items() if k.startswith('t:')}


def indexed(state):
    return {'t' + str(n) for k, v in state.items() if k.startswith('b:') for n in v['ids']}


def verify(s, state):
    recs = records(state)
    expected = sorted(set(recs) & indexed(state), key=lambda x: int(x[1:]))
    for limit in (1, 100):
        require(s.walk(limit) == expected, f'limit={limit} survivor mismatch')
    require(s.walk(7, 'done') == [x for x in expected if recs[x]['status'] == 'done'], 'filter mismatch')
    # Every listed id and every unindexed-but-readable survivor must GET 200.
    for tid in recs:
        require(s.expect('GET', '/v1/tickets/' + tid, 200)['id'] == tid, 'GET identity')
    return recs


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--runs', type=int, default=25)
    ap.add_argument('--artifacts', type=Path,
                    default=Path(tempfile.gettempdir()) / time.strftime('b6-storm-%Y%m%d-%H%M%S'))
    args = ap.parse_args()
    art = args.artifacts.resolve()
    art.mkdir(parents=True, exist_ok=True)
    print(f'ARTIFACTS {art}', flush=True)
    rng = random.Random(20261019)
    failures = 0
    with tempfile.TemporaryDirectory(prefix='b6-independent-') as tmp:
        root = Path(tmp)
        archive = subprocess.run(['git', 'archive', 'HEAD'], check=True, stdout=subprocess.PIPE).stdout
        subprocess.run(['tar', '-x', '-C', str(root)], input=archive, check=True)
        subprocess.run(['go', 'build', '-o', str(root / 'howlframe'), '.'], cwd=root, check=True)
        (root / 'out').mkdir()
        subprocess.run([str(root / 'howlframe'), 'build', str(root / 'milestone2/ticketdesk/src/ticketdesk.howl'),
                        '-o', str(root / 'out/ticketdesk.hfbc')], check=True, stdout=subprocess.DEVNULL)
        state = root / 'out/state.json'
        s = Server(root, state, art / 'seed.log')
        try:
            for i in range(1500):
                t = s.expect('POST', '/v1/tickets', 201, {'title': f'independent {i}', 'body': 'x' * 100})
                if i % 5 == 0:
                    s.expect('PATCH', '/v1/tickets/' + t['id'], 200, {'status': 'done'})
        finally:
            s.stop()
        snapshot = state.read_bytes()
        print(f'BUILD PASS; API seed=1500; bytes={len(snapshot)}; default runner ceilings', flush=True)
        total_deleted = total_gets = orphan_runs = 0
        for run in range(1, args.runs + 1):
            state.write_bytes(snapshot)
            s = Server(root, state, art / f'run{run:02d}_storm.log')
            confirmed, errors = [], []
            def deleting():
                try:
                    for n in range(1, 1501):
                        s.expect('DELETE', f'/v1/tickets/t{n}', 200)
                        confirmed.append(f't{n}')
                except (OSError, http.client.HTTPException):
                    pass  # expected connection loss at SIGKILL
                except Exception as e:
                    errors.append(str(e))
            worker = threading.Thread(target=deleting)
            worker.start()
            delay = rng.uniform(.005, .8)
            time.sleep(delay)
            s.p.kill()
            worker.join(15)
            if worker.is_alive():
                s.diag('delete worker still running 15s after SIGKILL', art / f'run{run:02d}_failure')
            storm_lat = s.latency()
            s.stop()
            tmps = sorted(f.name for f in state.parent.iterdir() if '.tmp-' in f.name)
            if tmps:
                print(f'run={run:02d} tmp files after kill: {tmps}', flush=True)
            try:
                require(not worker.is_alive() and not errors, f'delete worker errors: {errors}')
                durable = json.loads(state.read_text())
                recs = records(durable)
                require(not (set(confirmed) & set(recs)), 'confirmed deletion lost')
                dangling = indexed(durable) - set(recs)
                orphans = set(recs) - indexed(durable)
                orphan_runs += bool(orphans)
                s = Server(root, state, art / f'run{run:02d}_recovery.log')
                recs = verify(s, durable)
                total_gets += len(recs)
                for tid in recs:
                    s.expect('DELETE', '/v1/tickets/' + tid, 200)
                    s.expect('GET', '/v1/tickets/' + tid, 404)
                    s.expect('DELETE', '/v1/tickets/' + tid, 404)
                total_deleted += len(recs)
                require(s.walk(1) == [] and s.walk(100) == [], 'not empty after cleanup')
                print(f'run={run:02d} delay={delay:.4f}s confirmed={len(confirmed)} survivors={len(recs)} dangling={len(dangling)} unindexed={len(orphans)} storm[{storm_lat}] recovery[{s.latency()}] PASS', flush=True)
            except Exception as e:
                failures += 1
                if isinstance(e, (TimeoutError, socket.timeout)) and not s.log.closed:
                    s.diag(f'run={run:02d} request timeout', art / f'run{run:02d}_failure')
                if not s.log.closed:
                    print(f'run={run:02d} latency[{s.latency()}]', flush=True)
                print(f'run={run:02d} FAIL {type(e).__name__}: {e}', flush=True)
            finally:
                s.stop()
        # Deterministic opposite window: remove index id while record survives.
        crafted = json.loads(snapshot)
        for k, v in crafted.items():
            if k.startswith('b:') and 2 in v['ids']:
                v['ids'].remove(2)
        state.write_text(json.dumps(crafted))
        s = Server(root, state, art / 'crafted_opposite.log')
        try:
            verify(s, crafted)
            s.expect('GET', '/v1/tickets/t2', 200)
            s.expect('DELETE', '/v1/tickets/t2', 200)
            s.expect('GET', '/v1/tickets/t2', 404)
            print('opposite-window: unindexed t2 omitted; GET 200, DELETE 200, GET 404; all walks PASS', flush=True)
        finally:
            s.stop()
        # Deterministic original B6 window, with missing records across buckets.
        crafted = json.loads(snapshot)
        for n in (1, 2, 64, 65, 1500):
            del crafted[f't:{n:010d}']
        state.write_text(json.dumps(crafted))
        s = Server(root, state, art / 'crafted_dangling.log')
        try:
            verify(s, crafted)
            for n in (1, 2, 64, 65, 1500):
                s.expect('DELETE', f'/v1/tickets/t{n}', 404)
                s.expect('GET', f'/v1/tickets/t{n}', 404)
            t = s.expect('POST', '/v1/tickets', 201, {'title': 'after dangling'})
            s.expect('DELETE', '/v1/tickets/' + t['id'], 200)
            verify(s, json.loads(state.read_text()))
            require(len(indexed(json.loads(state.read_text())) - set(records(json.loads(state.read_text())))) == 5,
                    'unexpected dangling repair behavior')
            print('dangling-state: 5 ids skipped; walks 200; GET/DELETE 404; create/delete succeeds; stale entries persist harmlessly', flush=True)
        finally:
            s.stop()
        print(f'SUMMARY runs={args.runs} failures={failures} orphan_runs={orphan_runs} survivor_GETs={total_gets} survivor_DELETEs={total_deleted}; crafted probes=2 PASS', flush=True)
        require(failures == 0, 'kill storm failed')


if __name__ == '__main__':
    main()
