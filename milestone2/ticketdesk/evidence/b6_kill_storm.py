#!/usr/bin/env python3
"""Independent B6 probe. Stdlib only; builds committed HEAD in a temporary archive.
Run from repository root: python3 milestone2/ticketdesk/evidence/b6_kill_storm.py
Each trial restores the same API-seeded 1500-ticket snapshot, kills an active
DELETE loop, and verifies recovery against the durable records/index, including
all surviving records and all subsequent deletes. No runner ceilings overridden.
"""
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


def require(ok, detail):
    if not ok:
        raise AssertionError(detail)


class Server:
    def __init__(self, root, state):
        self.state = state
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            self.port = sock.getsockname()[1]
        grants = f'network,database,environment,filesystem:read={state.parent},filesystem:write={state.parent}'
        env = dict(os.environ, TICKETDESK_ADMIN_TOKEN=TOKEN,
                   TICKETDESK_DATA=str(state), TICKETDESK_PORT=str(self.port))
        self.log = tempfile.TemporaryFile()
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
        raise AssertionError('startup timeout')

    def req(self, method, path, body=None):
        c = http.client.HTTPConnection('127.0.0.1', self.port, timeout=10)
        headers = {'Authorization': 'Bearer ' + TOKEN}
        data = None
        if body is not None:
            data = json.dumps(body)
            headers['Content-Type'] = 'application/json'
        try:
            c.request(method, path, data, headers)
            r = c.getresponse()
            return r.status, json.loads(r.read())
        finally:
            c.close()

    def stop(self, kill=False):
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
        s = Server(root, state)
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
        for run in range(1, 26):
            state.write_bytes(snapshot)
            s = Server(root, state)
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
            s.stop()
            try:
                require(not worker.is_alive() and not errors, f'delete worker errors: {errors}')
                durable = json.loads(state.read_text())
                recs = records(durable)
                require(not (set(confirmed) & set(recs)), 'confirmed deletion lost')
                dangling = indexed(durable) - set(recs)
                orphans = set(recs) - indexed(durable)
                orphan_runs += bool(orphans)
                s = Server(root, state)
                recs = verify(s, durable)
                total_gets += len(recs)
                for tid in recs:
                    s.expect('DELETE', '/v1/tickets/' + tid, 200)
                    s.expect('GET', '/v1/tickets/' + tid, 404)
                    s.expect('DELETE', '/v1/tickets/' + tid, 404)
                total_deleted += len(recs)
                require(s.walk(1) == [] and s.walk(100) == [], 'not empty after cleanup')
                print(f'run={run:02d} delay={delay:.4f}s confirmed={len(confirmed)} survivors={len(recs)} dangling={len(dangling)} unindexed={len(orphans)} PASS', flush=True)
            except Exception as e:
                failures += 1
                print(f'run={run:02d} FAIL {e}', flush=True)
            finally:
                s.stop()
        # Deterministic opposite window: remove index id while record survives.
        crafted = json.loads(snapshot)
        for k, v in crafted.items():
            if k.startswith('b:') and 2 in v['ids']:
                v['ids'].remove(2)
        state.write_text(json.dumps(crafted))
        s = Server(root, state)
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
        s = Server(root, state)
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
        print(f'SUMMARY runs=25 failures={failures} orphan_runs={orphan_runs} survivor_GETs={total_gets} survivor_DELETEs={total_deleted}; crafted probes=2 PASS', flush=True)
        require(failures == 0, 'kill storm failed')


if __name__ == '__main__':
    main()
