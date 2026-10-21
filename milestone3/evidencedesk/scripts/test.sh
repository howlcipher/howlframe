#!/usr/bin/env bash
set -euo pipefail
app=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
"$app/scripts/build.sh"
python3 -u "$app/tests/verify_workflows.py"
