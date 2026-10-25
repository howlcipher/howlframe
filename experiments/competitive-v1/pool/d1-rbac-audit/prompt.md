# Cloud IAM Access Control and Privilege Escalation Auditor

Build an infrastructure security CLI tool that audits Identity and Access Management (IAM) role permissions against security boundaries, detects toxic capability combinations capable of privilege escalation, and reports access violations.

## CLI Interface

The executable is located at `./run`. It must implement the `audit-iam` subcommand:

```bash
./run audit-iam --iam <iam_json> --boundaries <boundaries_json> --out <report_json>
```

Arguments:
- `audit-iam`: Required subcommand. If absent or different, exit with code `1`.
- `--iam <iam_json>`: Path to JSON file containing IAM role definitions. Required.
- `--boundaries <boundaries_json>`: Path to JSON configuration specifying security boundaries. Required.
- `--out <report_json>`: Path where the audit report will be written. Required.

Options may be passed in any order.

### Exit Codes
- `0`: Execution succeeded and the IAM configuration is clean (zero findings).
- `2`: Execution succeeded but one or more boundary or privilege escalation findings were detected (`clean: false`).
- `1`: Invalid CLI flags, unrecognized subcommand, unreadable input files, or malformed JSON.

## Input File Formats

### 1. Boundaries Configuration (`--boundaries`)
A JSON file defining forbidden actions, escalation pairs, and session duration caps:
```json
{
  "max_session_duration_hours": 12,
  "forbidden_actions": ["iam:DeleteAccount", "billing:*"],
  "privilege_escalation_pairs": [
    ["iam:PassRole", "lambda:CreateFunction"],
    ["iam:CreateAccessKey", "iam:UpdateAssumeRolePolicy"]
  ]
}
```

### 2. IAM Roles Definition (`--iam`)
A JSON file containing an array of roles:
```json
{
  "roles": [
    {
      "role_name": "DevSecOps",
      "assigned_users": ["alice", "bob"],
      "session_duration_hours": 8,
      "permissions": [
        "s3:GetObject",
        "s3:PutObject",
        "iam:PassRole",
        "lambda:CreateFunction"
      ]
    }
  ]
}
```

## Auditing and Analysis Rules

Inspect each role in `roles`:
1. **Forbidden Actions Enforcement**:
   - For every action in `role.permissions`:
     - Compare against each pattern in `boundaries.forbidden_actions`.
     - An action matches if it is an exact string match, OR if the pattern ends with `*` and the action begins with the prefix preceding `*` (e.g. `billing:*` matches `billing:ViewInvoice`).
     - If matched, record a finding:
       ```json
       {
         "role_name": "<role_name>",
         "finding_type": "forbidden_action",
         "severity": "CRITICAL",
         "detail": "Role grants forbidden action '<action>'"
       }
       ```
2. **Toxic Privilege Escalation Detection**:
   - For each pair `[actA, actB]` in `boundaries.privilege_escalation_pairs`:
     - If both `actA` and `actB` are present in `role.permissions`:
       record a finding:
       ```json
       {
         "role_name": "<role_name>",
         "finding_type": "privilege_escalation",
         "severity": "HIGH",
         "detail": "Role contains toxic escalation pair: <actA> + <actB>"
       }
       ```
3. **Session Duration Bounds**:
   - If `role.session_duration_hours > boundaries.max_session_duration_hours`:
     record a finding:
     ```json
     {
       "role_name": "<role_name>",
       "finding_type": "excessive_session",
       "severity": "MEDIUM",
       "detail": "Session duration <session_duration_hours>h exceeds limit <max_session_duration_hours>h"
     }
     ```
4. **User Effective Roles Mapping**:
   - For every user in `role.assigned_users`, add `role.role_name` to their role list.
   - For each user, the role list must be unique and sorted alphabetically.
   - The user keys in `user_effective_roles` must be sorted ascending by username (code point order).

## Output Report (`--out`)

Write a JSON object to `<report_json>`:

```json
{
  "clean": false,
  "roles_audited": 1,
  "findings_count": 1,
  "findings": [
    {
      "role_name": "DevSecOps",
      "severity": "HIGH",
      "finding_type": "privilege_escalation",
      "detail": "Role contains toxic escalation pair: iam:PassRole + lambda:CreateFunction"
    }
  ],
  "user_effective_roles": {
    "alice": ["DevSecOps"],
    "bob": ["DevSecOps"]
  }
}
```

Ordering rules:
- `clean` is `true` if `findings_count == 0`, otherwise `false`.
- Sort `findings` primarily by `role_name` ascending, secondarily by `severity` rank (`CRITICAL` before `HIGH` before `MEDIUM`), tertiarily by `finding_type` ascending, and finally by `detail` ascending.
