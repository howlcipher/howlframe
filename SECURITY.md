# Security Policy

## Experimental Status

HowlFrame is an **experimental** AI-native language and capability-bounded execution runtime. 

It does **NOT** provide or guarantee:
- OS-level sandbox isolation
- Container isolation equivalent to hypervisors/VMs
- Formal verification of all programs
- Production-grade protection against novel VM escapes

The instruction limits and capability gating are logic-level restrictions evaluated within the host process, not a strict isolation boundary provided by the operating system. Do not use HowlFrame to execute untrusted code in a sensitive production environment without proper external containerization.

## Reporting a Vulnerability

If you discover a security issue or VM escape in the bytecode evaluator, please report it privately.

Do not open a public issue. Instead, please email the maintainers directly or use GitHub's private vulnerability reporting feature for this repository if enabled. We will attempt to acknowledge and address valid capability escapes.
