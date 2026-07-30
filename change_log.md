# Change Log

## Unreleased

### Added

* OpenAI model recommendations across the bug and improvement backlogs,
  matching the existing Claude/Gemini routing columns with GPT-5.6 Luna,
  Terra, and Sol tiers.
* Bounded, opt-in auto-patching for the observer. It validates a model-proposed
  `.zero` replacement in an isolated project copy, installs it atomically only
  after the configured tests pass, and then runs an explicit restart command.
* Isolated unit and subprocess integration coverage for path confinement,
  malformed model responses, test failure, atomic installation, and restart
  ordering.

### Changed

* Refreshed the README and GitHub Pages content to reflect the current
  semantic checker, typed backend metadata, direct binary bytecode, and expanded
  WebAssembly Text backend scope.
* Re-scored the remaining self-healing backlog after auto-patching shipped,
  corrected the WebAssembly prototype's WAT scope, and restored the missing
  Ephemeral Neural Circuits detail section.
