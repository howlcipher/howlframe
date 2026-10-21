#!/usr/bin/env bash
set -euo pipefail
app=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
compiler=${HOWLFRAME_BIN:-$app/build/howlframe}
if (( $# < 2 || $# > 3 )); then echo 'usage: project.sh INPUT EMPTY_OUTPUT_DIR [MAX=1000]' >&2; exit 2; fi
maximum=${3:-1000}
if [[ ! $maximum =~ ^[0-9]+$ ]]; then echo 'MAX must be integer 1..1000' >&2; exit 2; fi
# Validate the numeric value, not an arbitrary spelling length; avoid shell
# arithmetic overflow by removing leading zeroes before conversion.
while [[ ${#maximum} -gt 1 && $maximum == 0* ]]; do maximum=${maximum:1}; done
if (( ${#maximum} > 4 )) || (( 10#$maximum < 1 || 10#$maximum > 1000 )); then echo 'MAX must be integer 1..1000' >&2; exit 2; fi
if [[ ! -d $2 ]]; then echo 'OUTPUT_DIR must exist' >&2; exit 2; fi
shopt -s nullglob dotglob
files=("$2"/*)
if (( ${#files[@]} )); then echo 'OUTPUT_DIR must be empty' >&2; exit 2; fi
input=$(realpath -m -- "$1")
output=$(cd -- "$2" && pwd)
exec "$compiler" run --allow-caps "filesystem:read=$input,filesystem:read=$output,filesystem:write=$output,database" --max-instructions 10000000 "$app/build/projector.hfbc" "$input" "$output" "$((10#$maximum))"
