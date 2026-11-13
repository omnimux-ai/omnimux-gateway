#!/usr/bin/env bash
# check-core-change-gate.sh — block unapproved edits under core business paths.
#
# Usage:
#   scripts/ops/check-core-change-gate.sh staged
#   scripts/ops/check-core-change-gate.sh range <base> <head>
#   scripts/ops/check-core-change-gate.sh files <path>...
#
# Allow when protected paths are touched only if:
#   - Core-Change-Approved: <scope> appears in the relevant commit message(s), or
#   - OMNIMUX_CORE_CHANGE_APPROVED=1 (local staged mode; still require trailer on commit for CI), or
#   - range mode: each commit that touches core is an upstream-sync merge or carries the trailer.
#
# Exit 0 = pass, 1 = blocked, 2 = usage/error.

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "${ROOT}" ]; then
  echo "check-core-change-gate: not inside a git work tree" >&2
  exit 2
fi

PATHS_FILE="${ROOT}/scripts/ops/core-change-paths.txt"
if [ ! -f "${PATHS_FILE}" ]; then
  echo "check-core-change-gate: missing ${PATHS_FILE}" >&2
  exit 2
fi

# Load prefixes into a newline-separated string (Bash 3.2 portable; no mapfile).
CORE_PREFIXES="$(
  sed -e 's/#.*//' -e 's/[[:space:]]*$//' -e '/^$/d' "${PATHS_FILE}" | sed 's|/*$|/|'
)"

is_core_path() {
  f="${1#./}"
  echo "${CORE_PREFIXES}" | while IFS= read -r p; do
    [ -z "${p}" ] && continue
    case "${f}" in
      "${p}"*) exit 0 ;;
    esac
  done
  return 1
}

# Note: is_core_path uses a pipeline subshell for the loop; exit 0 inside only exits the
# subshell. Use a pure shell loop instead:
is_core_path() {
  f="${1#./}"
  old_ifs="${IFS}"
  IFS='
'
  for p in ${CORE_PREFIXES}; do
    [ -z "${p}" ] && continue
    case "${f}" in
      "${p}"*)
        IFS="${old_ifs}"
        return 0
        ;;
    esac
  done
  IFS="${old_ifs}"
  return 1
}

filter_core_files() {
  while IFS= read -r f; do
    [ -z "${f}" ] && continue
    if is_core_path "${f}"; then
      printf '%s\n' "${f}"
    fi
  done
}

is_upstream_sync_msg() {
  msg="$1"
  echo "${msg}" | grep -Eiq \
    '(Merge (remote-tracking )?branch .*upstream|Merge pull request .*sync/upstream|sync/upstream-|upstream/main|upstream/master)'
}

has_approval_trailer() {
  msg="$1"
  echo "${msg}" | grep -Eq '^Core-Change-Approved:[[:space:]]*[^[:space:]].*'
}

mode="${1:-}"
if [ -z "${mode}" ]; then
  echo "usage: $0 staged | range <base> <head> | files <path>..." >&2
  exit 2
fi
shift

core_list=""
RANGE_BASE=""
RANGE_HEAD=""

case "${mode}" in
  staged)
    core_list="$(git diff --cached --name-only --diff-filter=ACMRD | filter_core_files | sort -u)"
    ;;
  range)
    RANGE_BASE="${1:-}"
    RANGE_HEAD="${2:-HEAD}"
    if [ -z "${RANGE_BASE}" ]; then
      echo "usage: $0 range <base> <head>" >&2
      exit 2
    fi
    core_list="$(git diff --name-only --diff-filter=ACMRD "${RANGE_BASE}...${RANGE_HEAD}" | filter_core_files | sort -u)"
    ;;
  files)
    core_list="$(printf '%s\n' "$@" | filter_core_files | sort -u)"
    ;;
  *)
    echo "usage: $0 staged | range <base> <head> | files <path>..." >&2
    exit 2
    ;;
esac

if [ -z "${core_list}" ]; then
  exit 0
fi

echo "check-core-change-gate: protected paths touched:" >&2
echo "${core_list}" | sed 's/^/  - /' >&2

approved=0

if [ "${mode}" = "staged" ]; then
  if [ "${OMNIMUX_CORE_CHANGE_APPROVED:-}" = "1" ]; then
    approved=1
    echo "check-core-change-gate: local allow via OMNIMUX_CORE_CHANGE_APPROVED=1" >&2
    echo "check-core-change-gate: commit message MUST include: Core-Change-Approved: <scope>" >&2
  fi
  # prepare-commit-msg may not have run; allow if user passed -m with trailer via env
  if [ -n "${OMNIMUX_CORE_CHANGE_SCOPE:-}" ]; then
    approved=1
    echo "check-core-change-gate: local allow via OMNIMUX_CORE_CHANGE_SCOPE" >&2
  fi
fi

if [ "${mode}" = "range" ]; then
  msgs="$(git log --format='%B%n---' "${RANGE_BASE}...${RANGE_HEAD}" 2>/dev/null || true)"
  if has_approval_trailer "${msgs}"; then
    approved=1
    echo "check-core-change-gate: found Core-Change-Approved in commit range" >&2
  else
    all_ok=1
    while IFS= read -r c; do
      [ -z "${c}" ] && continue
      cfiles="$(git diff-tree --no-commit-id --name-only -r "${c}" | filter_core_files || true)"
      if [ -z "${cfiles}" ]; then
        continue
      fi
      cmsg="$(git log -1 --format='%B' "${c}")"
      if has_approval_trailer "${cmsg}" || is_upstream_sync_msg "${cmsg}"; then
        continue
      fi
      all_ok=0
      echo "check-core-change-gate: unapproved commit ${c}: $(git log -1 --format='%s' "${c}")" >&2
    done <<EOF
$(git rev-list "${RANGE_BASE}...${RANGE_HEAD}")
EOF
    if [ "${all_ok}" -eq 1 ]; then
      approved=1
      echo "check-core-change-gate: all core-touching commits approved or upstream-sync" >&2
    fi
  fi
fi

if [ "${mode}" = "files" ] && [ "${OMNIMUX_CORE_CHANGE_APPROVED:-}" = "1" ]; then
  approved=1
fi

if [ "${approved}" -eq 1 ]; then
  exit 0
fi

cat >&2 <<'EOF'

check-core-change-gate: BLOCKED — core business paths changed without approval.

Required (after human secondary approval for this exact scope):
  1. Put this line in the commit message:
       Core-Change-Approved: <short scope>
  2. For local pre-commit:
       OMNIMUX_CORE_CHANGE_APPROVED=1 git commit -m "fork: ...

       Core-Change-Approved: <short scope>"

Agent rules: AGENTS.md → Change policy (fork).
Do not invent Core-Change-Approved without human approval.
Do not use --no-verify to skip this gate.

EOF
exit 1
