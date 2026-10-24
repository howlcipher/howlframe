"""Black-box test runner used in every arm, by the agent (run-acceptance-tests) and by the scorer.

Tests are stdlib `unittest` modules named `test_*.py` in the test directory. They exercise the program only
through `./run` in the workspace (arguments, stdin, stdout, exit code, files, or HTTP on loopback), using the
helpers in `testkit.py`, so the same tests apply to every language. The runner prints a short summary and, with
--report, writes a JSON report the scorer reads.
"""

import argparse
import json
import os
import sys
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))


class _Collector(unittest.TextTestResult):
    def __init__(self, *a, **kw):
        super().__init__(*a, **kw)
        self.outcomes = []
        self._t0 = {}

    def startTest(self, test):
        self._t0[test.id()] = time.monotonic()
        super().startTest(test)

    def _record(self, test, status, detail=None):
        self.outcomes.append({"id": test.id(), "status": status,
                              "seconds": round(time.monotonic() - self._t0.get(test.id(), time.monotonic()), 3),
                              "detail": detail})

    def addSuccess(self, test):
        super().addSuccess(test)
        self._record(test, "pass")

    def addFailure(self, test, err):
        super().addFailure(test, err)
        self._record(test, "fail", self._exc_info_to_string(err, test)[-2000:])

    def addError(self, test, err):
        super().addError(test, err)
        self._record(test, "error", self._exc_info_to_string(err, test)[-2000:])

    def addSkip(self, test, reason):
        super().addSkip(test, reason)
        self._record(test, "skip", reason)


def run(tests_dir, app, report=None, pattern="test_*.py", verbosity=1):
    os.environ["CV1_APP"] = app
    os.environ["CV1_WORKSPACE"] = os.path.dirname(app)
    suite = unittest.defaultTestLoader.discover(tests_dir, pattern=pattern, top_level_dir=tests_dir)
    runner = unittest.TextTestRunner(resultclass=_Collector, verbosity=verbosity, stream=sys.stdout)
    result = runner.run(suite)
    summary = {
        "tests_dir": tests_dir, "total": result.testsRun,
        "passed": sum(1 for o in result.outcomes if o["status"] == "pass"),
        "failed": sum(1 for o in result.outcomes if o["status"] in ("fail", "error")),
        "skipped": sum(1 for o in result.outcomes if o["status"] == "skip"),
        "outcomes": result.outcomes,
    }
    summary["all_passed"] = summary["total"] > 0 and summary["failed"] == 0 and summary["skipped"] == 0
    if report:
        with open(report, "w") as f:
            json.dump(summary, f, indent=2)
    return summary


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--tests", required=True)
    p.add_argument("--app", required=True)
    p.add_argument("--report")
    p.add_argument("--pattern", default="test_*.py")
    args = p.parse_args(argv)
    s = run(args.tests, args.app, args.report, args.pattern)
    print(f"\nacceptance: {s['passed']} passed, {s['failed']} failed, {s['skipped']} skipped of {s['total']}")
    return 0 if s["all_passed"] else 1


if __name__ == "__main__":
    sys.exit(main())
