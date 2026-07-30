import re

with open("internal/backend/gogen/gogen.go", "r") as f:
    content = f.read()

# We need to extract the "else if head == "let" {" block
def extract_block(text, start_pattern):
    idx = text.find(start_pattern)
    if idx == -1: return ""
    brace_count = 0
    in_block = False
    for i in range(idx, len(text)):
        if text[i] == '{':
            brace_count += 1
            in_block = True
        elif text[i] == '}':
            brace_count -= 1
        if in_block and brace_count == 0:
            return text[idx:i+1]
    return ""

print("--- LET ---")
print(extract_block(content, 'else if head == "let" {'))
print("--- TRY_LET ---")
print(extract_block(content, 'else if head == "try_let" {'))
print("--- SPAWN ---")
print(extract_block(content, 'else if head == "spawn" {'))
print("--- CALL ---")
print(extract_block(content, 'else if head == "call" {'))
print("--- FOR ---")
print(extract_block(content, 'else if head == "for" {'))

