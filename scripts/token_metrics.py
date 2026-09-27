"""Reproducible tokenizer measurements; synthetic fixtures, no AWS requests."""
import argparse
import json
import subprocess
import sys

import tiktoken

parser = argparse.ArgumentParser()
parser.add_argument("--check-schema", action="store_true", help="fail if any measured schema exceeds 499 tokens")
parser.add_argument("--check-savings", action="store_true", help="fail if mean fixture reduction is below the 85% target")
args = parser.parse_args()

schema = subprocess.check_output(["go", "run", "./cmd/aws-mcp-turbo", "--print-tool-schema"], text=True).strip()
measurements = json.loads(subprocess.check_output(["go", "run", "./cmd/measure"], text=True))
failed = False
for name in ("cl100k_base", "o200k_base"):
    encoder = tiktoken.get_encoding(name)
    schema_tokens = len(encoder.encode(schema))
    print(f"\n{name}: complete four-tool definitions = {schema_tokens} tokens")
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
