# Policy-Enforced Subprocess Sandbox Runner

Implement a capability-enforcing command runner that evaluates role-based execution requests against security policies before spawning subprocesses.

## Invocation

```bash
./run <policy.json> <requests.json> <audit.json>
```

- `<policy.json>`: Path to policy configuration file defining roles, allowed binaries, forbidden arguments, and execution jail boundaries.
- `<requests.json>`: Path to batch execution requests.
- `<audit.json>`: Path to output audit log recording authorization decisions, security violations, and execution outcomes.

## Policy Specification

`<policy.json>` has the following structure:

```json
{
  "jail_root": "/path/to/jail",
  "roles": {
    "reporter": {
      "allowed_binaries": ["/usr/bin/coreutils", "/usr/bin/dash"],
      "disallowed_args": [";", "&", "|", ".."],
      "allow_env": ["LANG", "LC_ALL"],
      "max_timeout_ms": 3000,
      "max_output_bytes": 4096
    }
  }
}
```

- `jail_root` (string, optional): Base directory boundary. If defined, any task execution `working_dir` must resolve inside (or match) this directory.
- `roles` (object, required): Map of role names to permission configurations:
  - `allowed_binaries` (array of strings, required): List of exact binary paths permitted for execution.
  - `disallowed_args` (array of strings, optional, default: `[]`): Substrings forbidden from appearing anywhere within any command argument.
  - `allow_env` (array of strings, optional, default: `[]`): Whitelist of environment variable names permitted to pass to the subprocess.
  - `max_timeout_ms` (integer, optional, default: 5000): Maximum allowable timeout ceiling for this role.
  - `max_output_bytes` (integer, optional, default: 1048576): Maximum allowable combined output (stdout) bytes.

## Requests Specification

`<requests.json>` has the following structure:

```json
{
  "requests": [
    {
      "id": "req-1",
      "role": "reporter",
      "command": ["/usr/bin/coreutils", "echo", "hello world"],
      "working_dir": "/path/to/jail/workspace",
      "env": {
        "LANG": "C.UTF-8",
        "UNAUTHORIZED_VAR": "secret"
      },
      "timeout_ms": 1000
    }
  ]
}
```

Fields:
- `id` (string, required): Request identifier.
- `role` (string, required): Assigned role requesting execution.
- `command` (array of strings, required): Executable and arguments.
- `working_dir` (string, optional, default: current directory): Target working directory.
- `env` (object, optional, default: `{}`): Environment variables requested by the caller.
- `timeout_ms` (integer, optional, default: 5000): Requested timeout in milliseconds.

## Capability & Authorization Evaluation Rules

Before executing any request:
1. **Role Existence**: The requested `role` must be present in `policy.roles`. If absent, deny with `decision: "deny"`, `reason: "unknown_role"`.
2. **Binary Whitelist**: `command[0]` must be present in `role.allowed_binaries`. If not, deny with `decision: "deny"`, `reason: "binary_not_permitted"`.
3. **Disallowed Argument Check**: If any argument in `command` contains any substring listed in `role.disallowed_args`, deny with `decision: "deny"`, `reason: "forbidden_argument"`.
4. **Jail Boundary Enforcement**: If `jail_root` is specified:
   - Resolve the canonical absolute path of `working_dir` (resolving any `.` or `..`).
   - If `working_dir` does not start with `jail_root` (or is not inside `jail_root`), deny with `decision: "deny"`, `reason: "jail_violation"`.
5. **Environment Sanitization**:
   - The subprocess environment must ONLY contain variables explicitly listed in `role.allow_env`.
   - Take values only from `request.env` matching `role.allow_env`. Do not inherit arbitrary parent process environment variables except minimal standard execution requirements (`PATH`).
6. **Execution Limits**:
   - Effective timeout = `min(request.timeout_ms, role.max_timeout_ms)`.
   - If execution exceeds effective timeout, terminate the process: `status: "timed_out"`, `exit_code: null`, `timed_out: true`.
   - If stdout exceeds `role.max_output_bytes`, truncate stdout to `role.max_output_bytes` and set `truncated: true`.

## Output Audit Log Specification

`<audit.json>` must contain:

```json
{
  "total_requests": 2,
  "allowed_and_executed": 1,
  "denied": 1,
  "executions": [
    {
      "id": "req-1",
      "decision": "allow",
      "status": "success",
      "exit_code": 0,
      "stdout": "hello world\n",
      "stderr": "",
      "timed_out": false,
      "truncated": false
    },
    {
      "id": "req-2",
      "decision": "deny",
      "reason": "binary_not_permitted"
    }
  ]
}
```

Fields:
- `total_requests`: Total count of requests in input.
- `allowed_and_executed`: Count of requests with `decision: "allow"`.
- `denied`: Count of requests with `decision: "deny"`.
- `executions`: List of result objects in the order of requests.

## Exit Codes

- `0`: All requests were successfully evaluated (denials are logged cleanly as security decisions) and all executed commands succeeded (exit code 0).
- `1`: One or more executed commands exited with a non-zero exit code or timed out.
- `2`: Malformed JSON input, missing arguments, or invalid policy structure.
