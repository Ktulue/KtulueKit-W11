# Scope Contract
**Task:** Impeccable UI — apply suite design system across all KtulueKit-W11 Svelte screens | **Plan:** `docs/superpowers/plans/2026-03-14-impeccable-ui.md` | **Date:** 2026-03-14 | **Status:** CLOSED — 1 change logged

## In Scope

- **Files:**
  - `frontend/src/App.svelte`
  - `frontend/src/screens/SelectionScreen.svelte`
  - `frontend/src/screens/ProgressScreen.svelte`
  - `frontend/src/screens/SummaryScreen.svelte`
  - `frontend/src/components/CategoryAccordion.svelte`
  - `frontend/src/components/ItemRow.svelte`
  - `frontend/src/components/ProgressItem.svelte`

- **Features / Criteria:**
  - Task 1 — `App.svelte`: Nunito Google Fonts `@import`; all design tokens as CSS custom properties on `:root`; `var(--font-primary)` on body; `var(--color-bg-primary)` on root container
  - Task 2 — `SelectionScreen.svelte`: All hardcoded values → token vars; tab active indicator `--color-accent` (install) / `--color-danger` (uninstall); action button `--color-accent` / `--color-danger-action`; disabled → `--color-accent-disabled`; tab transition 100–150ms ease-out; button hover 100ms; profile button hover 100ms
  - Task 3 — `ProgressScreen.svelte`: All tokens; feed bg `--color-bg-secondary`; elapsed → `--color-text-tertiary` + `--font-size-sm`; Reboot Now button → `--color-accent`; new feed items fade-in 150ms; reboot dialog fade-in 100ms
  - Task 4 — `SummaryScreen.svelte`: All tokens; Succeeded header `--color-accent`; Failed `--color-danger`; Skipped `--color-text-secondary`; tinted badge fills; categories stagger-fade on mount (150ms, 50ms stagger)
  - Task 5 — `CategoryAccordion.svelte`: All tokens; header bg `--color-bg-secondary`; chevron `--color-text-secondary`; hover `--color-bg-hover` 100ms ease; no colorize/animate pass
  - Task 6 — `ItemRow.svelte`: All tokens; local `--item-accent` CSS var (defaults `--color-accent`, overridden to `--color-danger` in uninstall mode); CSS transitions 100ms on hover/checkbox; no animate pass
  - Task 7 — `ProgressItem.svelte`: All tokens; success icon `--color-accent`; failure `--color-danger`; pending `--color-text-secondary`; no colorize/animate pass
  - Task 8 — Audit: flag remaining hardcoded hex/magic numbers, spacing inconsistencies, animation timing mismatches; apply minimal fixes

- **Explicit Boundaries:**
  - Purely visual — zero behavioral changes to any component
  - No Go or Wails binding changes
  - No new files (tokens live on `:root` in App.svelte per plan — no shared `tokens.css`)
  - Verification steps (`wails dev`) are manual — user confirms visually after PR

## Out of Scope

- Any Go backend changes
- New shared CSS files (`tokens.css` — future work)
- Behavioral changes to install/uninstall flow, state, or event handling
- `feat/unit-tests` content (Track 5 not yet done — no UI changes, does not block this track)

## Prerequisite Note

`feat/unit-tests` (Track 5) is not yet merged. Per plan it's a prerequisite, but it contains **no UI changes**. Proceeding per user's "ok, let's tackle track4" instruction.

# Scope Change Log
| # | Category | What | Why | Decision | Outcome |
|---|----------|------|-----|----------|---------|
| 1 | opportunistic | Fix `8px` progress-bar height → `var(--spacing-md)` | Audit flagged exact token match was available | Permit | Applied in audit commit |

# Follow-up Tasks
- [ ] Add `rustup-init.exe` as a scrape-download Command entry in `ktuluekit.json` — from https://rustup.rs/ — deferred from feat/uninstall
- [ ] Centralize design tokens into `tokens.css` — `:root` in App.svelte is the step forward per this plan; full centralization is future work (noted in CLAUDE.md)
