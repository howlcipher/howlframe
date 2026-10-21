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
  * on every failure (timeout, 'confirmed deletion lost', anything), the state file and
    any *.tmp-* files are copied to <artifacts>/runNN_failure/; a still-live server also
    gets SIGQUIT (goroutine dump) before stop(), which falls back to SIGKILL if needed;
  * a server that fails to start is killed before the error propagates;
  * --expect-head accepts only HEAD == pin, or one commit directly on the pin whose
    --no-renames numstat is exactly `1 1 milestone2/MILESTONE2_DECISION.md` and which
    only turns the placeholder Code-under-test line into exactly that line naming the pin
    (exact match, no substring); the pin commit must be a squash or direct commit, not a
    merge; SUMMARY carries CODE_PINNED;
  * before SIGQUIT, diag records /proc/PID/wchan, status and task/*/stat so a
    SIGSTOPped or fsync-stuck server still produces diagnostics;
  * each run prints p99 and max request latency (storm and recovery phases);
  * state.json.tmp-* leftovers from a previous kill are cleared (or timestamped aside)
    after every snapshot restore;
  * the script prints and enforces its own sha256 (SCRIPT_SHA256) and records it in
    the evidence SUMMARY line.
Round 7 additions (pre-registered in MILESTONE2_DECISION.md; seed, delays, the 10 s
per-socket-operation timeout and the 1500-ticket workload unchanged):
  * heartbeat sidecar: a separate process (CPU-pinned when possible) sleeps 50 ms in a
    loop and logs every gap > 250 ms with CLOCK_MONOTONIC/REALTIME/BOOTTIME deltas
    (<artifacts>/heartbeat.log), plus an alive line every 10 s;
  * noise sampler sidecar: once a second, /proc/stat cpu (steal, iowait), PSI cpu/io/memory,
    cgroup cpu.stat/memory.pressure/memory.events, diskstats for the state device and
    pgmajfault (<artifacts>/noise.jsonl); new /dev/kmsg records (<artifacts>/kmsg.log);
  * per-request client log for every request (<artifacts>/runNN_<phase>_requests.tsv);
  * in-flight watchdog: once a request is in flight 2 s, samples /proc/PID/task/*/
    {stat,wchan,schedstat,stack}, /proc/PID/status and diskstats every 500 ms, with deltas
    against the last sample before the request started (runNN_<phase>_watchdog.log);
  * TOTAL DEADLINE (criterion change): any request whose client elapsed time exceeds 10 s
    fails, even when a response arrives; a socket timeout is a deadline failure too;
  * the server runs with --timing-log and GODEBUG=gctrace=1 (stderr, kept in the run log);
  * pre-run idle gate (120 s) and in-run noise gates; INFRA-VOID on a breach; every failure
    is classified product/infra by fixed rules; a VERDICT line follows SUMMARY;
  * on a failure: Round 6 diagnostics plus schedstat/diskstats deltas since the request
    started, the watchdog samples, a heartbeat excerpt and a copy of the full server log.
Usage: b6_kill_storm.py [--runs N] [--artifacts DIR] [--expect-sha256 HEX] [--expect-head SHA]
                        [--self-test-sigstop] [--self-test-noise]
       (defaults: 25 runs, $TMPDIR/b6-storm-<time>)
