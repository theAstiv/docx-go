#!/usr/bin/env bash
# benchgate.sh — fail on a significant benchmark regression.
#
# Usage: bash scripts/benchgate.sh baseline.txt new.txt
#
# Runs benchstat and fails when a benchmark got slower by more than
# MAX_TIME_PCT percent (sec/op) or allocates more than MAX_ALLOC_PCT percent
# more (allocs/op), and benchstat finds the change significant (it prints
# "~" instead of a delta when it is not). Allocation counts are
# deterministic, so their threshold is tighter than the time one, which
# shared CI runners make noisy.

set -euo pipefail

MAX_TIME_PCT="${MAX_TIME_PCT:-20}"
MAX_ALLOC_PCT="${MAX_ALLOC_PCT:-10}"

benchstat "$1" "$2"
echo
benchstat -format csv "$1" "$2" | awk -F, -v maxt="$MAX_TIME_PCT" -v maxa="$MAX_ALLOC_PCT" '
    # Header lines start with a comma; the second names the unit.
    /^,/ { unit = $2; next }
    $1 == "geomean" || $6 !~ /^\+[0-9.]+%$/ { next }
    {
        pct = substr($6, 2); sub(/%/, "", pct)
        max = (unit == "sec/op") ? maxt : (unit == "allocs/op") ? maxa : -1
        if (max >= 0 && pct + 0 > max) {
            printf "regression: %s %s %s (%s, limit +%s%%)\n", $1, unit, $6, $7, max
            bad = 1
        }
    }
    END { exit bad }
'
