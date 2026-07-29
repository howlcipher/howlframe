import outlines
from outlines import models
import subprocess
import json
import sys
from orchestrator_schema import BytecodeProgram

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
        with open("AI_PROMPT.md", "r") as f:
            system_context = f.read() + "\n\n"
    except FileNotFoundError:
        print("Warning: AI_PROMPT.md not found. The model may struggle without language context.")

    user_goal = "Calculate the sum of 10 and 20, store it in a variable, and print it."
    prompt = system_context + "Goal:\n" + user_goal
    max_retries = 3
    current_prompt = prompt

    for attempt in range(max_retries):
        print(f"\n--- Attempt {attempt+1} ---")
        print(f"Prompt: {current_prompt}")
        print("Generating Zero code... (waiting for local model response)")
        
        # Generate JSON bytecode
        code = generator(current_prompt)
        print(f"Generated Code:\n{code}\n")
        
        with open("app.json", "w") as f:
            f.write(code)
            
        print("Running VM...")
        result = subprocess.run(["go", "run", "zero.go", "-run-bc", "app.json"], capture_output=True, text=True)
        
        if result.returncode != 0:
            output = result.stdout.strip() or result.stderr.strip()
            try:
                err_data = json.loads(output)
                print(f"VM error detected at line {err_data.get('line')}, column {err_data.get('column')}: {err_data.get('reason')}")
                
                # Feedback loop
                current_prompt = f"{prompt}\n\nYour previous output caused a VM error:\n{json.dumps(err_data)}\nPlease fix the JSON bytecode."
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
