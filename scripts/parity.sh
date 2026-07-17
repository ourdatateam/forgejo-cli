#!/usr/bin/env bash
# Parity harness: bash CLI vs Go CLI on read verbs, --json normalized with
# jq -S. Exit codes are asserted too. Usage:
#   scripts/parity.sh <owner/repo> [go-binary] [bash-script]
# Uses the caller's ~/.config/forgejo-cli/config. Read-only against the repo.
set -uo pipefail

REPO="${1:?usage: parity.sh <owner/repo> [go-binary] [bash-script]}"
GO_BIN="${2:-bin/forgejo}"
SH_BIN="${3:-./forgejo}"

pass=0 fail=0

compare() {
    local desc="$1"; shift
    local a b ra rb
    a=$("$SH_BIN" "$@" --json 2>/dev/null | jq -S . 2>/dev/null); ra=$?
    b=$("$GO_BIN" "$@" --json 2>/dev/null | jq -S . 2>/dev/null); rb=$?
    if [[ $ra -ne 0 || $rb -ne 0 ]]; then
        if [[ $ra -ne 0 && $rb -ne 0 ]]; then
            echo "PASS (both error) $desc"
            pass=$((pass+1))
        else
            echo "FAIL $desc — exit bash=$ra go=$rb"
            fail=$((fail+1))
        fi
        return
    fi
    if [[ "$a" == "$b" ]]; then
        echo "PASS $desc"
        pass=$((pass+1))
    else
        echo "FAIL $desc — normalized JSON differs:"
        diff <(echo "$a") <(echo "$b") | head -20
        fail=$((fail+1))
    fi
}

compare "repo view"        repo view "$REPO"
compare "repo tags list"   repo tags list "$REPO"
compare "issue list"       issue list "$REPO"
compare "pr list"          pr list "$REPO"
compare "branch list"      branch list "$REPO"
compare "release list"     release list "$REPO"
compare "search repos"     search repos --query="${REPO#*/}"
compare "repo view (404)"  repo view "no-such-owner-xyz/never"

echo
echo "parity: $pass passed, $fail failed"
exit $((fail > 0))
