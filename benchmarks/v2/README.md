# HowlFrame Benchmark v2

This directory contains Benchmark v2, a suite designed to measure how efficiently an AI or coding agent can arrive at verified, working software in HowlFrame compared with mainstream languages (Go, Python, JavaScript/Node).

## What it measures

This benchmark tracks the "AI Write Cost". It measures the effort (time, tokens, repairs) required for an LLM or Agent to produce a **correct, compiler/runtime-verified** solution.

### Tracks

The benchmark is split into two tracks to distinguish raw generation quality from autonomous agent capability:

* **Track A — Language generation benchmark**: A fixed task prompt is given to a model. The model generates source code in a single turn. The code is then built/validated/tested. This evaluates how easy the language is for a model to produce correctly on the first try.
* **Track B — Agentic development benchmark**: A fixed task is given to an autonomous agent. The agent edits files, runs builds/tests, diagnoses failures, and repairs the code iteratively until a verified solution is reached. This evaluates the total work required for an AI coding agent to finish the task.

### Metrics Collected

See [`schema.json`](schema.json) for the full JSON schema of a benchmark result. Core metrics include:

1. **First-pass success/failure**: Did the model get it right on the first generation?
2. **Eventual success/failure**: Did it eventually pass?
3. **Repair attempts**: How many fix cycles were needed?
4. **Total generated tokens**: The output token cost of generation + all repairs.
5. **Final source tokens**: Token count of the final source artifact (cl100k_base proxy).
6. **Elapsed time**: Wall-clock seconds from prompt to verified artifact.
7. **Verification commands run**: Number of builds/tests executed.
8. **Code churn**: Lines changed during repairs.
9. **Verification result**: pass/fail.
10. **Runtime/backend used**: HowlFrame VM, Go 1.22, Node.js 20, etc.

## Tasks

The benchmark includes 8 representative tasks:

1. **`task_01_typed_function`**: A small typed function (`add(a, b)`) with a unit test.
2. **`task_02_cli_file_processing`**: A file-processing CLI tool that reads names and prints greetings.
3. **`task_03_http_json_endpoint`**: An HTTP/JSON endpoint.
4. **`task_04_multi_module_cli`**: A multi-module CLI, testing import/export mechanics.
5. **`task_05_list_dict_transform`**: List/dictionary transformation (map/reduce/filter logic).
6. **`task_06_error_handling`**: Structured error handling (testing try/catch or result types).
7. **`task_07_native_store`**: A stateful task interacting with a native key-value store (where supported).
8. **`task_08_capability_restricted`**: A capability-restricted application (testing capability-denial behavior).

## How to run

Currently, the benchmark provides the harness schema and tasks. 
To run a manual or semi-automated Track A evaluation:

1. Pick a task (e.g. `task_01_typed_function`).
2. Give the LLM the prompt defined in `tasks/<task>/prompt.md`.
3. Save the output to `tasks/<task>/<language>/solution.*`.
4. Run the language's native verification command (e.g. `go test`, `node test.js`, `howlframe run solution.howl`).
5. Record the metrics matching `schema.json` into a JSON file, representing one trial.

### Repeated Trials

Do not draw strong conclusions from a single run. AI outputs are non-deterministic.
To get credible results, you should run **5–10 independent trials** per task per language.
Report statistics like:
* Success rate (e.g. 80% first-pass success)
* Median elapsed time / token count
* p90 elapsed time
* Min/max repair counts

## Limitations and threats to validity

- Model familiarity: Models like GPT-4 and Claude have seen billions of lines of Python and Go, but very little HowlFrame. This benchmark explicitly tests if HowlFrame's constrained, regular design overcomes that inherent familiarity deficit.
- LLM non-determinism: A model might randomly output a perfect solution or get stuck in a syntax loop. Repeated trials are essential.
- Tokenizer bias: Using `cl100k_base` may slightly favor or penalize certain syntaxes depending on how they align with BPE token boundaries.

## Historical Data

Benchmark v1 (which measured just tasks A, B, and C manually) is retained in `benchmarks/language_write_cost` and documented in `docs/language_write_cost_benchmark.md`.
Do not present reused historical timings as fresh independent measurements for Benchmark v2.
