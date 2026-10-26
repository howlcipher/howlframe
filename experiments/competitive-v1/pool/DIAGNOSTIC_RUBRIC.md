# Diagnostic Quality Rubric: First Error Message Scoring

This rubric is used by blind evaluators to score the quality and diagnostic helpfulness of the first compiler, build, or runtime error message encountered by an agent during task execution. Language names, compiler brands, and toolchain identifiers are scrubbed or hidden where possible before grading.

The scale ranges from 1 (unusable / misleading) to 5 (exemplary diagnostic).

---

## Evaluation Criteria Dimensions

Every diagnostic is evaluated against four observable, objective criteria:
1. **Source Location**: Does the diagnostic accurately identify the source file, line number, and column or token offset?
2. **Discrepancy Clarity (Expected vs. Found)**: Does it explicitly state both what the system expected and what it actually encountered?
3. **Accuracy / Ground Truth**: Is the stated error cause factually correct, or is it a downstream artifact of a different root cause?
4. **Actionable Remediation / Fix Suggestion**: Does it offer a clear, context-aware suggestion or hint for resolving the error?

---

## Scoring Scale (1 to 5)

### Score 1 — Obscure, Misleading, or Non-Localizing
- **Criteria**:
  - Fails to identify the relevant source file or line number (e.g., bare process crash, segmentation fault, generic exit code, or points to internal compiler runtime memory).
  - Or, provides a fundamentally incorrect reason that misleads the developer about where or what the fault is.
  - No indication of expected versus found.
  - Zero actionable guidance.
- **Invented Pseudo-Language Example**:
  ```text
  FATAL: Process terminated with code 139 (SIGSEGV) at memory address 0x00007fff89ab10c0
  ```

### Score 2 — Minimal Location, Vague Error Description
- **Criteria**:
  - Names a file and approximate line number, but fails to isolate the specific token or expression.
  - Uses generic or opaque phrasing (e.g., "syntax error", "parse failure", "invalid operation") without stating what was expected or what was found.
  - Does not suggest any fix.
  - Usually technically accurate at the failure point, but provides little explanatory context.
- **Invented Pseudo-Language Example**:
  ```text
  src/service.pseudo:42: parse error
  ```

### Score 3 — Accurate Location with Problem Statement (Missing One Dimension)
- **Criteria**:
  - Identifies the exact file, line, and token or expression.
  - Accurately states what went wrong (e.g., type mismatch, unknown symbol, unclosed delimiter).
  - Explicitly states *either* what was found or what was expected, but lacks full bidirectional contrast (expected vs. found), OR lacks any actionable hint/suggestion.
  - Completely accurate and truthful regarding the immediate syntax/type rule violated.
- **Invented Pseudo-Language Example**:
  ```text
  src/service.pseudo:42:15: error: type mismatch in variable assignment
      found expression of type 'String', but variable 'timeout_ms' requires 'Integer'
  ```

### Score 4 — Comprehensive Diagnostic (Location, Discrepancy, and Context)
- **Criteria**:
  - Identifies the exact file, line, column, and displays the source snippet with an underline/caret highlighting the faulty token.
  - Explicitly states both what was expected and what was found.
  - Truthful and accurate root cause identification.
  - May provide a generic hint or context note, but lacks a tailored, drop-in replacement or specific suggested fix.
- **Invented Pseudo-Language Example**:
  ```text
  src/service.pseudo:42:15: error: unmatched argument type in function call
      42 |   connect(host, port, timeout_ms)
         |                       ^^^^^^^^^^
      expected: Integer (milliseconds)
      found:    String ("5000")
      note: 'connect' declared at src/net.pseudo:12:1
  ```

### Score 5 — Exemplary Diagnostic (Precise, Clear Discrepancy, Context, and Actionable Fix)
- **Criteria**:
  - Identifies file, line, column, and underlines/quotes the exact problematic code snippet in context.
  - Explicitly and unambiguously states both what was expected and what was found.
  - Diagnostically accurate without misleading cascading error artifacts.
  - Provides a specific, actionable, context-aware suggestion or fix (e.g., "did you mean...", "consider converting with...", or a code replacement proposal).
- **Invented Pseudo-Language Example**:
  ```text
  src/service.pseudo:42:15: error: argument 3 ('timeout') has incompatible type
      42 |   connect(host, port, "5000")
         |                       ^^^^^^
      expected: Integer
      found:    String
      help: parse the string to integer before passing, e.g.:
      42 |   connect(host, port, Integer.parse("5000"))
  ```

---

## Tie-Break and Edge-Case Rules

1. **Cascading / Downstream Errors**: If a compiler emits multiple diagnostics from a single syntactic or semantic root cause, score **only the first diagnostic**. If the first diagnostic is an artifact of a prior undetected typo, cap the score at **2** because it misdiagnoses the true root cause.
2. **Ambiguous Score Boundary (e.g., between 3 and 4)**:
   - If the diagnostic highlights the offending code snippet with carets/underlines, award **4**.
   - If it only prints text coordinates `file:line:col` without showing the line text, award **3**.
3. **Ambiguous Score Boundary (e.g., between 4 and 5)**:
   - To receive a **5**, the suggestion must be syntactically valid and directly applicable to the local context. If a suggestion is generic boilerplate (e.g., "refer to documentation") or inapplicable, the score remains **4**.
4. **Runtime Exceptions vs. Compile Errors**:
   - Runtime stack traces that provide file and line coordinates, exception class, and descriptive message with parameter values start at baseline **3**.
   - If the runtime error includes inspection of the offending operand (e.g., `AttributeError: 'NoneType' object has no attribute 'status', expected object with attribute 'status'`), award **4**.
   - If it suggests the likely remedy (e.g., null-check or initializing call), award **5**.
   - An unhandled crash dumping raw core memory or minidump data without stack frames scores **1**.
