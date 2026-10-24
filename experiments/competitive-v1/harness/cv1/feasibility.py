"""The four-run feasibility test (Revision 3 G1): one provider, one calibration task, four arms at once.

    # Infrastructure check against the local mock (no provider is contacted; costs are simulated):
    python3 -m cv1.feasibility --mock SCENARIO.json --task TASK_DIR --provider anthropic --out OUT

    # Paid run. Refused unless an Owner authorization file names this exact provider, model, cap and date:
    python3 -m cv1.feasibility --live --authorization AUTH.json --key-file KEY --task TASK_DIR --out OUT

The authorization file is written by the Owner, never by an agent:

    {"authorized_by": "Owner", "experiment": "competitive-v1 feasibility", "date": "YYYY-MM-DD",
     "valid_through": "YYYY-MM-DD", "provider": "anthropic", "model": "claude-opus-5-5",
     "cap_usd": 150, "run_ceiling_usd": 30, "task_id": "...", "statement": "..."}

The cap and ceiling come only from that file; there is no flag to raise them. The ledger lives in OUT, so a
restart continues against the same spend, and a different cap needs a new OUT and a new authorization.
"""

import argparse
import datetime
import json
import os
import sys
import threading

from . import agents, pricing, runner, scorer
from .budget import Budget

UPSTREAM = {"anthropic": "https://api.anthropic.com", "openai": "https://api.openai.com"}
MODELS = {"anthropic": "claude-opus-5-5", "openai": "gpt-6.1-sol"}
TEMPLATE = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))), "prompts",
                        "agent_task.md")
# Protocol §8 hard limits for the feasibility test. An authorization may set lower limits, never higher.
MAX_CAP_USD = 150.0
MAX_RUN_CEILING_USD = 30.0
REQUIRED_AUTH = ("authorized_by", "experiment", "date", "valid_through", "provider", "model", "cap_usd",
                 "run_ceiling_usd", "task_id", "statement")


class NotAuthorized(Exception):
    pass


def check_authorization(path, provider, task_id, today=None):
    with open(path) as f:
        auth = json.load(f)
    missing = [k for k in REQUIRED_AUTH if k not in auth]
    if missing:
        raise NotAuthorized(f"authorization lacks {missing}")
    if auth["authorized_by"] != "Owner" or auth["experiment"] != "competitive-v1 feasibility":
        raise NotAuthorized("authorization is not an Owner authorization for the feasibility test")
    today = today or datetime.date.today()
    start = datetime.date.fromisoformat(auth["date"])
    end = datetime.date.fromisoformat(auth["valid_through"])
    if not start <= today <= end:
        raise NotAuthorized(f"authorization is valid {start}..{end}, not {today}")
    if auth["provider"] != provider or auth["model"] != MODELS[provider]:
        raise NotAuthorized("authorization names a different provider or model")
    if auth["task_id"] != task_id:
        raise NotAuthorized("authorization names a different task")
    if not (0 < float(auth["run_ceiling_usd"]) <= float(auth["cap_usd"])):
        raise NotAuthorized("authorization limits are inconsistent")
    if float(auth["cap_usd"]) > MAX_CAP_USD or float(auth["run_ceiling_usd"]) > MAX_RUN_CEILING_USD:
        raise NotAuthorized(f"authorization exceeds the protocol §8 limits (${MAX_CAP_USD:g} total, "
                            f"${MAX_RUN_CEILING_USD:g} per run); raising them needs a protocol revision")
    return auth


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    mode = p.add_mutually_exclusive_group(required=True)
    mode.add_argument("--mock", metavar="SCENARIO", help="run against the local mock upstream")
    mode.add_argument("--live", action="store_true", help="contact the provider (needs --authorization)")
    p.add_argument("--authorization")
    p.add_argument("--key-file", help="provider API key, chmod 600; read by the proxy only")
    p.add_argument("--provider", default="anthropic", choices=sorted(MODELS))
    p.add_argument("--task", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--cache", default=os.environ.get("CV1_CACHE"))
    p.add_argument("--claude-bin", default=os.environ.get("CV1_CLAUDE_BIN"))
    p.add_argument("--codex-dir", default=os.environ.get("CV1_CODEX_DIR"))
    p.add_argument("--served-model", action="append", help="additional served model ID to accept (dated snapshot)")
    p.add_argument("--cpus", help="taskset CPU list applied to every arm")
    args = p.parse_args(argv)

    task = runner.Task(args.task)
    provider = args.provider
    mock_server = None
    if args.live:
        if not (args.authorization and args.key_file):
            p.error("--live needs --authorization and --key-file")
        auth = check_authorization(args.authorization, provider, task.id)
        cap, ceiling = float(auth["cap_usd"]), float(auth["run_ceiling_usd"])
        from .proxy import read_api_key
        api_key = read_api_key(args.key_file)
        upstream = UPSTREAM[provider]
    else:
        sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "mock"))
        import mock_upstream
        with open(args.mock) as f:
            scenario = json.load(f)
        mock_server = mock_upstream.make_server(scenario, os.path.join(args.out, "mock_requests.jsonl"))
        threading.Thread(target=mock_server.serve_forever, daemon=True).start()
        cap, ceiling = 150.0, 30.0
        api_key = "mock-key"
        upstream = f"http://127.0.0.1:{mock_server.server_address[1]}"

    os.makedirs(args.out, exist_ok=True)
    budget = Budget(os.path.join(args.out, "ledger.jsonl"), cap, ceiling, 150)
    layout = runner.Layout(args.cache, claude_binary=args.claude_bin, codex_dir=args.codex_dir)
    model = MODELS[provider]
    cfg = agents.AgentConfig(provider, model, effort="high",
                             max_budget_usd=ceiling if provider == "anthropic" else None)
    limits = {"memory_max": "8G", "tasks_max": 1024, "cpus": args.cpus, "use_systemd": True}
    specs = [runner.RunSpec(f"feasibility-{provider}-{arm}", task, arm, cfg, limits=limits) for arm in runner.ARMS]
    results, errors = runner.run_group(
        specs, layout=layout, budget=budget, prices=pricing.load_prices(), out_dir=args.out, upstream_url=upstream,
        api_key=api_key, template_path=TEMPLATE, allowed_served=[model] + (args.served_model or []))
    summary = {"mode": "live" if args.live else "mock", "provider": provider, "model": model, "task": task.id,
               "cap_usd": cap, "run_ceiling_usd": ceiling, "spent_usd": budget.snapshot()["spent_usd"],
               "surface_mismatch_arms": runner.audit_surfaces(results), "runs": [], "errors": errors,
               "note": "Infrastructure check and budget calibration only. Not evidence for or against any hypothesis."}
    frozen = runner.tree_manifest(task.acceptance)
    for r in results:
        if r is None:
            continue
        s = scorer.score_run(os.path.join(args.out, r["run_id"]), task, layout, frozen)
        summary["runs"].append({"arm": r["arm"], "status": r["status"], "outcome": s["outcome"],
                                "spend_usd": r["spend_usd"], "elapsed_s": r["elapsed_s"],
                                "model_calls": r["model_calls"], "tool_calls": r["tool_calls"]})
    with open(os.path.join(args.out, "feasibility_summary.json"), "w") as f:
        json.dump(summary, f, indent=2)
    print(json.dumps(summary, indent=2))
    if mock_server:
        mock_server.shutdown()
    return 0 if not any(errors) else 1


if __name__ == "__main__":
    sys.exit(main())
