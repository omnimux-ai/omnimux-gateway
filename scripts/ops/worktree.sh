#!/usr/bin/env bash
# worktree.sh — worktree lifecycle for the OmniMux fork contract.
#
# Implements root AGENTS.md "Hard bounds (fork & git)" (contract detail:
# docs/ops/worktree-zh.md):
#   primary checkout on main stays clean; product work lives in
#   <repo>/.worktrees/<task> on feat/<task>, based on latest origin/main;
#   after merge or abandon the worktree MUST be removed; MUST NOT place
#   worktrees as portfolio siblings or long-lived under /tmp.
#
# Usage:
#   worktree.sh new <task> [base]           create .worktrees/<task> on feat/<task>
#                                           from base (default origin/main)
#   worktree.sh list                        show worktrees and dirty state
#   worktree.sh ship <task>                 finish: gate -> push feat -> merge main ->
#                                           push main -> remove worktree + branch
#   worktree.sh remove <task> [--discard]   remove; --discard drops uncommitted changes,
#                        [--abandon]        --abandon drops an unmerged branch
#   worktree.sh prune                       drop stale registrations; report merged feat branches
#   worktree.sh clean                       one-shot sweep: remove worktrees + feat branches
#                                           merged into origin/main (skips dirty/unmerged; reports orphans)
#   worktree.sh help

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "${ROOT}" ]; then
  echo "worktree.sh: not inside a git work tree" >&2
  exit 2
fi

WT_DIR="${ROOT}/.worktrees"
GATE="${ROOT}/scripts/ops/check-core-change-gate.sh"
PUBLISH="${ROOT}/scripts/ops/ship-publish.sh"
MAIN="${SHIP_PUBLISH_MAIN:-main}"
UPSTREAM="${SHIP_PUBLISH_UPSTREAM:-origin}"
TASK_RE='^[a-z0-9][a-z0-9-]*$'

die() { echo "worktree.sh: $*" >&2; exit 1; }
say() { echo "==> $*"; }

validate_task() {
  local task="$1"
  if [ -z "${task}" ]; then
    die "missing <task> (kebab-case, e.g. hosted-accounts-domain)"
  fi
  case "${task}" in
    main | origin | origin/main | upstream | upstream/main | feat | merge | .git | .worktrees)
      die "reserved name: ${task}" ;;
  esac
  if ! printf '%s' "${task}" | grep -Eq "${TASK_RE}"; then
    die "task must be ASCII kebab-case (^[a-z0-9][a-z0-9-]*$): ${task}"
  fi
  if [ "${#task}" -gt 60 ]; then
    die "task too long (max 60 chars): ${task}"
  fi
}

