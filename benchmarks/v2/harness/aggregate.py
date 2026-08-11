#!/usr/bin/env python3
import os
import json
import argparse
from collections import defaultdict

def median(lst):
    if not lst: return None
    lst = sorted(lst)
    n = len(lst)
    if n % 2 == 1: return lst[n//2]
    return (lst[n//2 - 1] + lst[n//2]) / 2.0

def p90(lst):
    if len(lst) < 10: return "n/a"
    lst = sorted(lst)
    idx = int(0.9 * len(lst))
    return lst[idx]

def main():
    parser = argparse.ArgumentParser(description="Aggregate Benchmark v2 Results")
    parser.add_argument("--results-dir", default="benchmarks/v2/results", help="Path to results directory")
    args = parser.parse_args()

    results = []
    if not os.path.exists(args.results_dir):
        print(f"Directory {args.results_dir} does not exist.")
        return

    for root, _, files in os.walk(args.results_dir):
        for filename in files:
            if filename.endswith(".json"):
                path = os.path.join(root, filename)
                try:
                    with open(path, 'r') as f:
                        results.append(json.load(f))
                except Exception as e:
                    print(f"Failed to read {path}: {e}")

    if not results:
        print("No results found.")
        return

    grouped = defaultdict(list)
    for r in results:
        model = r.get("model", "unknown")
        lang = r.get("language", "unknown")
        task = r.get("task", "unknown")
        track = r.get("track", "unknown")
        grouped[(model, lang, task, track)].append(r)

    print("# Benchmark v2 Aggregated Results\n")
    print("| Model | Language | Task | Track | Trials | 1st Pass % | Eventual % | Med. Repairs (Min/Max) | Med. Time (p90) | Med. Tokens | Gen Tokens | Notes |")
    print("|---|---|---|---|---|---|---|---|---|---|---|---|")

    unsupported = []

    for (model, lang, task, track), trials in sorted(grouped.items()):
        if any(t.get("notes") == "unsupported" for t in trials):
            unsupported.append((model, lang, task, track))
            continue
            
        trial_count = len(trials)
        first_pass_count = sum(1 for t in trials if t.get("metrics", {}).get("first_pass_success", False))
        eventual_count = sum(1 for t in trials if t.get("metrics", {}).get("eventual_success", False))
        
        repairs = [t.get("metrics", {}).get("repair_attempts", 0) for t in trials]
        times = [t.get("metrics", {}).get("elapsed_time_seconds", 0.0) for t in trials]
        final_tokens = [t.get("metrics", {}).get("final_source_tokens", 0) for t in trials]
        
        gen_tokens_raw = [t.get("metrics", {}).get("total_generated_tokens") for t in trials]
        gen_tokens = [x for x in gen_tokens_raw if x is not None]
        
        med_rep = median(repairs)
        min_rep, max_rep = (min(repairs), max(repairs)) if repairs else (0, 0)
        med_time = median(times)
        p90_time = p90(times)
        p90_str = f"{p90_time:.2f}" if isinstance(p90_time, float) else p90_time
        
        med_final = median(final_tokens)
        med_gen = median(gen_tokens) if gen_tokens else "n/a"
        
        fp_rate = (first_pass_count / trial_count) * 100
        ev_rate = (eventual_count / trial_count) * 100
        
        notes = []
        if task == "task_08_capability_restricted":
            cap_success = sum(1 for t in trials if t.get("metrics", {}).get("capability_denial_behavior") == "Gracefully caught Access Denied")
            notes.append(f"Cap Denied: {cap_success}/{trial_count}")

        note_str = ", ".join(notes) if notes else "-"

        print(f"| {model} | {lang} | {task} | {track} | {trial_count} | {fp_rate:.1f}% | {ev_rate:.1f}% | {med_rep} ({min_rep}/{max_rep}) | {med_time:.1f} ({p90_str}) | {med_final} | {med_gen} | {note_str} |")

    if unsupported:
        print("\n## Unsupported Combinations\n")
        print("| Model | Language | Task | Track |")
        print("|---|---|---|---|")
        for (model, lang, task, track) in unsupported:
            print(f"| {model} | {lang} | {task} | {track} |")

if __name__ == "__main__":
    main()
