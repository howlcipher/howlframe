# Work Next Item

When invoked, perform the following steps:

1. **Analyze Backlogs:** Read both `bugs.md` and `improvements.md`.
2. **Find Highest ROI:** Identify the `Pending` item with the highest `Score (V×D÷E)` across both files. Bugs and Improvements compete on the same scale.
3. **Execute:** Follow the Working Protocol defined in the backlogs to implement the fix or feature. Use a task journal in `docs/journals/` if the task is complex.
4. **Discover:** If any new bugs or necessary improvements are discovered during execution, document them in the appropriate backlog (`bugs.md` or `improvements.md`) with a calculated score.
5. **Complete:** Once the item is fully tested and verified, update its Status in the backlog to `Done (YYYY-MM-DD)`. Fill in the model columns if applicable.
6. **Commit:** Commit the changes to the repository with a clear commit message referencing the issue number.

*Note: Ensure all build artifacts are kept out of the repository root, as per the Working Protocol.*
