"""Spend ledger and cap enforcement shared by every run of one experiment.

The ledger is an append-only JSON-lines file, flushed and fsynced after every entry, so a restart rebuilds the
exact totals by replaying it. Three limits apply:

* **Admission:** a run starts only if the cap still covers its whole ceiling after reserving the unspent
  ceilings of every run already admitted. A run is therefore never cut short by the experiment cap.
* **Per call:** before forwarding, the call's worst-case cost is reserved; the call is refused unless the run's
  spend plus its in-flight reservations stays within the run ceiling, and the experiment's within the cap.
* **Tool calls:** a run that has received its maximum number of model tool calls gets no further model call.

There is no override. Raising a cap means a new ledger under a new written Owner authorization.
"""

import json
import os
import threading
import time


class BudgetRefused(Exception):
    def __init__(self, reason, detail=""):
        super().__init__(f"{reason}: {detail}" if detail else reason)
        self.reason = reason
        self.detail = detail


class Budget:
    def __init__(self, ledger_path, experiment_cap_usd, run_ceiling_usd, max_tool_calls):
        if run_ceiling_usd > experiment_cap_usd:
            raise ValueError("run ceiling exceeds the experiment cap")
        self.ledger_path = ledger_path
        self.cap = float(experiment_cap_usd)
        self.ceiling = float(run_ceiling_usd)
        self.max_tool_calls = int(max_tool_calls)
        self._lock = threading.Lock()
        self._settled = threading.Condition(self._lock)
        self.spent = 0.0
        self.run_spent = {}
        self.run_tools = {}
        self.run_pending = {}
        self.active = set()
        self.stopped = None
        self._next_reservation = 0
        self._reservations = {}
        self._replay()

    # Ledger ------------------------------------------------------------------------------------------------
    def _replay(self):
        if not os.path.exists(self.ledger_path):
            self._append({"kind": "open", "cap_usd": self.cap, "run_ceiling_usd": self.ceiling,
                          "max_tool_calls": self.max_tool_calls})
            return
        with open(self.ledger_path) as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                e = json.loads(line)
                kind = e.get("kind")
                if kind == "open":
                    if (e["cap_usd"], e["run_ceiling_usd"], e["max_tool_calls"]) != (
                            self.cap, self.ceiling, self.max_tool_calls):
                        raise ValueError("ledger was opened with different limits; a new cap needs a new ledger")
                elif kind == "admit":
                    self.active.add(e["run"])
                    self.run_spent.setdefault(e["run"], 0.0)
                    self.run_tools.setdefault(e["run"], 0)
                elif kind == "call":
                    self.spent += e["cost_usd"]
                    self.run_spent[e["run"]] = self.run_spent.get(e["run"], 0.0) + e["cost_usd"]
                    self.run_tools[e["run"]] = self.run_tools.get(e["run"], 0) + e.get("tool_calls", 0)
                elif kind == "release":
                    self.active.discard(e["run"])
                elif kind == "stop":
                    self.stopped = e.get("reason", "stopped")
        # Runs that were in flight when the previous process died are finished: they release their reservation
        # and are reported as infrastructure failures by the runner.
        for run in sorted(self.active):
            self._append({"kind": "release", "run": run, "reason": "recovered after restart"})
        self.active.clear()

    def _append(self, entry):
        entry = dict(entry, ts=time.time())
        line = json.dumps(entry, sort_keys=True) + "\n"
        fd = os.open(self.ledger_path, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)
        try:
            os.write(fd, line.encode())
            os.fsync(fd)
        finally:
            os.close(fd)

    # Admission ---------------------------------------------------------------------------------------------
    def _outstanding(self):
        return sum(max(0.0, self.ceiling - self.run_spent.get(r, 0.0)) for r in self.active)

    def admit(self, run):
        with self._lock:
            if self.stopped:
                raise BudgetRefused("experiment_stopped", self.stopped)
            if run in self.run_spent:
                raise BudgetRefused("duplicate_run", run)
            headroom = self.cap - self.spent - self._outstanding()
            if headroom < self.ceiling:
                self._append({"kind": "refuse_admit", "run": run, "headroom_usd": headroom})
                raise BudgetRefused("experiment_cap", f"headroom ${headroom:.2f} < run ceiling ${self.ceiling:.2f}")
            self.active.add(run)
            self.run_spent[run] = 0.0
            self.run_tools[run] = 0
            self._append({"kind": "admit", "run": run})

    def release(self, run, reason):
        with self._lock:
            if run in self.active:
                self.active.discard(run)
                self._append({"kind": "release", "run": run, "reason": reason})

    def stop(self, reason):
        with self._lock:
            if not self.stopped:
                self.stopped = reason
                self._append({"kind": "stop", "reason": reason})

    # Calls -------------------------------------------------------------------------------------------------
    def reserve(self, run, worst_usd, settle_wait_s=0.0):
        """Reserve the worst case of one call. Returns a token for settle(), or raises BudgetRefused.

        A CLI can send its next request a moment before the proxy has settled the previous one (the response is
        streamed first, then charged). So a reservation first waits, up to settle_wait_s, for the run's in-flight
        calls to settle; the limits are then checked against settled tool counts. If the wait times out, in-flight
        reservations still count at their worst case, so the dollar limits hold either way."""
        with self._lock:
            self._settled.wait_for(lambda: self.run_pending.get(run, 0.0) <= 1e-12, timeout=settle_wait_s)
            if self.stopped:
                raise BudgetRefused("experiment_stopped", self.stopped)
            if run not in self.active:
                raise BudgetRefused("run_not_admitted", run)
            if self.run_tools.get(run, 0) >= self.max_tool_calls:
                raise BudgetRefused("tool_calls", f"{self.run_tools[run]} of {self.max_tool_calls} used")
            pending = self.run_pending.get(run, 0.0)
            if self.run_spent[run] + pending + worst_usd > self.ceiling:
                raise BudgetRefused("run_ceiling", f"spent ${self.run_spent[run]:.4f} + in flight ${pending:.4f} "
                                                   f"+ worst ${worst_usd:.4f} > ${self.ceiling:.2f}")
            all_pending = sum(self.run_pending.values())
            if self.spent + all_pending + worst_usd > self.cap:
                raise BudgetRefused("experiment_cap", f"spent ${self.spent:.4f} + worst ${worst_usd:.4f} "
                                                      f"> ${self.cap:.2f}")
            self.run_pending[run] = pending + worst_usd
            self._next_reservation += 1
            token = self._next_reservation
            self._reservations[token] = (run, worst_usd)
            return token

    def settle(self, token, cost_usd, tool_calls, record):
        """Record a finished call. The caller passes the actual cost (the worst case if usage was missing)."""
        with self._lock:
            run, worst = self._reservations.pop(token)
            self.run_pending[run] = max(0.0, self.run_pending.get(run, 0.0) - worst)
            self.spent += cost_usd
            self.run_spent[run] = self.run_spent.get(run, 0.0) + cost_usd
            self.run_tools[run] = self.run_tools.get(run, 0) + tool_calls
            self._append(dict(record, kind="call", run=run, cost_usd=cost_usd, worst_usd=worst,
                              tool_calls=tool_calls))
            self._settled.notify_all()
            exceeded = cost_usd > worst + 1e-9
        if exceeded:
            self.stop(f"call cost ${cost_usd:.4f} exceeded its reserved worst case ${worst:.4f}")
        return exceeded

    def note(self, run, entry):
        with self._lock:
            self._append(dict(entry, run=run))

    def snapshot(self, run=None):
        with self._lock:
            out = {"spent_usd": round(self.spent, 6), "cap_usd": self.cap, "stopped": self.stopped}
            if run is not None:
                out.update(run_spent_usd=round(self.run_spent.get(run, 0.0), 6),
                           run_tool_calls=self.run_tools.get(run, 0))
            return out
