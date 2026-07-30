import re

def process_file(filename, is_js):
    with open(filename, "r") as f:
        content = f.read()

    # The exact blocks to replace in `generateStatementRaw`:
    # 1. let
    let_idx = content.find('} else if head == "let" {')
    if let_idx != -1:
        end_idx = content.find('} else if head == "try_let" {')
        content = content[:let_idx] + content[end_idx:]

    # 2. try_let
    tl_idx = content.find('} else if head == "try_let" {')
    if tl_idx != -1:
        if is_js:
            end_idx = content.find('} else if head == "dom_query" {')
        else:
            end_idx = content.find('} else if head == "spawn" {')
        content = content[:tl_idx] + content[end_idx:]

    # 3. spawn
    sp_idx = content.find('} else if head == "spawn" {')
    if sp_idx != -1:
        end_idx = content.find('} else if head == "spawn_agent" {')
        content = content[:sp_idx] + content[end_idx:]

    # 4. for
    for_idx = content.find('} else if head == "for" {')
    if for_idx != -1:
        if is_js:
            end_idx = content.find('} else if head == "call" {')
        else:
            end_idx = content.find('} else if head == "read_file" {')
        content = content[:for_idx] + content[end_idx:]

    # 5. call
    call_idx = content.find('} else if head == "call" {')
    if call_idx != -1:
        if is_js:
            end_idx = content.find('} else if head == "spawn" {')
        else:
            end_idx = content.find('} else if head == "cli_args" {')
        content = content[:call_idx] + content[end_idx:]
    
    with open(filename, "w") as f:
        f.write(content)

process_file("internal/backend/gogen/gogen.go", False)
process_file("internal/backend/javascript/javascript.go", True)
