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
3. v1.0 wrap-up — worktree cleanup, README check, tag

**Branch:** `chore/v1-wrap-up` — all work lands here, squash-merged to main before tagging.

---

## Phase 1: TODO Verification

Systematically confirm every item in `TODO.md` reflects actual codebase state.

**Method:** For each item, grep/read the relevant code and confirm status. Produce a verification report before touching the file.

**Known stale unchecked items (based on commit history):**

| Item | Committed in | Action |
|---|---|---|
| Post-install hooks | a9ad01d (feat/cli-polish) | Mark `[x]` |
| Summary export formats | a9ad01d (feat/cli-polish) | Mark `[x]` |
| Unit tests | e542f47 (feat/unit-tests) | Mark `[x]` |
| Parallel installs | Explicitly out of scope (finishing sprint spec) | Mark `[x]` with "Out of scope" note |

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
- **`accept`** — acknowledged risk; design decision (personal tool, no multi-user threat model); document in a code comment

### Phase 2c — Fixes + Tests

- Implement patches for all `fix` items on `chore/v1-wrap-up`
- Write Go tests for all `test` items, following the existing table-driven pattern
- If any single fix is large enough to warrant isolation, it gets its own branch off `chore/v1-wrap-up`

**Deliverable:** All findings triaged; fixes and tests committed; no open `fix` items remaining.

---

## Phase 3: v1.0 Wrap-Up

After security work is merged:

1. **Worktree cleanup** — remove `.worktrees/polish-sprint/` and `.worktrees/feat-gui/` (leftover from previous feature work)
2. **README check** — verify Ko-fi support section (`ko-fi.com/ktulue`) exists at bottom; append if missing
3. **Merge `chore/v1-wrap-up` → main** — squash merge, `/security-review` run before PR
4. **Tag `v1.0`** — `git tag v1.0` on main post-merge
5. **Final status check** — confirm clean tree, no stray files or branches

**Deliverable:** Tagged `v1.0` on main. Clean repo. KtulueKit-Migration handoff ready.

---

## Success Criteria

- [ ] Every `TODO.md` item is `[x]` with accurate status
- [ ] All security findings triaged into fix / test / accept
- [ ] No open `fix` findings remain unpatched
- [ ] Regression tests written for all `test` findings
- [ ] `accept` findings documented with code comments
- [ ] `.worktrees/polish-sprint/` and `.worktrees/feat-gui/` removed
- [ ] README.md contains Ko-fi support section
- [ ] `chore/v1-wrap-up` squash-merged to main
- [ ] `v1.0` tag on main
- [ ] Clean `git status` on main post-tag
