import re
import os

def process_file(filename, prefix, emit_func):
    with open(filename, "r") as f:
        content = f.read()

    blocks_to_remove = ["let", "try_let", "spawn", "call", "for"]
    extracted = {}
    
    for b in blocks_to_remove:
        start_str = f'else if head == "{b}" {{'
        idx = content.find(start_str)
        if idx == -1:
            start_str = f'\tif head == "{b}" {{'
            idx = content.find(start_str)
            if idx == -1:
                print(f"Could not find block {b} in {filename}")
                continue
        
        brace_count = 0
        in_block = False
        end_idx = -1
        for i in range(idx, len(content)):
            if content[i] == '{':
                brace_count += 1
                in_block = True
            elif content[i] == '}':
                brace_count -= 1
            
            if in_block and brace_count == 0:
                end_idx = i + 1
                break
        
        block_content = content[idx:end_idx]
        extracted[b] = block_content
        content = content[:idx] + content[end_idx:]

    adapted_cases = []
    for b in blocks_to_remove:
        if b not in extracted: continue
        block = extracted[b]
        lines = block.split('\n')
        inner = '\n'.join(lines[1:-1])
        
        # Remove validation logic since it's now in ir.go
        if 'ast.ReportError(' in inner:
            lines = inner.split('\n')
            new_inner = []
            skip = False
            for line in lines:
                if 'ast.ReportError(' in line:
                    skip = True
                    # If this is inside an if block, we probably want to remove the if block
                    continue
                if skip and line.strip() == '}':
                    skip = False
                    continue
                if not skip and not 'if len(node.Children)' in line:
                    new_inner.append(line)
            inner = '\n'.join(new_inner)

        if b == "let":
            inner = inner.replace('curr := node', 'curr := &ast.Node{Type: "List", Children: []*ast.Node{{Type: "SYMBOL", Value: "let"}, ir.Kids[0], ir.Kids[1]}}')
        elif b == "try_let":
            inner = inner.replace('binds := node.Children[1]', 'binds := ir.Kids[0]')
            inner = inner.replace('catchNode := node.Children[2]', 'catchNode := ir.Kids[1]')
            inner = inner.replace('node.Children[3]', 'ir.Kids[2]')
        elif b == "spawn":
            inner = inner.replace('lambdaNode := node.Children[1]', 'lambdaNode := ir.Kids[0]')
        elif b == "call":
            inner = inner.replace('funcName := node.Children[1].Value', 'funcName := ir.Kids[0].Value')
            inner = re.sub(r'for j := 2; j < len\(node\.Children\); j\+\+ \{[\s\S]*?argNode := node\.Children\[j\]', 
                           r'for j := 1; j < len(ir.Kids); j++ {\n\t\t\targNode := ir.Kids[j]', inner)
        elif b == "for":
            inner = inner.replace('itemNode := node.Children[1].Value', 'itemNode := ir.Kids[0].Value')
            inner = inner.replace('listNode := node.Children[2].Value', 'listNode := ir.Kids[1].Value')
            inner = inner.replace('node.Children[3]', 'ir.Kids[2]')

        # Ensure node.Line is fallback handled if they somehow use it
        inner = inner.replace('node.Line', '0')
        inner = inner.replace('node.Column', '0')
            
        adapted_cases.append(f'\tcase "{b}":\n{inner}')

    new_cases_str = '\n'.join(adapted_cases)
    
    idx = content.find('\tcase "return":')
    if idx != -1:
        content = content[:idx] + new_cases_str + "\n" + content[idx:]
    else:
        print(f"Could not find case return in {filename}")

    # clean up any leftover empty "else " bits if the blocks were part of an if-else chain
    content = content.replace("} else \n", "}\n")

    with open(filename, "w") as f:
        f.write(content)

process_file("internal/backend/gogen/gogen.go", "generate", "EmitGoIR")
process_file("internal/backend/javascript/javascript.go", "generateJS", "EmitJSIR")

