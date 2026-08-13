# Contributing to HowlFrame

HowlFrame is in an experimental, evidence-driven development phase (post-v0.1).

## Post-v0.1 Development Principle

> After v0.1, HowlFrame prioritizes correctness, user-reported friction, real use cases, and evidence from dogfooding before expanding the language or runtime surface.

Before submitting new features, please open an issue describing:
- What actual problem this solves.
- Who needs it.
- What evidence says it is the bottleneck.

## Process
1. Check the [issues](https://github.com/howlcipher/howlframe/issues) and [improvements.md](improvements.md) / [bugs.md](bugs.md) backlogs.
2. Discuss architectural changes in an issue before writing code.
3. Submit a Pull Request targeting the `main` branch.
4. Ensure all tests (`go test ./...`) and validations (`howlframe check`) pass.

Keep PRs focused and coherent. We prefer small, incremental improvements over large sweeping rewrites.
