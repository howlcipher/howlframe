# Change Log

## Unreleased

### Added

* Bounded, opt-in auto-patching for the observer. It validates a model-proposed
  `.zero` replacement in an isolated project copy, installs it atomically only
  after the configured tests pass, and then runs an explicit restart command.
* Isolated unit and subprocess integration coverage for path confinement,
  malformed model responses, test failure, atomic installation, and restart
  ordering.
