#!/usr/bin/env python3
import os
import json
import argparse
from collections import defaultdict

def main():
    parser = argparse.ArgumentParser(description="Aggregate Benchmark v2 Results")
    parser.add_argument("--results-dir", default="benchmarks/v2/results", help="Path to results directory")
    args = parser.parse_args()

    results = []
    if not os.path.exists(args.results_dir):
        print(f"Directory {args.results_dir} does not exist.")
        return

    for filename in os.listdir(args.results_dir):
        if filename.endswith(".json"):
            path = os.path.join(args.results_dir, filename)
            try:
                with open(path, 'r') as f:
                    results.append(json.load(f))
            except Exception as e:
                print(f"Failed to read {path}: {e}")

    if not results:
        print("No results found.")
        return

    # Group by language and task
    grouped = defaultdict(list)
    for r in results:
        lang = r.get("language", "unknown")
        task = r.get("task", "unknown")
        grouped[(lang, task)].append(r)

    print("# Benchmark v2 Aggregated Results\n")
    print("| Language | Task | Track | Model | Success Rate | Avg Elapsed (s) | Avg Final Tokens |")
    print("|---|---|---|---|---|---|---|")

    for (lang, task), trials in sorted(grouped.items()):
        success_count = sum(1 for t in trials if t.get("metrics", {}).get("eventual_success", False))
        success_rate = (success_count / len(trials)) * 100
        
        avg_elapsed = sum(t.get("metrics", {}).get("elapsed_time_seconds", 0) for t in trials) / len(trials)
        avg_tokens = sum(t.get("metrics", {}).get("final_source_tokens", 0) for t in trials) / len(trials)
        
        # Assume same track and model for now per group
        track = trials[0].get("track", "A")
        model = trials[0].get("model", "unknown")

        print(f"| {lang} | {task} | {track} | {model} | {success_rate:.1f}% ({success_count}/{len(trials)}) | {avg_elapsed:.2f} | {avg_tokens:.0f} |")

if __name__ == "__main__":
    main()
