# Comparing aws-mcp-turbo with the official AWS MCP Server

Ten read-only questions a developer asks in a normal week. Run each one against both servers
and compare time, cost in tokens, and whether the answer is right. Nothing here changes AWS state.

Use an identity with read access only (see [examples/iam-readonly.json](../examples/iam-readonly.json)),
and a non-production account or a quiet region if you can. Replace the `<placeholders>` with your own
resources. Questions that need a resource you do not have can be skipped; record "skipped".

## The ten questions

Paste each prompt as written into an agent that has **only one** of the two servers enabled.

| # | Prompt | What a good answer contains |
| --- | --- | --- |
| 1 | Which AWS account and identity am I using right now, and in which region? | Account ID, principal ARN, region. The cheapest possible call, so it mostly measures per-call overhead. |
| 2 | List my running EC2 instances in `<region-a>`, `<region-b>` and `<region-c>`: id, name and private IP. | One row per instance, grouped or labeled by region. |
| 3 | My Lambda function `<function>` seems to be timing out. What is its timeout setting and what do the last hour of logs show? | The configured timeout and any `Task timed out` lines, or a clear statement that there are none. |
| 4 | Which Lambda functions do I have, and which runtime does each use? I want to find outdated runtimes. | Every function with its runtime, across all pages. |
| 5 | Which CloudWatch alarms are in the ALARM state right now, and why? | Alarm names with their state reason. Composite alarms count. |
| 6 | Why did tasks stop recently in my ECS cluster `<cluster>`? | Stop reasons and container exit codes for recent stopped tasks. |
| 7 | Which AWS services cost the most over the last 30 days? | Cost per service, highest first. |
| 8 | Show me ERROR lines from the log group `<log-group>` in the last hour. | The matching lines with timestamps, or a clear "none". |
| 9 | Who stopped EC2 instances in the last 24 hours? | Event time, user or role, and event name from CloudTrail. |
| 10 | Which security groups allow SSH (port 22) from anywhere (0.0.0.0/0)? | Group ID, name and VPC for each, or a clear "none". |

Questions 2, 4, 5, 10 test paging and filtering. Questions 3, 6, 7 are multi-step investigations
that the turbo server answers with one recipe and the official server answers with a script.
Questions 8 and 9 test log and audit reads.

## What to record

For each question and server:

| Measure | How |
| --- | --- |
| Wall time | From sending the prompt to the final answer, or the sum of tool-call times if you time only the tools. |
| Tool calls | Count the calls the agent made, including failed ones. |
| Tokens | Input plus output tokens for the whole turn, from the client or the API usage report. |
| Correct | Compare against the AWS console or CLI: complete, nothing invented, nothing missing. |
| Notes | Retries, errors, truncated output, anything surprising. |

Run each question at least three times per server and report the median. The first run on either
server includes credential and connection warm-up, so discard it or report it separately.

## Result template

Copy this table into your results file or an issue. Fill it with your numbers.

| # | Server | Time (s) | Tool calls | Tokens | Correct | Notes |
| --- | --- | ---: | ---: | ---: | :---: | --- |
| 1 | official | | | | | |
| 1 | turbo | | | | | |
| 2 | official | | | | | |
| 2 | turbo | | | | | |
| … | | | | | | |

## Scripted run

The same ten questions are in [bench/tasks.json](../bench/tasks.json) as fixed tool calls, so the
comparison does not depend on a model. This measures the smallest request each server needs; it
does not measure how many attempts a real agent makes.

```sh
node scripts/bench-compare.mjs --turbo ./bin/aws-mcp-turbo \
  --other "uvx mcp-proxy-for-aws-cli@latest https://aws-mcp.us-east-1.api.aws/mcp --metadata AWS_REGION=us-west-2" \
  --var REGION_A=us-east-1 --var REGION_B=us-west-2 --var REGION_C=eu-west-1 \
  --var FUNCTION=<function> --var CLUSTER=<cluster> --var LOG_GROUP=<log-group> \
  --out bench/results.json
python3 scripts/token_metrics.py --compare bench/results.json
```

The script prints tool-definition tokens for each server, then calls, tokens and milliseconds per
question. The argument name for the official server's script tool (`script`) is not documented by AWS;
check it against that server's tool list and edit `bench/tasks.json` if it differs.

## Keeping the comparison fair

- Same identity, region, account and moment for both servers.
- Same model and client. Enable one server at a time, so each one's tool list is the only difference.
- Do not hand-tune prompts per server. If a server needs a hint, record that as a note.
- Publish the raw transcripts (`bench/results.json`) with any numbers you share.
- Results depend on your account. An account with 3 instances and one with 3,000 will not agree.
