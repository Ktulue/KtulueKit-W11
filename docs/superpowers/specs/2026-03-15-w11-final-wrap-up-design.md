# KtulueKit-W11: Final Wrap-Up Design

**Date:** 2026-03-15
**Status:** Approved
**Scope:** TODO verification, security review + triage, v1.0 tag

---

## Overview

All five finishing sprint tracks are merged to main. This spec covers the final work needed to close out KtulueKit-W11 at v1.0 and hand off cleanly to KtulueKit-Migration.

**Three sequential phases:**
1. TODO verification — reconcile every item against actual shipped code
2. Security review → triage → fixes → tests
3. v1.0 wrap-up — worktree cleanup, README update, tag + push

**Branch:** `chore/v1-wrap-up` — all work lands here, squash-merged to main before tagging.

---

## Phase 1: TODO Verification

Systematically confirm every item in `TODO.md` reflects actual codebase state.

**Method:** For each item, grep/read the relevant code and confirm status. Produce a verification report before touching the file.

**Known stale unchecked items (based on commit history):**

| Item | Committed in | Action |
|---|---|---|
| Post-install hooks | a9ad01d (feat/cli-polish) | Verify `PostInstall` field in `schema.go` + hook execution in runner, then mark `[x]` |
| Summary export formats | a9ad01d (feat/cli-polish) | Verify `--output-format` in `cmd/main.go` + reporter, then mark `[x]` |
| Unit tests | e542f47 (feat/unit-tests) | **Verify** that `classifyWingetExit` and `buildWingetArgs` (called out by name in the TODO item) have test coverage before marking `[x]`; if missing, write those tests in this pass |
| Parallel installs | Explicitly out of scope per finishing sprint spec | Mark `[x]` with note: "Out of scope for v1.0 — winget concurrent install behavior is undefined; deferred to post-v1.0 consideration" |

**Deliverable:** `TODO.md` fully reconciled, committed on `chore/v1-wrap-up`.

---

## Phase 2: Security Review → Triage → Fixes → Tests

### Phase 2a — Review

Run `sentry-skills:security-review` against the full codebase. Scope: Go backend (`cmd/`, `internal/`) and Svelte frontend (`frontend/src/`).

**Primary threat surfaces:**

| Surface | Risk |
|---|---|
| Command injection | Shell commands built from config-supplied strings (`post_install`, `uninstall_cmd`, `cmd` fields) passed to `cmd /C` |
| Path traversal | Config file loading, state file writes to `%LOCALAPPDATA%\KtulueKit\state.json` |
| Remote config fetch | HTTPS enforcement, 1MB cap, temp file handling, URL validation, cleanup on error paths |
| Registry writes | Extension install/uninstall renumbering, Windows settings tweaks (developer mode, show-hidden, etc.) |
| Winget arg injection | `--id`, `--version`, `--scope` args built from config-supplied values |
| Svelte / WebView2 | Eval-like patterns, unsanitized content rendered from Go backend events |

### Phase 2b — Triage

Each finding receives one of three verdicts:

- **`fix`** — genuine vulnerability; patch required
- **`test`** — correct behavior but needs a regression test to stay that way
- **`accept`** — acknowledged risk; design decision (personal tool, no multi-user threat model); document in code with `// SECURITY-ACCEPT: <one-line rationale>`

### Phase 2c — Fixes + Tests

- Implement patches for all `fix` items on `chore/v1-wrap-up`
- Write Go tests for all `test` items, following the existing table-driven pattern
- If any single fix is large enough to warrant isolation, it gets its own branch off `chore/v1-wrap-up`

**Deliverable:** All findings triaged; fixes and tests committed; no open `fix` items remaining.

---

## Phase 3: v1.0 Wrap-Up

After security work is committed:

1. **Worktree cleanup** — run `git worktree remove .worktrees/polish-sprint` and `git worktree remove .worktrees/feat-gui` (deregisters from git metadata and removes directories); delete local branches `maint/polish-sprint` and `feat/gui` with `git branch -d` if no longer needed
2. **README update** — verify Ko-fi support section (`ko-fi.com/ktulue`) exists at bottom; append if missing. Update the CLI flags table to include all shipped flags: `--profile`, `--only`, `--exclude`, `--phase`, `--upgrade-only`, `--no-upgrade`, `--output-format`, `--no-desktop-shortcuts`
3. **Merge `chore/v1-wrap-up` → main** — squash merge via PR; run `/security-review` before opening PR
4. **Tag and push** — `git tag v1.0` on main post-merge, then `git push origin v1.0`
5. **Branch sweep** — delete all merged feature branches that are no longer needed: `feat/cli-features`, `feat/cli-polish`, `feat/config-url`, `feat/export-scan`, `feat/finish-sprint`, `feat/impeccable-ui`, `feat/scrape-download-installer`, `feat/small-wins`, `feat/uninstall`, `feat/unit-tests`, `maint/polish-sprint`, `feat/gui` (local only; remote tracking refs pruned via `git remote prune origin`)
6. **Final status check** — confirm clean tree, no stray worktrees, no local feature branches remaining

**Deliverable:** Tagged and pushed `v1.0` on main. Clean repo. KtulueKit-Migration handoff ready.

---

## Success Criteria

- [ ] Every `TODO.md` item is `[x]` with accurate status
- [ ] `classifyWingetExit` and `buildWingetArgs` confirmed covered by tests (or tests written)
- [ ] All security findings triaged into fix / test / accept
- [ ] No open `fix` findings remain unpatched
- [ ] Regression tests written for all `test` findings
- [ ] `accept` findings documented with `// SECURITY-ACCEPT:` comments
- [ ] Worktrees removed via `git worktree remove`; local branches cleaned up
- [ ] README.md contains Ko-fi support section
- [ ] README.md CLI flags table is current with all shipped flags
- [ ] `chore/v1-wrap-up` squash-merged to main
- [ ] `v1.0` tag created and pushed to origin
- [ ] Clean `git status` on main post-tag