print_wt() {
  local wt="$1" branch="$2" rel dirty
  case "${wt}" in
    "${ROOT}") rel="(primary)" ;;
    "${ROOT}"/*) rel="${wt#${ROOT}/}" ;;
    *) rel="${wt}" ;;
  esac
  dirty="$(git -C "${wt}" status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
  printf '  %-52s %-34s dirty:%s\n' "${rel}" "${branch}" "${dirty}"
}

cmd_new() {
  local task="$1"
  shift || true
  local base="${1:-origin/main}"
  local branch="feat/${task}"
  local wt="${WT_DIR}/${task}"

  validate_task "${task}"
  [ -e "${wt}" ] && die "worktree already exists: ${wt#${ROOT}/}"
  if git show-ref --verify --quiet "refs/heads/${branch}"; then
    die "branch already exists: ${branch}"
  fi

  say "fetch origin (latest base per contract)"
  git fetch origin --prune || die "git fetch origin failed"

  if ! git rev-parse --verify --quiet "${base}^{commit}" >/dev/null; then
    die "unknown base ref: ${base}"
  fi

  say "git worktree add -b ${branch} .worktrees/${task} ${base}"
  git worktree add -b "${branch}" "${wt}" "${base}" || die "git worktree add failed"

  echo "    path:   ${wt#${ROOT}/}"
  echo "    branch: ${branch}"
  echo "    base:   ${base}"
  echo "    next:   cd ${wt#${ROOT}/}   # implement, commit (hooks enforce gates), then:"
  echo "            scripts/ops/worktree.sh ship ${task}"
}

cmd_list() {
  echo "worktrees (contract: <repo>/.worktrees/<task>, removed after merge/abandon):"
  echo
  local wt="" branch=""
  while IFS= read -r line; do
    case "${line}" in
      worktree*)
        if [ -n "${wt}" ]; then print_wt "${wt}" "${branch}"; fi
        wt="${line#worktree }"
        branch=""
        ;;
      branch*)
        branch="${line#branch refs/heads/}"
        ;;
      detached)
        branch="(detached)"
        ;;
    esac
  done < <(git worktree list --porcelain)
  if [ -n "${wt}" ]; then print_wt "${wt}" "${branch}"; fi
}

cmd_ship() {
  local task="$1"
  local branch="feat/${task}"
  local wt="${WT_DIR}/${task}"

  validate_task "${task}"
  [ -d "${wt}" ] || die "no worktree for task: ${task} (expected ${wt#${ROOT}/})"
  if [ "$(git -C "${wt}" symbolic-ref --short HEAD 2>/dev/null || true)" != "${branch}" ]; then
    die "worktree ${task} is not on ${branch}; ship refuses"
  fi
  if [ -n "$(git -C "${wt}" status --porcelain)" ]; then
    die "worktree ${wt#${ROOT}/} has uncommitted changes; commit them first (or remove --discard)"
  fi

  # Contract: primary checkout on main stays clean.
  if [ "$(git -C "${ROOT}" symbolic-ref --short HEAD 2>/dev/null || true)" != "main" ]; then
    die "primary checkout is not on main: $(git -C "${ROOT}" symbolic-ref --short HEAD)"
  fi
  if [ -n "$(git -C "${ROOT}" diff --name-only)" ] || [ -n "$(git -C "${ROOT}" diff --cached --name-only)" ]; then
    die "primary checkout has tracked changes; commit/stash them first (contract: primary stays clean)"
  fi
  if [ "$(git -C "${ROOT}" ls-files --others --exclude-standard | wc -l | tr -d ' ')" -gt 0 ]; then
    say "note: primary checkout has untracked files (left as-is)"
  fi
  if [ "$(git config --get core.hooksPath 2>/dev/null || true)" != ".githooks" ]; then
    say "warning: core.hooksPath is not '.githooks'; commit gates may not be enforced (git config core.hooksPath .githooks)"
  fi

  say "publish: push ${branch}, merge ${MAIN}, push ${UPSTREAM}"
  SHIP_PUBLISH_MAX_ATTEMPTS="${SHIP_PUBLISH_MAX_ATTEMPTS:-3}" \
    bash "${PUBLISH}" "${ROOT}" "${wt}" "${branch}" \
    || die "publish failed; worktree kept for a manual push"

  say "remove worktree + branch"
  git worktree remove "${wt}" || die "git worktree remove failed; prune manually"
  if ! git branch -d "${branch}" 2>/dev/null; then
    echo "    note: branch ${branch} kept (delete with: git branch -D ${branch})" >&2
  fi
  git worktree prune

  say "done: ${branch} merged to origin/main"
}


# Session-hygiene report: worktrees that survived a finished task, and stashes.
# A worktree whose branch is already pushed but unmerged is an orphan: the
# commits are safe on origin, so the local checkout should be reclaimed.
cmd_stale() {
  git fetch origin --prune >/dev/null 2>&1 || true
  local now self primary
  now="$(date +%s)"
  self="$(git rev-parse --show-toplevel 2>/dev/null || true)"
  primary="$(git worktree list --porcelain | awk '/^worktree /{print $2; exit}')"
  local orphans=0 merged_idle=0
  local -a lines=()
  while IFS= read -r wt; do
    [ -n "${wt}" ] || continue
    [ "${wt}" = "${primary}" ] && continue
    local branch state idle last
    branch="$(git -C "${wt}" rev-parse --abbrev-ref HEAD 2>/dev/null || echo '?')"
    last="$(git -C "${wt}" log -1 --format=%ct 2>/dev/null || echo 0)"
    idle=$(( (now - last) / 86400 ))
    local counts=1
    [ "${wt}" = "${self}" ] && counts=0
    if git merge-base --is-ancestor "${branch}" origin/main 2>/dev/null; then
      state="merged"; [ "${counts}" = 1 ] && merged_idle=$((merged_idle + 1))
    elif git rev-parse --verify --quiet "origin/${branch}" >/dev/null 2>&1; then
      state="pushed-unmerged"; [ "${counts}" = 1 ] && orphans=$((orphans + 1))
    else
      state="local-only"; [ "${counts}" = 1 ] && orphans=$((orphans + 1))
    fi
    [ "${wt}" = "${self}" ] && state="${state} (current)"
    lines+=("$(printf '  %-50s %-15s idle %3sd  %s' "${wt#${ROOT}/}" "${state}" "${idle}" "${branch}")")
  done < <(git worktree list --porcelain | awk '/^worktree /{print $2}')
  local stashes
  stashes="$(git stash list | wc -l | tr -d ' ')"
  echo "worktree hygiene:"
  if [ "${#lines[@]}" -eq 0 ]; then
    echo "  (no linked worktrees)"
  else
    printf '%s\n' "${lines[@]}"
  fi
  echo "  stashes: ${stashes}"
  echo
  if [ "${orphans}" -gt 0 ]; then
    echo "  ${orphans} worktree(s) still hold unmerged branches; once the work is merged (PR) or abandoned,"
    echo "  reclaim them with: scripts/ops/worktree.sh remove <task> [--abandon]"
  fi
  if [ "${merged_idle}" -gt 0 ]; then
    echo "  ${merged_idle} merged worktree(s) can be swept with: scripts/ops/worktree.sh clean"
  fi
  if [ "${stashes}" -gt 0 ]; then
    echo "  ${stashes} stash entr(ies) are invisible to git status; cross-session work belongs on a wip/<task> branch."
  fi
  return 0
}

cmd_remove() {
  local task="$1"
  local mode="${2:-}"
  local branch="feat/${task}"
  local wt="${WT_DIR}/${task}"
  local discard=0 abandon=0 flags=""

  case "${mode}" in
    "") ;;
    --discard) discard=1 ;;
    --abandon) abandon=1 ;;
    *) die "unknown flag: ${mode} (use --discard to drop uncommitted changes, --abandon for an unmerged branch)" ;;
  esac

  validate_task "${task}"
  [ -d "${wt}" ] || die "no worktree for task: ${task} (expected ${wt#${ROOT}/})"

  if [ -n "$(git -C "${wt}" status --porcelain)" ]; then
    if [ "${discard}" -eq 1 ]; then
      say "discarding uncommitted changes in ${wt#${ROOT}/}"
    else
      die "worktree ${wt#${ROOT}/} has uncommitted changes; commit/stash them or rerun with --discard"
    fi
  fi

  git fetch origin --prune >/dev/null 2>&1 || true
  local merged=""
  if git rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
    merged="$(git for-each-ref --merged=origin/main --format='%(refname:short)' refs/heads | grep -x "${branch}" || true)"
  fi
  if [ -z "${merged}" ]; then
    if [ "${abandon}" -eq 1 ]; then
      say "branch ${branch} not merged into origin/main; removing as abandoned"
    else
      die "branch ${branch} not merged into origin/main; rerun with --abandon to abandon, or ship it"
    fi
  fi

  [ "${discard}" -eq 1 ] && flags="--force"
  say "git worktree remove ${flags} .worktrees/${task}"
  git worktree remove ${flags} "${wt}" || die "git worktree remove failed"

  if git show-ref --verify --quiet "refs/heads/${branch}"; then
    if [ "${abandon}" -eq 1 ]; then
      git branch -D "${branch}" 2>/dev/null || true
    else
      git branch -d "${branch}" 2>/dev/null || echo "    note: branch ${branch} kept" >&2
    fi
  fi
  git worktree prune
  say "done: .worktrees/${task} removed"
}

cmd_prune() {
  say "git worktree prune"
  git worktree prune
  echo
  echo "local feat/* branches merged into origin/main (safe to delete):"
  if git rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
    git for-each-ref --merged=origin/main --format='%(refname:short)' refs/heads/feat | sed 's/^/  - /'
  else
    echo "  (origin/main unknown; run: git fetch origin)"
  fi
  echo
  echo "active worktrees:"
  git worktree list
}

cmd_clean() {
  local removed=0 skipped=0 orphans=0
  local merged="" b d line wt branch registered=""

  say "fetch origin (sweep against latest origin/main)"
  if ! git fetch origin --prune 2>/dev/null; then
    say "warning: git fetch origin failed; using local origin/main (conservative)"
  fi
  if ! git rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
    die "origin/main unknown; run: git fetch origin"
  fi
  merged="$(git for-each-ref --merged=origin/main --format='%(refname:short)' refs/heads)"

  # Sweep A: registered worktrees on feat/* branches merged into origin/main.
  while IFS= read -r line; do
    case "${line}" in
      worktree*)
        if [ -n "${wt}" ]; then clean_one "${wt}" "${branch}"; fi
        wt="${line#worktree }"
        branch=""
        ;;
      branch*)
        branch="${line#branch refs/heads/}"
        ;;
      detached)
        branch=""
        ;;
    esac
  done < <(git worktree list --porcelain)
  if [ -n "${wt}" ]; then clean_one "${wt}" "${branch}"; fi

  # Sweep B: merged feat/* branches left with no worktree (checked-out branches are
  # refused by git branch -d and stay; they belong to a skipped/dirty worktree).
  while IFS= read -r b; do
    [ -z "${b}" ] && continue
    case "${b}" in
      feat/*) ;;
      *) continue ;;
    esac
    if git show-ref --verify --quiet "refs/heads/${b}" && git branch -d "${b}" >/dev/null 2>&1; then
      say "delete ${b} (merged into origin/main, no worktree)"
      removed=$((removed+1))
    fi
  done <<EOF
$(printf '%s\n' "${merged}" | grep '^feat/')
EOF

  # Orphan report: dirs under .worktrees/ not registered as worktrees (report only).
  if [ -d "${WT_DIR}" ]; then
    while IFS= read -r line; do
      case "${line}" in
        worktree*) registered="${registered}${line#worktree }
" ;;
      esac
    done < <(git worktree list --porcelain)
    for d in "${WT_DIR}"/*; do
      [ -e "${d}" ] || continue
      if ! printf '%s\n' "${registered}" | grep -Fqx "${d}"; then
        echo "  - orphan: ${d#${ROOT}/}"
        orphans=$((orphans+1))
      fi
    done
  fi

  git worktree prune
  say "clean done: removed ${removed}, skipped ${skipped}, orphan dirs ${orphans}"
}

clean_one() {
  local wt="$1" branch="$2" rel
  if [ "${wt}" = "${ROOT}" ]; then
    return 0
  fi
  rel="${wt#${ROOT}/}"
  case "${branch}" in
    feat/*) ;;
    *)
      say "skip ${rel}: not on a feat/* branch (${branch:-detached})"
      skipped=$((skipped+1))
      return 0
      ;;
  esac
  if ! printf '%s\n' "${merged}" | grep -qx "${branch}"; then
    say "skip ${rel}: ${branch} not merged into origin/main"
    skipped=$((skipped+1))
    return 0
  fi
  if [ -n "$(git -C "${wt}" status --porcelain 2>/dev/null)" ]; then
    say "skip ${rel}: uncommitted changes (use: worktree.sh remove <task> --discard)"
    skipped=$((skipped+1))
    return 0
  fi
  say "remove ${rel} + ${branch} (merged into origin/main)"
  if ! git worktree remove "${wt}" 2>/dev/null; then
    say "skip ${rel}: git worktree remove failed"
    skipped=$((skipped+1))
    return 0
  fi
  if ! git branch -d "${branch}" 2>/dev/null; then
    echo "    note: branch ${branch} kept (delete with: git branch -D ${branch})" >&2
  fi
  removed=$((removed+1))
}

usage() {
  cat <<'EOF'
worktree.sh — worktree lifecycle for the OmniMux fork contract (AGENTS.md "Hard bounds (fork & git)").

Usage:
  worktree.sh new <task> [base]           create .worktrees/<task> on feat/<task>
                                          from base (default origin/main)
  worktree.sh list                        show worktrees and dirty state
  worktree.sh ship <task>                 finish: gate -> push feat -> merge main ->
                                          push main -> remove worktree + branch
  worktree.sh remove <task> [--discard]   remove; --discard drops uncommitted changes,
                       [--abandon]        --abandon drops an unmerged branch
  worktree.sh prune                       drop stale registrations; report merged feat branches
  worktree.sh clean                       sweep: remove worktrees + feat branches merged into origin/main
  worktree.sh help

Contract detail: docs/ops/worktree-zh.md
EOF
}

cmd="${1:-}"
shift || true
case "${cmd}" in
  new)    [ "$#" -ge 1 ] || die "usage: worktree.sh new <task> [base]"; cmd_new "$@" ;;
  list)   cmd_list ;;
  ship)   [ "$#" -ge 1 ] || die "usage: worktree.sh ship <task>"; cmd_ship "$1" ;;
  remove) [ "$#" -ge 1 ] || die "usage: worktree.sh remove <task> [--discard|--abandon]"; cmd_remove "$@" ;;
  prune)  cmd_prune ;;
  clean)  cmd_clean ;;
  stale)  cmd_stale ;;
  help | -h | --help) usage ;;
  "")     usage; exit 1 ;;
  *)      die "unknown command: ${cmd} (see: worktree.sh help)" ;;
esac
