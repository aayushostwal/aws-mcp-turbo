"""Reproducible tokenizer measurements; synthetic fixtures, no AWS requests."""
import argparse
import json
import subprocess
import sys

import tiktoken

parser = argparse.ArgumentParser()
parser.add_argument("--check-schema", action="store_true", help="fail if any measured schema exceeds 499 tokens")
parser.add_argument("--compare", metavar="RESULTS", help="summarize a scripts/bench-compare.mjs transcript instead of the fixtures")
parser.add_argument("--check-savings", action="store_true", help="fail if mean fixture reduction is below the 85%% target")
args = parser.parse_args()

if args.compare:
    encoder = tiktoken.get_encoding("cl100k_base")
    count = lambda s: len(encoder.encode(s))
    runs = json.load(open(args.compare))
    print("Server  tool-definition tokens")
    for run in runs:
        print(f'{run["label"]:7} {count(json.dumps(run["tools"], separators=(",", ":"))):6}')
    print("\nTask                 " + "".join(f'{r["label"]:>8} calls  tokens   ms  ' for r in runs))
    totals = [[0, 0, 0] for _ in runs]
    for task_id in runs[0]["tasks"]:
        row = f"{task_id:20} "
        for i, run in enumerate(runs):
            task = run["tasks"].get(task_id, {})
            if "skipped" in task or "calls" not in task:
                row += f'{"skipped":>16}{"":>14}'
                continue
            tokens = sum(count(json.dumps(c["arguments"], separators=(",", ":"))) + count(c["response"]) for c in task["calls"])
            ms = sum(c["ms"] for c in task["calls"])
            totals[i][0] += len(task["calls"]); totals[i][1] += tokens; totals[i][2] += ms
            row += f'{len(task["calls"]):>16}{tokens:>8}{ms:>6}  '
        print(row)
    print("TOTAL                " + "".join(f"{t[0]:>16}{t[1]:>8}{t[2]:>6}  " for t in totals))
    sys.exit(0)

schema = subprocess.check_output(["go", "run", "./cmd/aws-mcp-turbo", "--print-tool-schema"], text=True).strip()
measurements = json.loads(subprocess.check_output(["go", "run", "./cmd/measure"], text=True))
failed = False
for name in ("cl100k_base", "o200k_base"):
    encoder = tiktoken.get_encoding(name)
    schema_tokens = len(encoder.encode(schema))
    print(f"\n{name}: complete tool definitions = {schema_tokens} tokens")
    print("Action                              Raw   Tool  Reduction")
    reductions = []
    for item in measurements:
        raw = len(encoder.encode(item["raw"]))
        compressed = len(encoder.encode(item["compressed"]))
        reduction = 100 * (1 - compressed / raw)
        reductions.append(reduction)
        print(f'{item["action"]:34} {raw:5} {compressed:6} {reduction:8.1f}%')
    mean = sum(reductions) / len(reductions)
    print(f"Unweighted mean reduction: {mean:.1f}% (synthetic fixtures; target 85%)")
    failed |= args.check_schema and schema_tokens >= 500
    failed |= args.check_savings and mean < 85
sys.exit(1 if failed else 0)
