# SBOM Capability and License Authority Policy Verifier

Build a security verification CLI tool that analyzes a Software Bill of Materials (SBOM), evaluates requested package system capabilities and software licenses against a strict organizational access-control policy, and flags unauthorized privilege escalation.

## CLI Interface

The executable is located at `./run`. It must implement the `verify-sbom` subcommand:

```bash
./run verify-sbom --sbom <sbom_json> --policy <policy_json> --report <report_json>
```

Arguments:
- `verify-sbom`: Required subcommand. If absent or different, exit with code `1`.
- `--sbom <sbom_json>`: Path to the SBOM input file. Required.
- `--policy <policy_json>`: Path to the capability and license policy file. Required.
- `--report <report_json>`: Path where the verification audit report will be written. Required.

Options may be passed in any order.

### Exit Codes
- `0`: Execution succeeded and the SBOM is fully compliant with the security policy (zero violations).
- `2`: Execution succeeded but one or more capability or license policy violations were detected (`compliant: false`).
- `1`: Invalid CLI flags, unrecognized subcommand, unreadable input files, or malformed JSON.

## Input File Formats

### 1. Security Policy File (`--policy`)
A JSON file defining trust tiers, allowed capabilities, permitted licenses, and tier assignments:
```json
{
  "allowed_licenses": ["MIT", "Apache-2.0", "BSD-3-Clause", "ISC"],
  "tier_capabilities": {
    "tier_1_core": ["network", "filesystem_read", "filesystem_write", "process_spawn"],
    "tier_2_plugin": ["filesystem_read", "network"],
    "tier_3_untrusted": []
  },
  "package_tiers": {
    "auth-module": "tier_1_core",
    "logger-lib": "tier_2_plugin"
  },
  "default_tier": "tier_3_untrusted"
}
```

### 2. SBOM File (`--sbom`)
A JSON object listing declared dependencies and their requested system privileges:
```json
{
  "application": "payment-service",
  "packages": [
    {
      "name": "auth-module",
      "version": "1.0.0",
      "license": "Apache-2.0",
      "requested_capabilities": ["filesystem_read", "network"]
    },
    {
      "name": "untrusted-helper",
      "version": "0.1.2",
      "license": "GPL-3.0",
      "requested_capabilities": ["filesystem_write"]
    }
  ]
}
```

## Policy Evaluation Rules

For each package declared in the SBOM `packages` array:
1. **Tier Assignment**:
   - Determine the package's tier from `policy.package_tiers[package.name]`.
   - If not explicitly mapped, assign `policy.default_tier`.
2. **Capability Authority Enforcement**:
   - Look up the permitted capabilities for the assigned tier in `policy.tier_capabilities`.
   - For every capability in `package.requested_capabilities`:
     - If the capability is NOT in the permitted list for that tier, record a violation:
       ```json
       {
         "package": "<package.name>",
         "violation_type": "unauthorized_capability",
         "detail": "Capability '<capability>' not permitted for tier '<tier>'"
       }
       ```
   - Calculate `effective_capabilities` for the package as the sorted list (alphabetical/code-point ascending) of requested capabilities that ARE permitted for that tier.
3. **License Authorization**:
   - Check if `package.license` is included in `policy.allowed_licenses`.
   - If NOT permitted, record a violation:
     ```json
     {
       "package": "<package.name>",
       "violation_type": "disallowed_license",
       "detail": "License '<license>' is not permitted"
     }
     ```

## Output Format (`--report`)

Write a JSON report to `<report_json>`:

```json
{
  "compliant": false,
  "total_packages": 2,
  "violations_count": 2,
  "violations": [
    {
      "package": "untrusted-helper",
      "violation_type": "disallowed_license",
      "detail": "License 'GPL-3.0' is not permitted"
    },
    {
      "package": "untrusted-helper",
      "violation_type": "unauthorized_capability",
      "detail": "Capability 'filesystem_write' not permitted for tier 'tier_3_untrusted'"
    }
  ],
  "effective_capabilities": {
    "auth-module": ["filesystem_read", "network"],
    "untrusted-helper": []
  }
}
```

Formatting rules:
- `compliant` is `true` if `violations_count == 0`, otherwise `false`.
- `violations` must be sorted by `package` ascending, then `violation_type` ascending, then `detail` ascending.
- The keys in `effective_capabilities` must be sorted ascending by package name.
