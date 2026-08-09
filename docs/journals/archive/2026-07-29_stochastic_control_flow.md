# Implementation Journal

Task: Implement Stochastic Control Flow (Item 41)
Date: 2026-07-29

Features to implement:
- Add a `confidence` node in the AST that evaluates conditions to probability distributions.
- Enables syntax like `(if (> (confidence (is_fraud tx)) 0.95) ...)`.

Next Step: Implement `confidence` ast node parsing and codegen in `howlframe.go`.
