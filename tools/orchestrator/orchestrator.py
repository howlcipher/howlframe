import subprocess
import json
import sys
from pathlib import Path

import outlines
from outlines import models

from orchestrator_schema import BytecodeProgram


SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent.parent


def main():
    print("Initializing Outlines...")

    # Using local Ollama via OpenAI compatibility layer
    # Note: Ensure you have `openai` and `outlines` installed and Ollama running.
    # Replace "llama3" with your local model's name (e.g., "phi3").
    try:
        from openai import OpenAI
        client = OpenAI(base_url="http://localhost:11434/v1", api_key="ollama")
        model = models.openai(client, "llama3")

        # Alternatively, if you want guaranteed strict decoding via llama-cpp-python:
        # model = models.llamacpp("path/to/model.gguf")
    except Exception as e:
        print(f"Failed to load model. Error: {e}")
        sys.exit(1)

    print("Compiling JSON schema generator...")
    generator = outlines.generate.json(model, BytecodeProgram)

    # Load the Zero AI System Prompt to provide context on the language
    system_context = ""
    try:
        system_context = (
            (SCRIPT_DIR / "ai_prompt.md").read_text(encoding="utf-8")
            + "\n\n"
        )
    except FileNotFoundError:
        print(
            "Warning: ai_prompt.md not found. "
            "The model may struggle without language context."
        )

    user_goal = "Calculate the sum of 10 and 20, store it in a variable, and print it."
    prompt = system_context + "Goal:\n" + user_goal
    max_retries = 3
    current_prompt = prompt

    for attempt in range(max_retries):
        print(f"\n--- Attempt {attempt + 1} ---")
        print(f"Prompt: {current_prompt}")
        print("Generating Zero code... (waiting for local model response)")

        # Generate JSON bytecode
        code = generator(current_prompt)
        print(f"Generated Code:\n{code}\n")

        bytecode_file = REPO_ROOT / "app.json"
        bytecode_file.write_text(code, encoding="utf-8")

        print("Running VM...")
        result = subprocess.run(
            ["go", "run", "zero.go", "-run-bc", str(bytecode_file)],
            cwd=REPO_ROOT,
            capture_output=True,
            text=True,
            check=False,
        )

        if result.returncode != 0:
            output = result.stdout.strip() or result.stderr.strip()
            try:
                err_data = json.loads(output)
                print(
                    "VM error detected at line "
                    f"{err_data.get('line')}, column "
                    f"{err_data.get('column')}: {err_data.get('reason')}"
                )

                # Feedback loop
                current_prompt = (
                    f"{prompt}\n\nYour previous output caused a VM error:\n"
                    f"{json.dumps(err_data)}\nPlease fix the JSON bytecode."
                )
            except json.JSONDecodeError:
                print("Failed to parse JSON error from Go VM. Unexpected output:")
                print(output)
                break
        else:
            print("Execution successful!")
            print(result.stdout)
            break

if __name__ == "__main__":
    main()
