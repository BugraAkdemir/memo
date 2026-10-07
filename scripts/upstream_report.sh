#!/usr/bin/env bash
# Turn the upstream watch's report into GitHub issues (called by
# .github/workflows/upstream.yml; DRY_RUN=1 prints the gh commands instead).
#
#   scripts/upstream_report.sh REPORT.md TEST_EXIT_CODE
#
# - DRIFT lines (a vendor/endpoint/release moved)  -> ONE open issue labelled
#   `upstream-drift`, created or its body replaced with the latest report; closed
#   again with a comment when a later run is clean.
# - NOTICE lines (a newer CLIProxyAPI exists)      -> one issue per version,
#   labelled `upstream-notice`, never duplicated.
# - A failing test run WITHOUT any DRIFT line means the watch itself broke (compile
#   error, runner problem): exit non-zero so the workflow goes red instead of
#   silently reporting a clean bill of health.
# Inconclusive probes (rate limits, bot challenges) never open anything by
# themselves; they only appear in the issue body when a real drift does.
set -euo pipefail

REPORT="${1:?usage: upstream_report.sh REPORT.md TEST_EXIT_CODE}"
RC="${2:?missing test exit code}"
GH="gh"; [ "${DRY_RUN:-}" = 1 ] && GH="echo gh"
run() { if [ "${DRY_RUN:-}" = 1 ]; then echo "+ $*"; else "$@"; fi; }

touch "$REPORT"
drift="$(grep -F '**DRIFT**' "$REPORT" || true)"
notices="$(grep -F '**NOTICE**' "$REPORT" || true)"
others="$(grep -vF -e '**DRIFT**' -e '**NOTICE**' "$REPORT" || true)"

run_url="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-owner/repo}/actions/runs/${GITHUB_RUN_ID:-0}"
mkdir -p "${RUNNER_TEMP:-/tmp}"
body="${RUNNER_TEMP:-/tmp}/upstream_body.md"

ensure_label() { run gh label create "$1" --color "$2" --description "$3" 2>/dev/null || true; }
open_issue() { # title-prefix label -> number or empty
  if [ "${DRY_RUN:-}" = 1 ]; then echo ""; return; fi
  gh issue list --label "$2" --state open --search "in:title \"$1\"" --json number --jq '.[0].number // empty'
}

if [ -n "$drift" ]; then
  {
    echo "The scheduled upstream watch found something Memo was not built against. Details of the latest run: $run_url"
    echo
    echo "### Drift"
    echo "$drift"
    if [ -n "$others" ]; then echo; echo "### Other results (not alarming on their own)"; echo "$others"; fi
    echo
    echo "_This issue is updated by each run and closes itself when a run is clean._"
  } > "$body"
  ensure_label upstream-drift B60205 "A vendor endpoint, release or download host moved"
  n="$(open_issue "Upstream drift detected" upstream-drift)"
  if [ -n "$n" ]; then run gh issue edit "$n" --body-file "$body"
  else run gh issue create --title "Upstream drift detected" --label upstream-drift --body-file "$body"; fi
else
  n="$(open_issue "Upstream drift detected" upstream-drift)"
  if [ -n "$n" ]; then run gh issue close "$n" --comment "No drift in the latest run ($run_url)."; fi
fi

if [ -n "$notices" ]; then
  ensure_label upstream-notice 0E8A16 "A newer upstream release is available"
  while IFS= read -r line; do
    ver="$(printf '%s' "$line" | grep -oE 'CLIProxyAPI v[0-9][0-9A-Za-z.\-]* is out' | head -1 | awk '{print $2}')"
    title="CLIProxyAPI ${ver:-update} available"
    n="$(open_issue "$title" upstream-notice)"
    if [ -z "$n" ]; then
      printf '%s\n\nFound by the scheduled upstream watch: %s\n' "$line" "$run_url" > "$body"
      run gh issue create --title "$title" --label upstream-notice --body-file "$body"
    fi
  done <<< "$notices"
fi

# The test run failed but not because of drift: the watch is broken.
if [ "$RC" != 0 ] && [ -z "$drift" ]; then
  echo "::error title=Upstream watch::the probe run failed without reporting any drift — the watch itself is broken (see the log)" >&2
  exit 1
fi
exit 0