"""
import argparse
import contextlib
import hashlib
import io
import shutil
import signal
import http.client
import json
import os
import re
import sys
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
REQUEST_TIMEOUT = 10  # per socket operation; unchanged since Round 4
TOTAL_DEADLINE = 10.0  # Round 7: total client elapsed per request; over it fails even with a reply
SCRIPT_PATH = Path(__file__).resolve()

# Round 7 noise instrumentation (pre-registered values; not command-line options).
HEARTBEAT_SLEEP = 0.05
HEARTBEAT_GAP_LOG = 0.25
HEARTBEAT_ALIVE_EVERY = 10.0
SAMPLER_EVERY = 1.0
WATCHDOG_AFTER = 2.0
WATCHDOG_EVERY = 0.5
IDLE_GATE_SECONDS = 120.0
IDLE_MAX_STEAL_PCT = 1.0
IDLE_MAX_PSI_CPU_SOME_AVG60 = 5.0
IDLE_MAX_PSI_IO_FULL_AVG60 = 2.0
IDLE_MAX_PSI_MEM_FULL_AVG60 = 0.0
IDLE_MAX_HEARTBEAT_GAP = 0.25
RUN_MAX_HEARTBEAT_GAP = 2.0
RUN_MAX_STEAL_PCT = 2.0
RUN_STEAL_WINDOW = 60.0
RUN_MAX_PSI_IO_FULL_AVG10 = 10.0
KMSG_PATTERNS = ('virtual machine fork', 'virtio_balloon')
KMSG_OBSERVE_SLACK = 1.5  # kmsg is polled once a second; an event is attributed up to 1.5 s late
HEARTBEAT_STALE = 30.0  # no heartbeat record for this long (alive or gap) = heartbeat lost
CLASSIFY_GAP_COVER = 0.8
CLASSIFY_GAP_MIN = 1.0


def script_sha256():
    return hashlib.sha256(SCRIPT_PATH.read_bytes()).hexdigest()


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


class DeadlineExceeded(AssertionError):
    """A request exceeded TOTAL_DEADLINE (Round 7), whether or not a reply arrived.
    Not an OSError, so the storm worker never mistakes it for the expected kill."""


def read_text(path):
    try:
        return Path(path).read_text()
    except OSError:
        return None


def device_of(path):
    st = os.stat(path)
    return f'{os.major(st.st_dev)}:{os.minor(st.st_dev)}'


def diskstats_line(dev):
    """The /proc/diskstats line for major:minor dev, or None (e.g. overlay/tmpfs)."""
    text = read_text('/proc/diskstats') or ''
    if not dev:
        return None
    major, minor = dev.split(':')
    for line in text.splitlines():
        f = line.split()
        if len(f) > 3 and f[0] == major and f[1] == minor:
            return ' '.join(f)
    return None


def proc_sample(pid, dev):
    """One watchdog sample of a server process. Diagnostic only."""
    sample = {'mono': time.monotonic(), 'tasks': {}, 'status': None, 'disk': diskstats_line(dev)}
    status = read_text(f'/proc/{pid}/status')
    if status is not None:
        sample['status'] = ' | '.join(l for l in status.splitlines() if l.startswith(
            ('State:', 'Threads:', 'voluntary_ctxt_switches:', 'nonvoluntary_ctxt_switches:')))
    try:
        tasks = sorted(Path(f'/proc/{pid}/task').iterdir(), key=lambda x: int(x.name))
    except OSError:
        tasks = []
    for task in tasks[:32]:
        stat = read_text(task / 'stat')
        sched = read_text(task / 'schedstat')
        entry = {'state': '?', 'wchan': (read_text(task / 'wchan') or '?').strip() or '?',
                 'oncpu_ns': None, 'wait_ns': None, 'slices': None, 'stack': None}
        if stat:
            try:
                entry['state'] = stat.rsplit(')', 1)[1].split()[0]
            except IndexError:
                pass
        if sched:
            try:
                a, b, c = sched.split()[:3]
                entry['oncpu_ns'], entry['wait_ns'], entry['slices'] = int(a), int(b), int(c)
            except ValueError:
                pass
        stack = read_text(task / 'stack')
        if stack:
            entry['stack'] = ' <- '.join(l.split(' ', 1)[-1] for l in stack.strip().splitlines()[:6])
        sample['tasks'][task.name] = entry
    return sample


def sample_delta_lines(base, now, prefix):
    """Per-thread schedstat deltas (on-CPU, run-queue wait) and diskstats between samples."""
    lines = [f'{prefix} window={now["mono"] - base["mono"]:.3f}s status: {now["status"]}']
    for tid, t in now['tasks'].items():
        b = base['tasks'].get(tid, {})
        def d(k):
            if t.get(k) is None or b.get(k) is None:
                return '?'
            return f'{(t[k] - b[k]) / 1e6:.1f}ms' if k != 'slices' else str(t[k] - b[k])
        line = f'{prefix} task {tid} state={t["state"]} wchan={t["wchan"]} oncpu+={d("oncpu_ns")} rqwait+={d("wait_ns")} slices+={d("slices")}'
        if t.get('stack'):
            line += f' stack={t["stack"]}'
        lines.append(line)
    if base.get('disk') and now.get('disk'):
        bf, nf = base['disk'].split(), now['disk'].split()
        try:
            # fields: reads(3) ... writes(7) ... in-flight(11) io_ticks_ms(12) weighted_ms(13)
            lines.append(f'{prefix} disk {nf[2]} reads+={int(nf[3]) - int(bf[3])} writes+={int(nf[7]) - int(bf[7])} '
                         f'inflight={nf[11]} io_ms+={int(nf[12]) - int(bf[12])} weighted_ms+={int(nf[13]) - int(bf[13])}')
        except (IndexError, ValueError):
            lines.append(f'{prefix} disk {now["disk"]}')
    else:
        lines.append(f'{prefix} disk unavailable for the state device')
    return lines


class Server:
    def __init__(self, root, state, log_path):
        self.state = state
        self.log_path = log_path
        self.latencies = []
        self.deadline = TOTAL_DEADLINE
        self.breach = None  # (method, path, start, end, error) of the first deadline breach
        self.seq = 0
        self.req_log = open(log_path.with_name(log_path.stem + '_requests.tsv'), 'w')
        self.req_log.write('seq\tmethod\tpath\tmono_start\tmono_end\treal_start\tstatus\telapsed_s\n')
        try:
            self.dev = device_of(state.parent)
        except OSError:
            self.dev = None
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            self.port = sock.getsockname()[1]
        grants = f'network,database,environment,filesystem:read={state.parent},filesystem:write={state.parent}'
        env = dict(os.environ, TICKETDESK_ADMIN_TOKEN=TOKEN,
                   TICKETDESK_DATA=str(state), TICKETDESK_PORT=str(self.port))
        self.log = open(log_path, 'w+b')
        self.inflight = None  # (method, path, start monotonic) of the request in progress
        self.last_done = None  # (method, path, status, elapsed) of the last completed request
        env['GODEBUG'] = 'gctrace=1'
        self.p = subprocess.Popen([str(root / 'howlframe'), 'run', '--allow-caps', grants,
                                   '--max-request-bytes', '1048576', '--access-log', '--timing-log',
                                   str(root / 'out/ticketdesk.hfbc')], env=env,
                                  stdout=self.log, stderr=self.log)
        self.start_watchdog()
        try:
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
        except BaseException:
            # A server that fails to start must never outlive this object: the caller
            # never receives it, so nobody else would stop it.
            self.stop(kill=True)
            raise

    def req(self, method, path, body=None, timeout=REQUEST_TIMEOUT, enforce=True):
        c = http.client.HTTPConnection('127.0.0.1', self.port, timeout=timeout)
        headers = {'Authorization': 'Bearer ' + TOKEN}
        data = None
        if body is not None:
            data = json.dumps(body)
            headers['Content-Type'] = 'application/json'
        self.seq += 1
        real = time.time()
        start = time.monotonic()
        if enforce:
            self.inflight = (method, path, start)
        status = '-'
        try:
            c.request(method, path, data, headers)
            r = c.getresponse()
            result = r.status, json.loads(r.read())
            status = str(result[0])
            elapsed = time.monotonic() - start
            if enforce:
                self.latencies.append(elapsed)
                self.last_done = (method, path, result[0], elapsed)
            if enforce and elapsed > self.deadline:
                # Round 7: a reply after the total deadline is still a failure.
                self.breach = self.breach or (method, path, start, start + elapsed, f'reply {result[0]} after {elapsed:.3f}s')
                raise DeadlineExceeded(f'{method} {path}: total {elapsed:.3f}s > {self.deadline:g}s deadline (reply {result[0]})')
            if enforce:
                self.inflight = None
            return result
        except (OSError, http.client.HTTPException) as e:
            status = 'EXC:' + type(e).__name__
            elapsed = time.monotonic() - start
            if enforce and (isinstance(e, TimeoutError) or elapsed > self.deadline):
                self.breach = self.breach or (method, path, start, start + elapsed, f'{type(e).__name__} after {elapsed:.3f}s')
                raise DeadlineExceeded(f'{method} {path}: {type(e).__name__} after {elapsed:.3f}s (> {self.deadline:g}s deadline)') from e
            raise
        finally:
            c.close()
            end = time.monotonic()
            try:
                self.req_log.write(f'{self.seq}\t{method}\t{path}\t{start:.6f}\t{end:.6f}\t{real:.6f}\t{status}\t{end - start:.6f}\n')
            except (OSError, ValueError):
                pass

    def start_watchdog(self):
        """In-flight watchdog (Round 7). Diagnostic only: never kills, never changes a verdict."""
        self.wd_stop = threading.Event()
        self.wd_base = None  # last sample taken before the current in-flight request started
        self.wd_log = open(self.log_path.with_name(self.log_path.stem + '_watchdog.log'), 'w')
        self.wd_episodes = []
        def loop():
            last = None
            current = None
            sampled = 0
            while not self.wd_stop.is_set():
                if self.p.poll() is not None:
                    break
                inflight = self.inflight
                if inflight is not current:
                    if current is not None and sampled:
                        m, pth, st = current
                        self.wd_episodes.append((m, pth, st, time.monotonic(), sampled))
                    current, sampled = inflight, 0
                    self.wd_base = last if (last and inflight and last['mono'] <= inflight[2]) else None
                now = proc_sample(self.p.pid, self.dev)
                if current is not None and now['mono'] - current[2] >= WATCHDOG_AFTER:
                    if self.wd_base is None:
                        self.wd_base = last or now
                    sampled += 1
                    m, pth, st = current
                    out = [f'WD sample={sampled} {m} {pth} inflight={now["mono"] - st:.3f}s']
                    out += sample_delta_lines(self.wd_base, now, 'WD')
                    try:
                        self.wd_log.write('\n'.join(out) + '\n')
                        self.wd_log.flush()
                    except (OSError, ValueError):
                        pass
                if current is None or now['mono'] - current[2] < WATCHDOG_AFTER:
                    last = now
                self.wd_stop.wait(WATCHDOG_EVERY)
            if current is not None and sampled:
                m, pth, st = current
                self.wd_episodes.append((m, pth, st, time.monotonic(), sampled))
        self.wd_thread = threading.Thread(target=loop, daemon=True)
        self.wd_thread.start()

    def stop_watchdog(self):
        if getattr(self, 'wd_stop', None) is None:
            return
        self.wd_stop.set()
        self.wd_thread.join(5)
        for f in (self.wd_log, getattr(self, 'req_log', None)):
            try:
                if f is not None:
                    f.close()
            except Exception:
                pass

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
            pid = self.p.pid
            info = f'DIAG server pid={pid} alive'
            try:
                stat = Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()
                tick = os.sysconf('SC_CLK_TCK')
                info += f' state={stat[0]} utime={int(stat[11]) / tick:.2f}s stime={int(stat[12]) / tick:.2f}s threads={stat[17]}'
            except (OSError, IndexError, ValueError):
                pass
            lines.append(info)
            # Round 7: schedstat/diskstats deltas since the last sample before the request started.
            base = getattr(self, 'wd_base', None)
            if base is not None and inflight:
                try:
                    lines.extend(sample_delta_lines(base, proc_sample(pid, getattr(self, 'dev', None)), 'DIAG since_request_start'))
                except Exception as e:  # diagnostic only
                    lines.append(f'DIAG since_request_start unavailable: {e}')
            else:
                lines.append('DIAG since_request_start: no pre-request baseline sample')
            # Capture /proc before SIGQUIT: a SIGSTOPped or fsync-stuck process still has these.
            try:
                wchan = Path(f'/proc/{pid}/wchan').read_text().strip() or '?'
                lines.append(f'DIAG wchan={wchan}')
            except OSError as e:
                lines.append(f'DIAG wchan unavailable: {e}')
            try:
                status = Path(f'/proc/{pid}/status').read_text()
                keep = []
                for line in status.splitlines():
                    if line.startswith(('Name:', 'State:', 'Pid:', 'PPid:', 'Threads:', 'voluntary_ctxt_switches:', 'nonvoluntary_ctxt_switches:')):
                        keep.append(line)
                lines.append('DIAG status: ' + ' | '.join(keep))
            except OSError as e:
                lines.append(f'DIAG status unavailable: {e}')
            try:
                tasks = sorted(Path(f'/proc/{pid}/task').iterdir(), key=lambda x: int(x.name))
                for task in tasks[:16]:
                    try:
                        tstat = task.joinpath('stat').read_text().rsplit(')', 1)[1].split()
                        twchan = task.joinpath('wchan').read_text().strip() or '?'
                        lines.append(f'DIAG task {task.name} state={tstat[0]} wchan={twchan}')
                    except (OSError, IndexError, ValueError):
                        pass
                if len(tasks) > 16:
                    lines.append(f'DIAG task ... ({len(tasks) - 16} more)')
            except OSError as e:
                lines.append(f'DIAG task/*/stat unavailable: {e}')
            saved = self.inflight
            try:
                t0 = time.monotonic()
                st = self.req('GET', '/v1/health', timeout=3, enforce=False)[0]
                lines.append(f'DIAG health_probe status={st} elapsed={time.monotonic() - t0:.3f}s')
            except Exception as e:  # diagnostic only
                lines.append(f'DIAG health_probe error={type(e).__name__}: {e}')
            self.inflight = saved
            # Goroutine dump: SIGQUIT; a SIGSTOPped process will not exit, so fall back to SIGKILL.
            try:
                self.p.send_signal(signal.SIGQUIT)
                try:
                    self.p.wait(5)
                    lines.append(f'DIAG sigquit sent; server exited code={self.p.returncode}; goroutine dump in {self.log_path.name}')
                except subprocess.TimeoutExpired:
                    lines.append('DIAG sigquit: no exit in 5s (likely stopped or stuck); falling back to SIGKILL')
                    self.p.kill()
                    try:
                        self.p.wait(5)
                        lines.append(f'DIAG sigkill fallback; server exited code={self.p.returncode}')
                    except subprocess.TimeoutExpired:
                        lines.append('DIAG sigkill fallback: still alive after 5s')
            except Exception as e:  # diagnostic only
                lines.append(f'DIAG sigquit error={type(e).__name__}: {e}')
                try:
                    self.p.kill()
                    self.p.wait(5)
                    lines.append(f'DIAG sigkill after sigquit error; exited code={self.p.returncode}')
                except Exception as e2:
                    lines.append(f'DIAG sigkill after sigquit error also failed: {e2}')
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
        """Stop the server. Always falls back to SIGKILL if a polite signal does not exit."""
        if self.log.closed:
            return
        try:
            self.stop_watchdog()
        except Exception:
            pass
        if self.p.poll() is None:
            if kill:
                self.p.kill()
            else:
                self.p.terminate()
            try:
                self.p.wait(5)
            except subprocess.TimeoutExpired:
                try:
                    self.p.kill()
                    self.p.wait(5)
                except Exception:
                    pass
        else:
            try:
                self.p.wait(1)
            except Exception:
                pass
        try:
            self.log.close()
        except Exception:
            pass

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


def clear_tmp_files(state, art, run):
    """After a snapshot restore, clear leftover state.json.tmp-* from a prior kill.
    Files are timestamped aside into the artifacts dir when present, then removed."""
    moved = []
    for f in sorted(state.parent.iterdir()):
        if '.tmp-' in f.name:
            dest = art / f'run{run:02d}_pre_{int(time.time())}_{f.name}'
            try:
                shutil.move(str(f), str(dest))
                moved.append(f'{f.name}->{dest.name}')
            except OSError:
                try:
                    f.unlink()
                    moved.append(f'{f.name}->deleted')
                except OSError as e:
                    moved.append(f'{f.name}->error:{e}')
    return moved


def wait_stopped(pid, limit=5.0):
    """Poll /proc/PID/stat until the process is in state T (#194: SIGSTOP is asynchronous)."""
    end = time.monotonic() + limit
    while time.monotonic() < end:
        stat = read_text(f'/proc/{pid}/stat')
        if stat and stat.rsplit(')', 1)[1].split()[0] in ('T', 't'):
            return
        time.sleep(0.01)
    raise AssertionError(f'pid {pid} did not reach state T within {limit}s')


def self_test_sigstop(art):
    """Prove that a SIGSTOPped server still yields /proc diagnostics and that
    stop() falls back to SIGKILL after SIGQUIT times out. Stdlib only."""
    art.mkdir(parents=True, exist_ok=True)
    log = art / 'selftest_sigstop.log'
    # A long sleep stands in for a stuck Go server: it has /proc and ignores SIGQUIT while stopped.
    p = subprocess.Popen(['sleep', '120'], stdout=open(log, 'w+b'), stderr=subprocess.STDOUT)
    try:
        time.sleep(0.05)
        os.kill(p.pid, signal.SIGSTOP)
        wait_stopped(p.pid)
        s = Server.__new__(Server)
        s.state = art / 'dummy_state.json'
        s.state.write_text('{}')
        (art / 'dummy_state.json.tmp-selftest').write_text('partial')
        s.log_path = log
        s.log = open(log, 'a+b')
        s.latencies = []
        s.inflight = ('DELETE', '/v1/tickets/t1', time.monotonic() - 10.5)
        s.last_done = ('DELETE', '/v1/tickets/t0', 200, 0.01)
        s.p = p
        s.port = 1
        dump = art / 'selftest_failure'
        s.diag('self-test SIGSTOP', dump)
        # The first pass checks the dump and the SIGKILL fallback; the second captures the printed /proc lines.
        require(dump.exists(), 'failure dump dir missing')
        require(any(dump.iterdir()), 'failure dump empty')
        require(p.poll() is not None, 'server still alive after diag (SIGKILL fallback failed)')
        # Re-run diag capture via a second stopped process and capture printed lines.
        p2 = subprocess.Popen(['sleep', '120'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(0.05)
        os.kill(p2.pid, signal.SIGSTOP)
        wait_stopped(p2.pid)
        buf = io.StringIO()
        s2 = Server.__new__(Server)
        s2.state = s.state
        s2.log_path = art / 'selftest2.log'
        s2.log = open(s2.log_path, 'w+b')
        s2.latencies = []
        s2.inflight = None
        s2.last_done = None
        s2.p = p2
        s2.port = 1
        with contextlib.redirect_stdout(buf):
            s2.diag('self-test SIGSTOP capture', art / 'selftest2_failure')
        text = buf.getvalue()
        require('DIAG wchan=' in text, 'missing wchan')
        require('DIAG status:' in text and 'State:' in text, 'missing status')
        require('DIAG task ' in text, 'missing task/*/stat')
        require('falling back to SIGKILL' in text or 'sigkill' in text.lower(), 'missing SIGKILL fallback')
        require(p2.poll() is not None, 'second server still alive')
        print('SELFTEST_SIGSTOP PASS', flush=True)
    finally:
        if p.poll() is None:
            try:
                os.kill(p.pid, signal.SIGCONT)
            except OSError:
                pass
            p.kill()
            p.wait(5)


# ----------------------------------------------------------------------------- Round 7 noise
def clocks():
    return time.monotonic(), time.time(), time.clock_gettime(time.CLOCK_BOOTTIME)


def sidecar_heartbeat(art):
    """Heartbeat sidecar: sleep 50 ms in a loop; log gaps > 250 ms on any clock."""
    try:
        cpus = sorted(os.sched_getaffinity(0))
        os.sched_setaffinity(0, {cpus[-1]})
        pinned = cpus[-1]
    except (AttributeError, OSError, IndexError):
        pinned = 'none'
    parent = os.getppid()
    with open(art / 'heartbeat.log', 'a', buffering=1) as out:
        m0, r0, b0 = clocks()
        out.write(f'HB start mono={m0:.6f} real={r0:.6f} boot={b0:.6f} cpu={pinned}\n')
        alive_at = m0
        while os.getppid() == parent:
            time.sleep(HEARTBEAT_SLEEP)
            m1, r1, b1 = clocks()
            dm, dr, db = m1 - m0, r1 - r0, b1 - b0
            if max(dm, dr, db) > HEARTBEAT_GAP_LOG:
                out.write(f'HB gap mono_start={m0:.6f} mono_end={m1:.6f} mono_d={dm:.3f} real_d={dr:.3f} boot_d={db:.3f}\n')
            if m1 - alive_at >= HEARTBEAT_ALIVE_EVERY:
                out.write(f'HB alive mono={m1:.6f}\n')
                alive_at = m1
            m0, r0, b0 = m1, r1, b1


def cgroup_dir():
    text = read_text('/proc/self/cgroup') or ''
    for line in text.splitlines():
        if line.startswith('0::'):
            return Path('/sys/fs/cgroup') / line[3:].lstrip('/')
    return None


def kv_file(path):
    text = read_text(path)
    if text is None:
        return None
    out = {}
    for line in text.splitlines():
        f = line.split()
        if len(f) == 2:
            try:
                out[f[0]] = int(f[1])
            except ValueError:
                pass
    return out


def sidecar_sampler(art, dev):
    """Noise sampler sidecar: one JSON line per second, plus new /dev/kmsg records."""
    parent = os.getppid()
    cg = cgroup_dir()
    try:
        kmsg = os.open('/dev/kmsg', os.O_RDONLY | os.O_NONBLOCK)
        os.lseek(kmsg, 0, os.SEEK_END)
    except OSError as e:
        kmsg = None
        (art / 'kmsg.log').write_text(f'KMSG unavailable: {e}\n')
    with open(art / 'noise.jsonl', 'a', buffering=1) as out, open(art / 'kmsg.log', 'a', buffering=1) as klog:
        if kmsg is not None:
            klog.write(f'KMSG start mono={time.monotonic():.6f}\n')
        while os.getppid() == parent:
            m, r, b = clocks()
            cpu = (read_text('/proc/stat') or 'cpu').splitlines()[0].split()[1:]
            vm = kv_file('/proc/vmstat') or {}
            rec = {'mono': m, 'real': r, 'boot': b, 'cpu': [int(x) for x in cpu],
                   'psi': {k: read_text(f'/proc/pressure/{k}') for k in ('cpu', 'io', 'memory')},
                   'cg_cpu': kv_file(cg / 'cpu.stat') if cg else None,
                   'cg_mem_pressure': read_text(cg / 'memory.pressure') if cg else None,
                   'cg_mem_events': kv_file(cg / 'memory.events') if cg else None,
                   'disk': diskstats_line(dev), 'pgmajfault': vm.get('pgmajfault')}
            out.write(json.dumps(rec) + '\n')
            if kmsg is not None:
                while True:
                    try:
                        chunk = os.read(kmsg, 8192)
                    except BlockingIOError:
                        break
                    except OSError as e:  # EPIPE: records overwritten; keep reading
                        klog.write(f'KMSG read error {e} mono={time.monotonic():.6f}\n')
                        continue
                    if not chunk:
                        break
                    text = chunk.decode('utf-8', 'replace').split(';', 1)[-1].split('\n', 1)[0]
                    hit = any(pat in text for pat in KMSG_PATTERNS)
                    klog.write(f'KMSG observed_mono={time.monotonic():.6f} match={int(hit)} {text}\n')
            time.sleep(max(0.0, SAMPLER_EVERY - (time.monotonic() - m)))


def start_sidecars(art, dev):
    procs = []
    for name, extra in (('heartbeat', []), ('sampler', ['--state-dev', dev or ''])):
        procs.append(subprocess.Popen([sys.executable, str(SCRIPT_PATH), '--sidecar', name,
                                       '--artifacts', str(art)] + extra,
                                      stdout=subprocess.DEVNULL, stderr=open(art / f'sidecar_{name}.err', 'w')))
    return procs


def stop_sidecars(procs):
    for p in procs:
        if p.poll() is None:
            p.kill()
        try:
            p.wait(5)
        except subprocess.TimeoutExpired:
            pass


def psi_value(text, kind, field):
    """psi_value(text, 'full', 'avg10') from a /proc/pressure/* file."""
    if not text:
        return None
    for line in text.splitlines():
        if line.startswith(kind + ' '):
            m = re.search(field + r'=([0-9.]+)', line)
            if m:
                return float(m.group(1))
    return None


def load_noise(art):
    gaps, alive, starts = [], [], []
    for line in (read_text(art / 'heartbeat.log') or '').splitlines():
        f = dict(x.split('=', 1) for x in line.split()[2:] if '=' in x)
        if line.startswith('HB gap'):
            gaps.append({'start': float(f['mono_start']), 'end': float(f['mono_end']),
                         'size': max(float(f['mono_d']), float(f['real_d']), float(f['boot_d'])), 'line': line})
        elif line.startswith('HB alive'):
            alive.append(float(f['mono']))
        elif line.startswith('HB start'):
            starts.append(float(f['mono']))
    samples = []
    for line in (read_text(art / 'noise.jsonl') or '').splitlines():
        try:
            samples.append(json.loads(line))
        except ValueError:
            pass
    events, kmsg_ok = [], False
    for line in (read_text(art / 'kmsg.log') or '').splitlines():
        if line.startswith('KMSG start'):
            kmsg_ok = True
        elif line.startswith('KMSG observed_mono=') and ' match=1 ' in line:
            events.append({'mono': float(line.split()[1].split('=')[1]), 'line': line})
    return {'gaps': gaps, 'alive': alive, 'hb_starts': starts, 'samples': samples, 'events': events, 'kmsg_ok': kmsg_ok}


def steal_windows(samples, span):
    """[(start, end, steal_pct)] for each sample against the first sample >= span seconds earlier."""
    out = []
    j = 0
    for i, s in enumerate(samples):
        while j < i and s['mono'] - samples[j + 1]['mono'] >= span:
            j += 1
        if s['mono'] - samples[j]['mono'] < span * 0.5:
            continue
        a, b = samples[j]['cpu'], s['cpu']
        if len(a) < 8 or len(b) < 8:
            continue
        total = sum(b[:8]) - sum(a[:8])
        if total > 0:
            out.append((samples[j]['mono'], s['mono'], 100.0 * (b[7] - a[7]) / total))
    return out


def heartbeat_holes(noise, t0, t1):
    """Spans in [t0, t1] with no heartbeat record (gap or alive) for > HEARTBEAT_STALE."""
    marks = sorted([t0] + noise['alive'] + [g['end'] for g in noise['gaps']] + noise['hb_starts'])
    marks = [m for m in marks if t0 <= m <= t1] + [t1]
    holes, prev = [], t0
    for m in marks:
        if m - prev > HEARTBEAT_STALE:
            holes.append((prev, m))
        prev = max(prev, m)
    return holes


def idle_gate(noise, t0, t1):
    """Pre-run idle gate. Returns a list of reasons (empty = pass)."""
    reasons = []
    win = [s for s in noise['samples'] if t0 <= s['mono'] <= t1]
    if len(win) < 2:
        return ['noise sampler produced < 2 samples']
    if not noise['hb_starts']:
        reasons.append('heartbeat sidecar did not start')
    if not noise['kmsg_ok']:
        reasons.append('/dev/kmsg unavailable (required to see VM fork/balloon events)')
    a, b = win[0]['cpu'], win[-1]['cpu']
    total = sum(b[:8]) - sum(a[:8])
    steal = 100.0 * (b[7] - a[7]) / total if total > 0 else 0.0
    if steal >= IDLE_MAX_STEAL_PCT:
        reasons.append(f'steal {steal:.2f}% >= {IDLE_MAX_STEAL_PCT}%')
    last = win[-1]['psi']
    for kind, res, field, limit, strict in (('some', 'cpu', 'avg60', IDLE_MAX_PSI_CPU_SOME_AVG60, True),
                                             ('full', 'io', 'avg60', IDLE_MAX_PSI_IO_FULL_AVG60, True),
                                             ('full', 'memory', 'avg60', IDLE_MAX_PSI_MEM_FULL_AVG60, False)):
        v = psi_value(last.get(res), kind, field)
        if v is None:
            reasons.append(f'PSI {res} unavailable')
        elif (v >= limit) if strict else (v > limit):
            reasons.append(f'PSI {res} {kind} {field}={v} (limit {"<" if strict else "<="} {limit})')
    for g in noise['gaps']:
        if g['end'] >= t0 and g['start'] <= t1 and g['size'] > IDLE_MAX_HEARTBEAT_GAP:
            reasons.append(f'heartbeat gap {g["size"]:.3f}s > {IDLE_MAX_HEARTBEAT_GAP}s')
            break
    for e in noise['events']:
        if t0 <= e['mono'] <= t1 + KMSG_OBSERVE_SLACK:
            reasons.append('kmsg: ' + e['line'].split(' ', 3)[-1])
    for h in heartbeat_holes(noise, t0, t1):
        reasons.append(f'heartbeat silent {h[1] - h[0]:.1f}s')
    return reasons


def run_noise(noise, t0, t1):
    """In-run noise gates over [t0, t1]. Returns (breaches, steal_breach_windows)."""
    breaches = []
    for g in noise['gaps']:
        if g['end'] >= t0 and g['start'] <= t1 and g['size'] > RUN_MAX_HEARTBEAT_GAP:
            breaches.append(f'heartbeat gap {g["size"]:.3f}s at mono {g["start"]:.3f}')
    win = [s for s in noise['samples'] if t0 - RUN_STEAL_WINDOW <= s['mono'] <= t1]
    bad_steal = [w for w in steal_windows(win, RUN_STEAL_WINDOW) if w[2] > RUN_MAX_STEAL_PCT and w[1] >= t0]
    for w in bad_steal[:5]:
        breaches.append(f'steal {w[2]:.2f}% in 60s window ending mono {w[1]:.3f}')
    inrun = [s for s in noise['samples'] if t0 <= s['mono'] <= t1]
    for s in inrun:
        v = psi_value(s['psi'].get('io'), 'full', 'avg10')
        if v is not None and v > RUN_MAX_PSI_IO_FULL_AVG10:
            breaches.append(f'PSI io full avg10={v} at mono {s["mono"]:.3f}')
            break
    thr = [s['cg_cpu']['nr_throttled'] for s in inrun if s.get('cg_cpu') and 'nr_throttled' in s['cg_cpu']]
    if thr and thr[-1] > thr[0]:
        breaches.append(f'cgroup nr_throttled +{thr[-1] - thr[0]}')
    for e in noise['events']:
        if t0 <= e['mono'] <= t1 + KMSG_OBSERVE_SLACK:
            breaches.append('kmsg: ' + e['line'].split(' ', 3)[-1])
    holes = heartbeat_holes(noise, t0, t1)
    for h in holes:
        breaches.append(f'heartbeat silent {h[1] - h[0]:.1f}s from mono {h[0]:.3f} (sidecar lost)')
    if len(inrun) < max(2, int((t1 - t0) / SAMPLER_EVERY * 0.5)):
        breaches.append(f'noise sampler coverage too low ({len(inrun)} samples over {t1 - t0:.0f}s)')
    return breaches, bad_steal


def overlap(a0, a1, b0, b1):
    return max(0.0, min(a1, b1) - max(a0, b0))


def classify(failure, noise, steal_bad, gates_hold):
    """Pre-registered Round 7 failure classification. Returns (class, reason)."""
    w = failure.get('window')
    if not w:
        return 'product', 'no stalled request window (check failure on a reply)'
    start, end = w
    dur = max(end - start, 1e-9)
    gaps = [g for g in noise['gaps'] if overlap(g['start'], g['end'], start, end) > 0]
    cover = sum(overlap(g['start'], g['end'], start, end) for g in gaps)
    if cover >= CLASSIFY_GAP_COVER * dur:
        return 'infra', f'heartbeat gaps cover {cover:.3f}s of {dur:.3f}s (>= {CLASSIFY_GAP_COVER:.0%})'
    ev = [e for e in noise['events'] if start <= e['mono'] <= end + KMSG_OBSERVE_SLACK]
    if ev:
        return 'infra', 'kmsg event overlaps: ' + ev[0]['line'].split(' ', 3)[-1]
    st = [x for x in steal_bad if overlap(x[0], x[1], start, end) > 0]
    if st:
        return 'infra', f'steal breach {st[0][2]:.2f}% overlaps the request'
    big = [g for g in gaps if g['size'] >= CLASSIFY_GAP_MIN]
    if not big and gates_hold:
        return 'product', f'no heartbeat gap >= {CLASSIFY_GAP_MIN}s overlaps and noise gates hold'
    return 'product', f'ambiguous (gap cover {cover:.3f}s of {dur:.3f}s; gates_hold={gates_hold}); ambiguity defaults to product'


def heartbeat_excerpt(art, t0, t1):
    out = []
    for g in load_noise(art)['gaps']:
        if g['end'] >= t0 and g['start'] <= t1:
            out.append(g['line'])
    return out or [f'no heartbeat gap > {HEARTBEAT_GAP_LOG}s between mono {t0:.3f} and {t1:.3f}']


def self_test_noise(art):
    """Self-test of the Round 7 noise machinery. Stdlib only, about 3 s."""
    art.mkdir(parents=True, exist_ok=True)
    # 1. A real heartbeat sidecar sees a SIGSTOP pause as a gap.
    hb = subprocess.Popen([sys.executable, str(SCRIPT_PATH), '--sidecar', 'heartbeat', '--artifacts', str(art)])
    try:
        for _ in range(100):
            if 'HB start' in (read_text(art / 'heartbeat.log') or ''):
                break
            time.sleep(0.02)
        os.kill(hb.pid, signal.SIGSTOP)
        time.sleep(0.6)
        os.kill(hb.pid, signal.SIGCONT)
        time.sleep(0.3)
    finally:
        hb.kill()
        hb.wait(5)
    gaps = load_noise(art)['gaps']
    require(any(g['size'] >= 0.5 for g in gaps), f'heartbeat missed a 0.6s SIGSTOP pause: {gaps}')
    print('SELFTEST_NOISE heartbeat gap PASS', flush=True)
    # 2. The total deadline fails a slow reply, and a socket timeout, as DeadlineExceeded.
    import http.server
    class Slow(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            time.sleep(0.4)
            body = b'{"ok": 1}'
            try:
                self.send_response(200)
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            except OSError:
                pass  # the client already gave up (the timeout case)
        def log_message(self, *a):
            pass
    httpd = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Slow)
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    try:
        s = Server.__new__(Server)
        s.port, s.deadline, s.breach, s.seq, s.latencies = httpd.server_address[1], 0.2, None, 0, []
        s.inflight = s.last_done = None
        s.req_log = open(art / 'selftest_requests.tsv', 'w')
        try:
            s.req('GET', '/x')
            raise AssertionError('slow reply accepted')
        except DeadlineExceeded:
            pass
        require(s.breach and s.breach[0] == 'GET', 'breach not recorded')
        s.breach = None
        try:
            s.req('GET', '/x', timeout=0.1)
            raise AssertionError('socket timeout accepted')
        except DeadlineExceeded as e:
            require(isinstance(e.__cause__, TimeoutError), 'timeout not chained')
        s.deadline = 5
        require(s.req('GET', '/x')[0] == 200, 'fast-enough reply rejected')
        s.req_log.close()
        rows = (art / 'selftest_requests.tsv').read_text().splitlines()
        require(len(rows) == 3 and rows[1].endswith(tuple('0123456789')) and 'EXC:' in rows[1], f'request log rows: {rows}')
    finally:
        httpd.shutdown()
    print('SELFTEST_NOISE total deadline PASS', flush=True)
    # 3. The watchdog samples a stalled request with schedstat deltas.
    p = subprocess.Popen(['sleep', '30'])
    try:
        s = Server.__new__(Server)
        s.p, s.dev, s.log_path = p, None, art / 'selftest_wd.log'
        s.inflight = None
        s.start_watchdog()
        time.sleep(0.6)
        s.inflight = ('DELETE', '/v1/tickets/t1', time.monotonic() - WATCHDOG_AFTER)
        time.sleep(1.2)
        s.inflight = None
        time.sleep(0.6)
        s.stop_watchdog()
    finally:
        p.kill()
        p.wait(5)
    wd = (art / 'selftest_wd_watchdog.log').read_text()
    require('WD sample=1 DELETE /v1/tickets/t1' in wd and 'oncpu+=' in wd and 'rqwait+=' in wd, f'watchdog log: {wd}')
    print('SELFTEST_NOISE watchdog PASS', flush=True)
    # 4. Classification and gates on synthetic data (fixed rules).
    def gap(a, b):
        return {'start': a, 'end': b, 'size': b - a, 'line': f'HB gap {a} {b}'}
    base = {'gaps': [], 'alive': [], 'hb_starts': [0.0], 'samples': [], 'events': [], 'kmsg_ok': True}
    cases = [
        ('pause covers request', dict(base, gaps=[gap(100, 170)]), (101, 169), [], True, 'infra'),
        ('vm fork during request', dict(base, events=[{'mono': 150.5, 'line': 'KMSG x y virtual machine fork'}]), (100, 150), [], True, 'infra'),
        ('steal breach overlaps', base, (100, 150), [(90, 151, 5.0)], True, 'infra'),
        ('quiet box, slow handler', base, (100, 115), [], True, 'product'),
        ('small gap, partial cover', dict(base, gaps=[gap(100, 102)]), (100, 115), [], True, 'product'),
        ('no window', base, None, [], True, 'product'),
    ]
    for name, noise, window, steal, gates, want in cases:
        got, why = classify({'window': window}, noise, steal, gates)
        require(got == want, f'classify {name}: got {got} ({why}), want {want}')
    def sample(t, steal, psi_io='full avg10=0.00 avg60=0.00 avg300=0.00 total=0'):
        return {'mono': t, 'cpu': [100 * t, 0, 0, 100 * t, 0, 0, 0, steal * t, 0, 0],
                'psi': {'cpu': 'some avg10=0.00 avg60=0.00 avg300=0.00 total=0',
                        'io': 'some avg10=0.00 avg60=0.00 avg300=0.00 total=0\n' + psi_io,
                        'memory': 'some avg10=0.00 avg60=0.00 avg300=0.00 total=0\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0'},
                'cg_cpu': {'nr_throttled': 0}}
    quiet = dict(base, alive=[float(t) for t in range(10, 121, 10)], samples=[sample(t, 0) for t in range(0, 121)])
    require(idle_gate(quiet, 0, 120) == [], f'quiet idle gate failed: {idle_gate(quiet, 0, 120)}')
    require(run_noise(quiet, 0, 120)[0] == [], f'quiet run noise: {run_noise(quiet, 0, 120)[0]}')
    stealy = dict(quiet, samples=[sample(t, 10) for t in range(0, 121)])
    require(any('steal' in r for r in idle_gate(stealy, 0, 120)), 'idle gate missed steal')
    require(any('steal' in r for r in run_noise(stealy, 0, 120)[0]), 'run gate missed steal')
    gappy = dict(quiet, gaps=[gap(50, 50.3)])
    require(any('heartbeat gap' in r for r in idle_gate(gappy, 0, 120)), 'idle gate missed a 0.3s gap')
    require(run_noise(gappy, 0, 120)[0] == [], '0.3s gap must not void a run')
    require(run_noise(dict(quiet, gaps=[gap(50, 52.5)]), 0, 120)[0], 'run gate missed a 2.5s gap')
    forked = dict(quiet, events=[{'mono': 60.0, 'line': 'KMSG x y virtio_balloon virtio0: Out of puff!'}])
    require(idle_gate(forked, 0, 120) and run_noise(forked, 0, 120)[0], 'gates missed a balloon event')
    ioful = dict(quiet, samples=[sample(t, 0, 'full avg10=12.00 avg60=3.00 avg300=0.00 total=0') for t in range(0, 121)])
    require(any('PSI io' in r for r in idle_gate(ioful, 0, 120)), 'idle gate missed io pressure')
    require(any('PSI io' in r for r in run_noise(ioful, 0, 120)[0]), 'run gate missed io pressure')
    thr = dict(quiet, samples=[dict(sample(t, 0), cg_cpu={'nr_throttled': t // 60}) for t in range(0, 121)])
    require(any('nr_throttled' in r for r in run_noise(thr, 0, 120)[0]), 'run gate missed throttling')
    silent = dict(quiet, alive=[10.0])
    require(any('silent' in r for r in run_noise(silent, 0, 120)[0]), 'run gate missed a lost heartbeat')
    require(any('kmsg' in r for r in idle_gate(dict(quiet, kmsg_ok=False), 0, 120)), 'idle gate needs kmsg')
    print('SELFTEST_NOISE classify+gates PASS', flush=True)
    print('SELFTEST_NOISE PASS', flush=True)


DECISION_DOC = 'milestone2/MILESTONE2_DECISION.md'
PIN_LINE_PREFIX = '  * **Round 7 code-under-test SHA:** `'
PIN_PLACEHOLDER = 'TO BE PINNED AFTER THE TIMING PR MERGES (main including the --timing-log PR and this script)'


def git(*args):
    return subprocess.run(['git', *args], check=True, stdout=subprocess.PIPE, text=True).stdout


def verify_head(expect, head):
    """Return the pinned code-under-test commit, or exit. Accepted only:
    (a) HEAD == pin; or
    (b) HEAD is one commit directly on the pin (single parent == pin) whose whole diff,
        with renames disabled, is `1 1 milestone2/MILESTONE2_DECISION.md`, replacing the
        Code-under-test line with one that names the pin."""
    pinned = subprocess.run(['git', 'rev-parse', '--verify', expect + '^{commit}'],
                            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True).stdout.strip()
    if not pinned:
        raise SystemExit(f'code under test mismatch: {expect} is not a commit')
    if pinned == head:
        print(f'CODE_PINNED {pinned} (= HEAD)', flush=True)
        return pinned
    parents = git('rev-list', '--parents', '-n', '1', head).split()[1:]
    if parents != [pinned]:
        raise SystemExit(f'code under test mismatch: HEAD {head} is not a single commit directly on pin {pinned} (parents {parents})')
    numstat = git('diff', '--no-renames', '--numstat', pinned, head).splitlines()
    if numstat != [f'1\t1\t{DECISION_DOC}']:
        raise SystemExit(f'code under test mismatch: pin commit must change exactly 1 line of {DECISION_DOC}; numstat {numstat}')
    hunk = git('diff', '--no-renames', '-U0', pinned, head, '--', DECISION_DOC).splitlines()
    removed = [l[1:] for l in hunk if l.startswith('-') and not l.startswith('---')]
    added = [l[1:] for l in hunk if l.startswith('+') and not l.startswith('+++')]
    # Exact lines only: the placeholder line becomes exactly the pin line (no substring match,
    # no trailing text such as "(not <pin>)" or a relaxed threshold appended, and since Round 7
    # no trailing whitespace either: the comparison is byte-exact, #194).
    if (len(removed) != 1 or len(added) != 1
            or removed[0] != PIN_LINE_PREFIX + PIN_PLACEHOLDER + '`'
            or added[0] != PIN_LINE_PREFIX + pinned + '`'):
        raise SystemExit(f'code under test mismatch: the one changed line must replace the placeholder with exactly {PIN_LINE_PREFIX.strip()}{pinned}`')
    print(f'CODE_PINNED {pinned} (HEAD {head} only fills the pin line)', flush=True)
    return pinned


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--runs', type=int, default=25)
    ap.add_argument('--artifacts', type=Path,
                    default=Path(tempfile.gettempdir()) / time.strftime('b6-storm-%Y%m%d-%H%M%S'))
    ap.add_argument('--self-test-sigstop', action='store_true',
                    help='run the SIGSTOP diagnostics self-test and exit')
    ap.add_argument('--expect-sha256', default='',
                    help='if set, require the script sha256 to equal this value')
    ap.add_argument('--expect-head', default='',
                    help='pinned code-under-test commit: HEAD itself, or the parent of a commit that only fills the pin line')
    ap.add_argument('--check-head-only', action='store_true',
                    help='verify --expect-head and exit (used by tests)')
    ap.add_argument('--self-test-noise', action='store_true',
                    help='run the Round 7 heartbeat/deadline/watchdog/gate self-test and exit')
    ap.add_argument('--sidecar', choices=('heartbeat', 'sampler'), help=argparse.SUPPRESS)
    ap.add_argument('--state-dev', default='', help=argparse.SUPPRESS)
    args = ap.parse_args()
    art = args.artifacts.resolve()
    art.mkdir(parents=True, exist_ok=True)
    if args.sidecar == 'heartbeat':
        return sidecar_heartbeat(art)
    if args.sidecar == 'sampler':
        return sidecar_sampler(art, args.state_dev or None)
    sha = script_sha256()
    print(f'SCRIPT_SHA256 {sha}', flush=True)
    print(f'ARTIFACTS {art}', flush=True)
    if args.expect_sha256 and args.expect_sha256 != sha:
        raise SystemExit(f'script sha256 mismatch: got {sha}, expected {args.expect_sha256}')
    head = subprocess.run(['git', 'rev-parse', 'HEAD'], check=True, stdout=subprocess.PIPE, text=True).stdout.strip()
    print(f'CODE_SHA {head}', flush=True)
    pinned = verify_head(args.expect_head, head) if args.expect_head else 'unpinned'
    if args.check_head_only:
        print(f'CHECK_HEAD PASS pinned={pinned}', flush=True)
        return
    (art / 'script.sha256').write_text(sha + '\n')
    if args.self_test_sigstop:
        self_test_sigstop(art)
        return
    if args.self_test_noise:
        self_test_noise(art)
        return
    with tempfile.TemporaryDirectory(prefix='b6-independent-') as tmp:
        root = Path(tmp)
        try:
            dev = device_of(root)
        except OSError:
            dev = None
        sidecars = start_sidecars(art, dev)
        try:
            return storm(args, art, root, sha, head, pinned, sidecars)
        finally:
            stop_sidecars(sidecars)


def storm(args, art, root, sha, head, pinned, sidecars):
    """The registered invocation after the head/sha checks: idle gate, build, seed, runs, probes, verdict."""
    rng = random.Random(20261019)
    failures = 0
    failed = []  # Round 7: one record per failed run, classified after the invocation
    # Pre-run idle gate: 120 s of sampling before anything heavy. A failed gate is not an attempt.
    g0 = time.monotonic()
    print(f'IDLE_GATE sampling {IDLE_GATE_SECONDS:g}s', flush=True)
    time.sleep(IDLE_GATE_SECONDS)
    g1 = time.monotonic()
    reasons = idle_gate(load_noise(art), g0, g1)
    if any(p.poll() is not None for p in sidecars):
        reasons.append('a noise sidecar exited')
    if reasons:
        print('IDLE_GATE FAIL ' + '; '.join(reasons), flush=True)
        print('IDLE_GATE: box not quiet; this invocation is NOT an attempt (no build, no runs)', flush=True)
        raise SystemExit(3)
    print('IDLE_GATE PASS', flush=True)
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
    noise_t0 = time.monotonic()
    for run in range(1, args.runs + 1):
        state.write_bytes(snapshot)
        moved = clear_tmp_files(state, art, run)
        if moved:
            print(f'run={run:02d} cleared tmp after restore: {moved}', flush=True)
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
            dump = art / f'run{run:02d}_failure'
            if s.breach:
                window = (s.breach[2], s.breach[3])
            elif worker.is_alive() and s.inflight:  # storm worker hung past the kill
                window = (s.inflight[2], time.monotonic())
            else:
                window = None
            failed.append({'run': run, 'error': f'{type(e).__name__}: {e}', 'window': window})
            if window:
                for line in heartbeat_excerpt(art, window[0] - 5, window[1] + 5):
                    print(f'DIAG run={run:02d} heartbeat {line}', flush=True)
                wd = s.log_path.with_name(s.log_path.stem + '_watchdog.log')
                print(f'DIAG run={run:02d} watchdog samples in {wd.name}', flush=True)
            # Snapshot state on every failure (including 'confirmed deletion lost', where the
            # storm server is already stopped), independent of whether a server log is open.
            try:
                saved = s.save_state(dump)
                print(f'DIAG run={run:02d} failure state snapshot: {", ".join(saved)} -> {dump.name}/', flush=True)
            except OSError as se:
                print(f'DIAG run={run:02d} failure state snapshot error: {se}', flush=True)
            if not s.log.closed:
                # A live server also gets /proc + SIGQUIT (+ SIGKILL fallback) diagnostics.
                if s.p.poll() is None:
                    s.diag(f'run={run:02d} {type(e).__name__}', dump)
                print(f'run={run:02d} latency[{s.latency()}]', flush=True)
            print(f'run={run:02d} FAIL {type(e).__name__}: {e}', flush=True)
        finally:
            s.stop()
            if failed and failed[-1]['run'] == run:
                try:  # full server log (access + timing + gctrace) next to the failure snapshot
                    (art / f'run{run:02d}_failure').mkdir(parents=True, exist_ok=True)
                    shutil.copy2(s.log_path, art / f'run{run:02d}_failure' / s.log_path.name)
                except OSError:
                    pass
    noise_t1 = time.monotonic()
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
    noise = load_noise(art)
    breaches, steal_bad = run_noise(noise, noise_t0, noise_t1)
    if any(p.poll() is not None for p in sidecars):
        breaches.append('a noise sidecar exited during the invocation')
    for b in breaches:
        print(f'NOISE breach {b}', flush=True)
    quiet = not breaches
    classes = []
    for f in failed:
        cls, why = classify(f, noise, steal_bad, quiet)
        classes.append(cls)
        print(f'CLASSIFY run={f["run"]:02d} class={cls} reason={why}; error={f["error"]}', flush=True)
    if failures == 0:
        verdict = 'ACCEPTED' if quiet else 'INFRA-VOID'
    elif 'product' in classes:
        verdict = 'NOT_ACCEPTED_PRODUCT'
    else:
        verdict = 'NOT_ACCEPTED_INFRA'
    print(f'SUMMARY runs={args.runs} failures={failures} orphan_runs={orphan_runs} survivor_GETs={total_gets} survivor_DELETEs={total_deleted}; crafted probes=2 PASS; noise={"QUIET" if quiet else "INFRA-VOID"} breaches={len(breaches)}; SCRIPT_SHA256={sha}; CODE_SHA={head}; CODE_PINNED={pinned}', flush=True)
    rerun = ' (one isolation rerun allowed, same pins)' if verdict in ('INFRA-VOID', 'NOT_ACCEPTED_INFRA') else ''
    print(f'VERDICT {verdict}{rerun}', flush=True)
    require(failures == 0, 'kill storm failed')
    require(quiet, 'noise gates breached: INFRA-VOID')


if __name__ == '__main__':
    main()
