#!/usr/bin/env bash
# Live acceptance for the Go CLI against a THROWAWAY repo it creates and
# deletes under the token's own user. Only repo-scoped verbs from the
# allowlist below are exercised — never notification/user/org/admin or
# anything instance-scoped (those are covered by httptest unit tests).
# Usage: scripts/acceptance.sh [go-binary]
set -euo pipefail

BIN="${1:-bin/forgejo}"
SUFFIX="$(date +%s)-$$"
NAME="forgejo-cli-acceptance-$SUFFIX"

echo "== auth =="
"$BIN" auth status

LOGIN=$("$BIN" api /user --jq .login 2>/dev/null || "$BIN" api /user | jq -r .login)
REPO="$LOGIN/$NAME"
echo "== throwaway repo: $REPO =="

# shellcheck disable=SC2329  # invoked via trap
cleanup() {
    "$BIN" repo delete "$REPO" --yes >/dev/null 2>&1 || true
}
trap cleanup EXIT

"$BIN" api POST /user/repos -f name="$NAME" -F auto_init=true >/dev/null
sleep 1

fail=0
step() {
    local desc="$1"; shift
    if "$@" >/dev/null 2>&1; then
        echo "PASS $desc"
    else
        echo "FAIL $desc: $*"
        fail=1
    fi
}

step "repo view"            "$BIN" repo view "$REPO"
step "repo edit"            "$BIN" repo edit "$REPO" --desc="acceptance run"
step "repo topic add"       "$BIN" repo topic add "$REPO" --topics=acceptance
step "repo topic list"      "$BIN" repo topic list "$REPO"

step "issue create"         "$BIN" issue create "$REPO" --title="acc issue" --body="body text"
step "issue list"           "$BIN" issue list "$REPO"
step "issue view"           "$BIN" issue view "$REPO" 1
step "issue comment"        "$BIN" issue comment "$REPO" 1 --body="a comment"
step "label create"         "$BIN" issue label "$REPO" create --name=acc --color="#00aabb" --scope=repo
step "issue label add"      "$BIN" issue label "$REPO" add 1 --labels=acc
step "issue close"          "$BIN" issue close "$REPO" 1
step "issue reopen"         "$BIN" issue reopen "$REPO" 1

step "milestone create"     "$BIN" issue milestone create "$REPO" --title=m1
step "milestone list"       "$BIN" issue milestone list "$REPO"

step "branch list"          "$BIN" branch list "$REPO"
step "wiki create"          "$BIN" wiki create "$REPO" --title="Home" --content="hello"
step "wiki view"            "$BIN" wiki view "$REPO" Home

step "tag create"           "$BIN" repo tags create "$REPO" --tag=v0.0.1
step "release create"       "$BIN" release create "$REPO" --tag=v0.0.1 --title="v0.0.1"
step "release list"         "$BIN" release list "$REPO"

# PR flow: create a branch + file via api, then PR verbs.
step "branch create"        "$BIN" branch create "$REPO" acc-branch --from=main
step "api file put"         "$BIN" api POST "/repos/$REPO/contents/acc.txt" \
                                -f content="$(printf 'acceptance' | base64)" \
                                -f message="add acc.txt" -f branch=acc-branch
step "pr create"            "$BIN" pr create "$REPO" --title="acc pr" --head=acc-branch --base=main --body="pr body"
step "pr list"              "$BIN" pr list "$REPO"
step "pr view"              "$BIN" pr view "$REPO" 2
step "pr diff"              "$BIN" pr diff "$REPO" 2
step "pr comment create"    "$BIN" pr comment "$REPO" 2 --body="pr comment"
step "pr merge"             "$BIN" pr merge "$REPO" 2 --method=squash

step "dry-run blocks write" bash -c "! $BIN --dry-run issue create '$REPO' --title=x 2>/dev/null | grep -q ."
step "repo delete --yes"    "$BIN" repo delete "$REPO" --yes
trap - EXIT

echo
if [[ $fail -eq 0 ]]; then
    echo "acceptance: ALL PASS"
else
    echo "acceptance: FAILURES above"
fi
exit $fail
