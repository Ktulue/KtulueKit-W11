# Impeccable UI Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply Impeccable design system across all KtulueKit-W11 Svelte screens for suite-consistent visual quality.

**Architecture:** Purely visual — no behavioral changes. Apply Impeccable skill sequence (normalize→distill→polish→colorize→animate) per screen. Start with global tokens, then work screen-by-screen.

**Tech Stack:** Svelte 4, Impeccable design system, Wails v2 WebView2.

---

## Prerequisites (REQUIRED before starting)

**This branch must not start until all four feature branches are merged to `main`:**
- `feat/cli-polish` (adds `--profile`, post-install hooks, `--output-format`)
- `feat/config-url` (adds remote config fetch)
- `feat/uninstall` (adds Install/Uninstall tab bar to `SelectionScreen.svelte`)
- `feat/unit-tests` (test coverage — no UI changes)

The `SelectionScreen.svelte` tab bar referenced throughout this plan is added by `feat/uninstall`. If that branch is not yet merged, `SelectionScreen.svelte` will not have tabs and Task 2 cannot be executed as written.

**Create the branch first:**

```bash
git checkout main && git pull
git checkout -b feat/impeccable-ui
```

---

## Design Token Reference

All tokens are sourced from `CLAUDE.md` in the project root. Do not hardcode hex values anywhere — every value must reference a CSS custom property.

```css
/* Colors */
--color-bg-primary: #1a1a1a;
--color-bg-secondary: #111;
--color-bg-hover: #2a2a2a;
--color-border: #333;
--color-border-input: #555;
--color-text-primary: #e0e0e0;
--color-text-secondary: #888;
--color-text-tertiary: #aaa;
--color-accent: #0e7fd4;
--color-accent-hover: #1290e8;
--color-accent-disabled: #444;
--color-danger: #ff6b6b;
--color-danger-action: #c0392b;

/* Spacing (4px grid) */
--spacing-xs: 4px;
--spacing-sm: 6px;
--spacing-md: 8px;
--spacing-lg: 12px;
--spacing-xl: 16px;
--spacing-2xl: 20px;

/* Shape */
--radius: 4px;

/* Typography */
--font-primary: "Nunito", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
--font-size-xs: 11px;   /* raw output / timestamps */
--font-size-sm: 12px;   /* metadata, tooltips */
--font-size-base: 15px; /* body text */
--font-size-lg: 16px;   /* screen titles */
--font-size-xl: 18px;   /* app header */
```

**Uninstall mode rule:** Item rows use `--color-danger` (`#ff6b6b`) in place of `--color-accent` (`#0e7fd4`) when the app is in Uninstall mode. This applies only to `ItemRow.svelte` and the tab bar active indicator in `SelectionScreen.svelte`. All other UI stays the same.

---

## Chunk 1: Global Tokens + SelectionScreen

### Task 1: Global tokens — App.svelte font import and CSS custom properties

**Files:**
- `frontend/src/App.svelte`

**Context:** `App.svelte` is the root component. It must import the Nunito font from Google Fonts and declare all design tokens as CSS custom properties on `:root`. Any token currently hardcoded in individual component `<style>` blocks gets removed from those blocks in later tasks and inherited from `:root` instead. This task does not touch any component other than `App.svelte`.

---

- [ ] **Step 1: Read CLAUDE.md design tokens**

  Open `CLAUDE.md` and confirm the full token list. The source of truth is the "Current Design Tokens" block. Do not proceed if the list has changed from what appears in this plan — reconcile first.

- [ ] **Step 2: Use Skill tool — normalize on App.svelte**

  Call the `Skill` tool with `skill: "normalize"` targeting `frontend/src/App.svelte`. Target outcome:
  - A `@import` for Nunito via Google Fonts is present in the global `<style>` (or `<svelte:head>`).
  - All design tokens from the reference block above are declared on `:root` in a single CSS custom properties block.
  - `font-family` on `body` or the root container references `var(--font-primary)`.
  - `background-color` on the root container references `var(--color-bg-primary)`.

- [ ] **Step 3: Use Skill tool — distill on App.svelte**

  Call the `Skill` tool with `skill: "distill"` targeting `frontend/src/App.svelte`. Remove any duplicate property declarations, redundant wrapper `div`s that add no layout value, and any legacy hardcoded color or font values left over after normalize.

- [ ] **Step 4: Use Skill tool — polish on App.svelte**

  Call the `Skill` tool with `skill: "polish"` targeting `frontend/src/App.svelte`. Confirm consistent use of spacing tokens on the root layout container. Verify the app header (`#111` / `--color-bg-secondary`) sits flush against the window edge with no unintended gap.

- [ ] **Step 5: Verify in Wails dev server**

  Run `wails dev` from the project root. Check:
  - Nunito font loads (inspect in DevTools → Network → Fonts, or visually compare character shapes).
  - Background is `#1a1a1a`, not pure black or white.
  - No flash of unstyled content on load.
  - DevTools → Elements → `:root` shows all custom properties.

- [ ] **Step 6: Commit**

  ```
  git add frontend/src/App.svelte
  git commit -m "feat(ui): apply Impeccable global tokens and Nunito font import to App.svelte"
  ```

---

### Task 2: SelectionScreen.svelte — full Impeccable sequence

**Files:**
- `frontend/src/screens/SelectionScreen.svelte`

**Context:** This is the primary interactive screen. It contains:
- A tab bar with Install / Uninstall tabs and an active indicator
- A category accordion list (delegates to `CategoryAccordion.svelte`)
- Item rows within each category (delegates to `ItemRow.svelte`)
- Profile load/save buttons
- A primary action button (Install / Uninstall)

The tab bar active indicator must use `--color-accent` in Install mode and `--color-danger` in Uninstall mode. The action button follows the same rule. Component internals (accordion, item rows) are handled in their own tasks — this task focuses on layout, tab bar, profile buttons, and action button only.

---

- [ ] **Step 1: Use Skill tool — normalize on SelectionScreen.svelte**

  Call the `Skill` tool with `skill: "normalize"` targeting `frontend/src/screens/SelectionScreen.svelte`. Target outcome:
  - All hardcoded color values (`#0e7fd4`, `#1a1a1a`, `#e0e0e0`, etc.) replaced with the corresponding CSS custom property references.
  - Font sizes reference `var(--font-size-*)` tokens.
  - Spacing values reference `var(--spacing-*)` tokens.
  - Tab bar active indicator switches between `var(--color-accent)` and `var(--color-danger)` based on the mode prop/store.

- [ ] **Step 2: Use Skill tool — distill on SelectionScreen.svelte**

  Call the `Skill` tool with `skill: "distill"` targeting `frontend/src/screens/SelectionScreen.svelte`. Remove:
  - Visual noise: unnecessary borders, shadows, or decorative rules that don't communicate structure.
  - Redundant wrapper elements around the tab bar or action button area.
  - Any inline styles that duplicate what the `<style>` block already handles.

- [ ] **Step 3: Use Skill tool — polish on SelectionScreen.svelte**

  Call the `Skill` tool with `skill: "polish"` targeting `frontend/src/screens/SelectionScreen.svelte`. Verify:
  - Tab bar height and padding are consistent with the 4px grid (`--spacing-*` tokens).
  - Profile button group has uniform spacing between buttons.
  - Action button is full-width or clearly anchored to the bottom of the screen with consistent padding.
  - Text labels are vertically centered within their containers.

- [ ] **Step 4: Use Skill tool — colorize on SelectionScreen.svelte**

  Call the `Skill` tool with `skill: "colorize"` targeting `frontend/src/screens/SelectionScreen.svelte`. Verify:
  - Active tab indicator uses the correct accent color for the current mode.
  - Inactive tabs use `--color-text-secondary`.
  - Profile buttons use a neutral secondary style (not the primary blue accent).
  - Action button uses `--color-accent` (Install) or `--color-danger-action` (Uninstall) as background, with `--color-text-primary` label.
  - Disabled state of the action button uses `--color-accent-disabled`.

- [ ] **Step 5: Use Skill tool — animate on SelectionScreen.svelte**

  Call the `Skill` tool with `skill: "animate"` targeting `frontend/src/screens/SelectionScreen.svelte`. Add purposeful micro-interactions:
  - Tab switch: active indicator slides or cross-fades (100–150 ms ease-out). No bounce or spring.
  - Action button: subtle scale or background shift on hover/active (no glow, no shadow spread).
  - Profile buttons: background transition on hover matching `--color-bg-hover` (100 ms ease).

- [ ] **Step 6: Verify in Wails dev server**

  Run `wails dev`. Check:
  - Switch between Install and Uninstall tabs — active indicator color changes correctly.
  - Tab transition animation plays without jank.
  - Action button hover and click states are responsive.
  - Disabled action button looks visually distinct but not broken.
  - Profile buttons are legible and their hover state is visible.

- [ ] **Step 7: Commit**

  ```
  git add frontend/src/screens/SelectionScreen.svelte
  git commit -m "feat(ui): apply Impeccable sequence to SelectionScreen (tabs, profile buttons, action button)"
  ```

---

## Chunk 2: Remaining Screens + Audit

### Task 3: ProgressScreen.svelte — full Impeccable sequence

**Files:**
- `frontend/src/screens/ProgressScreen.svelte`

**Context:** Shown during installation/uninstallation. Contains:
- A scrolling progress feed of log lines (delegates to `ProgressItem.svelte`)
- An elapsed time counter
- A reboot dialog (modal or inline) shown when a reboot is required

---

- [ ] **Step 1: Use Skill tool — normalize on ProgressScreen.svelte**

  Call the `Skill` tool with `skill: "normalize"` targeting `frontend/src/screens/ProgressScreen.svelte`. Target outcome:
  - All hardcoded colors, font sizes, and spacing replaced with token references.
  - Progress feed container background uses `--color-bg-secondary` to differentiate it from the screen background (`--color-bg-primary`).
  - Elapsed time label uses `--color-text-tertiary` and `--font-size-sm`.

- [ ] **Step 2: Use Skill tool — distill on ProgressScreen.svelte**

  Call the `Skill` tool with `skill: "distill"` targeting `frontend/src/screens/ProgressScreen.svelte`. Remove:
  - Redundant borders or dividers between the feed and the elapsed time display.
  - Any decorative elements in the reboot dialog that don't serve the message.
  - Duplicate layout wrappers around the progress feed.

- [ ] **Step 3: Use Skill tool — polish on ProgressScreen.svelte**

  Call the `Skill` tool with `skill: "polish"` targeting `frontend/src/screens/ProgressScreen.svelte`. Verify:
  - Progress feed has comfortable internal padding (`--spacing-xl`) and scrolls smoothly.
  - Elapsed time counter is right-aligned or clearly positioned below the feed without crowding.
  - Reboot dialog has centered layout with consistent padding (`--spacing-2xl`), a clear heading, and two buttons (Reboot Now / Later) with proper spacing between them.
  - Reboot dialog button hierarchy: Reboot Now is primary (accent), Later is secondary (neutral/outlined).

- [ ] **Step 4: Use Skill tool — colorize on ProgressScreen.svelte**

  Call the `Skill` tool with `skill: "colorize"` targeting `frontend/src/screens/ProgressScreen.svelte`. Verify:
  - Reboot Now button uses `--color-accent` (not danger — a reboot is a normal action, not destructive).
  - Elapsed time text color is `--color-text-tertiary` (low priority, non-intrusive).
  - Feed background is `--color-bg-secondary`, creating a subtle tonal separation.
  - No stray hardcoded hex values remain.

- [ ] **Step 5: Use Skill tool — animate on ProgressScreen.svelte**

  Call the `Skill` tool with `skill: "animate"` targeting `frontend/src/screens/ProgressScreen.svelte`. Add:
  - New `ProgressItem` entries fade in as they appear (150 ms ease-in opacity transition).
  - Reboot dialog appears with a short fade-in (100 ms), not a jarring instant render.
  - No looping or attention-grabbing animations — this screen is informational and must feel calm.

- [ ] **Step 6: Verify in Wails dev server**

  Run `wails dev` and trigger a dry-run install to populate the progress feed. Check:
  - New items animate in without layout shift.
  - Elapsed time updates without visual jitter.
  - If the reboot dialog can be triggered manually (via a dev flag or test), confirm its appearance and button layout.
  - Scroll behavior in the feed is smooth and does not jump when new items arrive.

- [ ] **Step 7: Commit**

  ```
  git add frontend/src/screens/ProgressScreen.svelte
  git commit -m "feat(ui): apply Impeccable sequence to ProgressScreen (feed, elapsed time, reboot dialog)"
  ```

---

### Task 4: SummaryScreen.svelte — full Impeccable sequence

**Files:**
- `frontend/src/screens/SummaryScreen.svelte`

**Context:** Shown after installation completes. Contains:
- Result category headers (Succeeded, Failed, Skipped)
- Status badges next to each category header (count)
- A list of item names under each category

---

- [ ] **Step 1: Use Skill tool — normalize on SummaryScreen.svelte**

  Call the `Skill` tool with `skill: "normalize"` targeting `frontend/src/screens/SummaryScreen.svelte`. Target outcome:
  - All hardcoded colors, font sizes, and spacing replaced with token references.
  - Succeeded category uses `--color-accent` as its header accent color.
  - Failed category uses `--color-danger` as its header accent color.
  - Skipped category uses `--color-text-secondary` as its header accent color (neutral, not alarming).

- [ ] **Step 2: Use Skill tool — distill on SummaryScreen.svelte**

  Call the `Skill` tool with `skill: "distill"` targeting `frontend/src/screens/SummaryScreen.svelte`. Remove:
  - Redundant borders between categories if spacing alone provides sufficient separation.
  - Any duplicate wrapping elements around the status badges.
  - Decorative rules or dividers that add noise without adding structure.

- [ ] **Step 3: Use Skill tool — polish on SummaryScreen.svelte**

  Call the `Skill` tool with `skill: "polish"` targeting `frontend/src/screens/SummaryScreen.svelte`. Verify:
  - Category headers are clearly larger than item rows (`--font-size-lg` vs `--font-size-base`).
  - Status badges (item counts) are vertically centered with the header text and have consistent pill padding (`--spacing-xs` vertical, `--spacing-md` horizontal).
  - Item list rows have consistent left indentation relative to their category header.
  - Inter-category spacing is larger than intra-category spacing (visual grouping).

- [ ] **Step 4: Use Skill tool — colorize on SummaryScreen.svelte**

  Call the `Skill` tool with `skill: "colorize"` targeting `frontend/src/screens/SummaryScreen.svelte`. Verify:
  - Succeeded badge background is a tinted `--color-accent` (low-opacity fill, full-opacity text or border).
  - Failed badge background is a tinted `--color-danger`.
  - Skipped badge is neutral (`--color-bg-hover` fill, `--color-text-secondary` text).
  - Item name text uses `--color-text-primary`; no accent color bleed into item rows.
  - Screen background is `--color-bg-primary`.

- [ ] **Step 5: Use Skill tool — animate on SummaryScreen.svelte**

  Call the `Skill` tool with `skill: "animate"` targeting `frontend/src/screens/SummaryScreen.svelte`. Add:
  - Categories stagger-fade in on screen mount (150 ms per category, 50 ms stagger delay). Keep it subtle — the summary is a read-only results screen.
  - No persistent or looping animations.

- [ ] **Step 6: Verify in Wails dev server**

  Run a dry-run install to completion so the summary screen renders. Check:
  - Category headers and their accent colors are visually distinct and correct.
  - Badges show correct counts and are legibly styled.
  - Stagger animation plays on first render without causing layout shift.
  - All three category types (Succeeded, Failed, Skipped) look correct — test with a mixed result set if possible.

- [ ] **Step 7: Commit**

  ```
  git add frontend/src/screens/SummaryScreen.svelte
  git commit -m "feat(ui): apply Impeccable sequence to SummaryScreen (category headers, badges, item list)"
  ```

---

### Task 5: CategoryAccordion.svelte — normalize → distill → polish

**Files:**
- `frontend/src/components/CategoryAccordion.svelte`

**Context:** Renders each software category in the SelectionScreen. Has an expand/collapse toggle. Does not independently carry accent color — it receives context from SelectionScreen.

**Why no colorize or animate:** This component has no independent accent color (all color is inherited from parent context or is neutral UI chrome). The hover transition (100 ms ease to `--color-bg-hover`) is a polish-level concern and is specified in Step 3. Expand/collapse animation is behavioral, not cosmetic — it's handled in polish as transition timing on the body slot height, not as a standalone animate pass.

---

- [ ] **Step 1: Use Skill tool — normalize on CategoryAccordion.svelte**

  Call the `Skill` tool with `skill: "normalize"` targeting `frontend/src/components/CategoryAccordion.svelte`. Target outcome:
  - All hardcoded colors, font sizes, and spacing replaced with token references.
  - Header background uses `--color-bg-secondary`.
  - Header text uses `--color-text-primary` at `--font-size-base`.
  - Expand/collapse chevron or indicator uses `--color-text-secondary`.

- [ ] **Step 2: Use Skill tool — distill on CategoryAccordion.svelte**

  Call the `Skill` tool with `skill: "distill"` targeting `frontend/src/components/CategoryAccordion.svelte`. Remove:
  - Unnecessary nested wrappers inside the header or body slot.
  - Any border or shadow on the accordion body that duplicates what the header already communicates.
  - Redundant click-target padding (consolidate into the header element itself).

- [ ] **Step 3: Use Skill tool — polish on CategoryAccordion.svelte**

  Call the `Skill` tool with `skill: "polish"` targeting `frontend/src/components/CategoryAccordion.svelte`. Verify:
  - Header height is on the 4px grid (minimum 40px / `--spacing-2xl * 2`).
  - Chevron is right-aligned and visually balanced with the category label.
  - When collapsed, the border-bottom of the header provides clear visual separation between categories.
  - When expanded, the body slot has consistent internal top/bottom padding (`--spacing-lg`).
  - Hover state on the header uses `--color-bg-hover` with a 100 ms ease transition.

- [ ] **Step 4: Verify in Wails dev server**

  Run `wails dev`. Check:
  - Click a category header — it expands and collapses correctly.
  - Hover state is visible and matches `--color-bg-hover`.
  - Multiple categories open simultaneously all have consistent spacing.
  - The header text does not truncate or wrap unexpectedly at normal window sizes.

- [ ] **Step 5: Commit**

  ```
  git add frontend/src/components/CategoryAccordion.svelte
  git commit -m "feat(ui): apply Impeccable normalize/distill/polish to CategoryAccordion"
  ```

---

### Task 6: ItemRow.svelte — normalize → distill → polish → colorize (blue + red variants)

**Files:**
- `frontend/src/components/ItemRow.svelte`

**Context:** Each software item in a category. Has a checkbox, a label, and an optional tooltip. Receives a `mode` prop (or reads from a store) to know whether to render in Install (blue accent) or Uninstall (red accent) mode. This is the single component where the blue/red variant logic lives.

**Why no animate:** The only meaningful interaction is the checkbox state change and row hover. Both are handled as CSS transitions in polish (100 ms ease). There are no modal reveals, entrances, or multi-state transitions that warrant a dedicated animate pass on this component.

---

- [ ] **Step 1: Use Skill tool — normalize on ItemRow.svelte**

  Call the `Skill` tool with `skill: "normalize"` targeting `frontend/src/components/ItemRow.svelte`. Target outcome:
  - All hardcoded colors replaced with token references.
  - Checkbox accent color bound to a local CSS custom property (e.g., `--item-accent`) that defaults to `--color-accent` and is overridden to `--color-danger` when in Uninstall mode.
  - Label text uses `--color-text-primary` at `--font-size-base`.
  - Tooltip text uses `--color-text-secondary` at `--font-size-sm`.

- [ ] **Step 2: Use Skill tool — distill on ItemRow.svelte**

  Call the `Skill` tool with `skill: "distill"` targeting `frontend/src/components/ItemRow.svelte`. Remove:
  - Any extra wrapper `div` between the checkbox and the label if they can be siblings in a flex row.
  - Tooltip markup that duplicates the label text rather than adding supplementary information.
  - Unused CSS classes or rules left over from earlier iterations.

- [ ] **Step 3: Use Skill tool — polish on ItemRow.svelte**

  Call the `Skill` tool with `skill: "polish"` targeting `frontend/src/components/ItemRow.svelte`. Verify:
  - Row height is consistent across all items and on the 4px grid.
  - Checkbox and label are vertically centered in the row.
  - Sufficient horizontal padding on the left (`--spacing-xl`) so the checkbox does not sit flush against the accordion body edge.
  - Hover state over the entire row uses `--color-bg-hover` (not just the checkbox), making the click target obvious.
  - Tooltip appears below or to the right of the label with a short delay and does not clip at screen edges.

- [ ] **Step 4: Use Skill tool — colorize on ItemRow.svelte**

  Call the `Skill` tool with `skill: "colorize"` targeting `frontend/src/components/ItemRow.svelte`. Verify both variants:
  - **Install mode (blue):** Checkbox checked state uses `--color-accent`. Row hover uses `--color-bg-hover`. No red anywhere.
  - **Uninstall mode (red):** Checkbox checked state uses `--color-danger`. Row hover still uses `--color-bg-hover` (neutral). Label text remains `--color-text-primary` — do not make the entire label red.
  - The mode switch must happen purely via the `--item-accent` CSS custom property toggle — no duplicated style blocks.

- [ ] **Step 5: Verify in Wails dev server**

  Run `wails dev`. Check both tabs:
  - In Install mode: check and uncheck several items — checkbox accent is blue.
  - Switch to Uninstall tab: checkbox accent becomes red.
  - Row hover is visible in both modes.
  - Tooltip appears for items that have descriptions; confirm it does not obscure adjacent rows.

- [ ] **Step 6: Commit**

  ```
  git add frontend/src/components/ItemRow.svelte
  git commit -m "feat(ui): apply Impeccable sequence to ItemRow with blue/red mode variants"
  ```

---

### Task 7: ProgressItem.svelte — normalize → distill → polish

**Files:**
- `frontend/src/components/ProgressItem.svelte`

**Context:** A single line in the progress feed. Has a status icon (success, failure, running, pending), a text message, and optional timestamp or spacing. Does not carry accent color independently — status icon color is determined by item status, not by install/uninstall mode.

**Why no colorize or animate:** Colorize is redundant here — icon status colors (accent for success, danger for failure, secondary for pending) are all defined in normalize. There is no secondary coloring decision to make. Animation of new feed entries entering the list is handled at the `ProgressScreen` level (the parent), not at the `ProgressItem` level — the item itself is static once rendered.

---

- [ ] **Step 1: Use Skill tool — normalize on ProgressItem.svelte**

  Call the `Skill` tool with `skill: "normalize"` targeting `frontend/src/components/ProgressItem.svelte`. Target outcome:
  - All hardcoded colors replaced with token references.
  - Success icon uses `--color-accent`.
  - Failure icon uses `--color-danger`.
  - Running/pending icon uses `--color-text-secondary` or `--color-text-tertiary`.
  - Message text uses `--color-text-primary` at `--font-size-base`.
  - Timestamp (if present) uses `--color-text-tertiary` at `--font-size-xs`.

- [ ] **Step 2: Use Skill tool — distill on ProgressItem.svelte**

  Call the `Skill` tool with `skill: "distill"` targeting `frontend/src/components/ProgressItem.svelte`. Remove:
  - Any wrapper elements between the icon and text that are not needed for alignment.
  - Redundant margin or padding rules that duplicate what the parent feed container already provides.
  - Unused status variants or dead CSS rules.

- [ ] **Step 3: Use Skill tool — polish on ProgressItem.svelte**

  Call the `Skill` tool with `skill: "polish"` targeting `frontend/src/components/ProgressItem.svelte`. Verify:
  - Icon and text are vertically centered in the row.
  - Icon has a fixed width so text aligns across all rows regardless of icon character width.
  - Row padding uses `--spacing-sm` vertical and `--spacing-xl` horizontal (comfortable but dense — this is a log feed).
  - No bottom border or divider between rows — spacing alone provides rhythm.
  - Text does not wrap in normal cases; use `overflow: hidden` + `text-overflow: ellipsis` if lines can be long.

- [ ] **Step 4: Verify in Wails dev server**

  Run `wails dev` and trigger a dry-run install. Check:
  - Icons render at the correct size and color for each status type.
  - Text is left-aligned and icon-aligned across all rows.
  - Dense feed of 10+ items remains readable without feeling cramped.

- [ ] **Step 5: Commit**

  ```
  git add frontend/src/components/ProgressItem.svelte
  git commit -m "feat(ui): apply Impeccable normalize/distill/polish to ProgressItem"
  ```

---

### Task 8: Final audit pass — all screens

**Files:**
- `frontend/src/App.svelte`
- `frontend/src/screens/SelectionScreen.svelte`
- `frontend/src/screens/ProgressScreen.svelte`
- `frontend/src/screens/SummaryScreen.svelte`
- `frontend/src/components/CategoryAccordion.svelte`
- `frontend/src/components/ItemRow.svelte`
- `frontend/src/components/ProgressItem.svelte`

**Context:** A holistic review pass using the `audit` Impeccable skill across all screens before the branch is considered done. The goal is to catch anything that was addressed in isolation but looks off in the full flow: inconsistent spacing between screens, token drift, animation timing mismatches, or regressions introduced by component interactions.

---

- [ ] **Step 1: Use Skill tool — audit across all screens**

  Call the `Skill` tool with `skill: "audit"` with all seven files as scope. The audit should flag:
  - Any remaining hardcoded hex values or magic numbers.
  - Spacing or font-size inconsistencies between components (e.g., ItemRow padding does not match ProgressItem padding at the same conceptual level).
  - Animation timing that is out of step with the rest of the UI (e.g., one component uses 300 ms where others use 100–150 ms).
  - Color usage that contradicts the intent in the design token reference (e.g., accent color used for a non-CTA element).

- [ ] **Step 2: Apply audit fixes**

  For each issue flagged by `audit`, apply the minimal fix using the appropriate Impeccable skill (normalize, distill, polish, colorize, or animate). Document each fix in the commit message.

- [ ] **Step 3: Full visual walkthrough in Wails dev server**

  Run `wails dev`. Walk through the full user flow:
  1. App launches → SelectionScreen renders with Install tab active.
  2. Browse categories, check items, hover rows — verify Install mode (blue) is consistent.
  3. Switch to Uninstall tab — verify red accent appears on tab indicator and item checkboxes.
  4. Start a dry-run install → ProgressScreen renders, feed populates, animations play.
  5. Dry-run completes → SummaryScreen renders with category results and stagger animation.
  6. Confirm no layout shift, no font fallback flash, no color inconsistencies across the flow.

- [ ] **Step 4: Commit**

  ```
  git add frontend/src/App.svelte \
           frontend/src/screens/SelectionScreen.svelte \
           frontend/src/screens/ProgressScreen.svelte \
           frontend/src/screens/SummaryScreen.svelte \
           frontend/src/components/CategoryAccordion.svelte \
           frontend/src/components/ItemRow.svelte \
           frontend/src/components/ProgressItem.svelte
  git commit -m "feat(ui): final Impeccable audit pass — token consistency and animation alignment"
  ```

---

## Branch Summary

| Task | Files | Skills Applied | Omissions |
|------|-------|---------------|-----------|
| 1 | `App.svelte` | normalize, distill, polish | colorize/animate: root shell has no interactive elements |
| 2 | `SelectionScreen.svelte` | normalize, distill, polish, colorize, animate | full sequence |
| 3 | `ProgressScreen.svelte` | normalize, distill, polish, colorize, animate | full sequence |
| 4 | `SummaryScreen.svelte` | normalize, distill, polish, colorize, animate | full sequence |
| 5 | `CategoryAccordion.svelte` | normalize, distill, polish | colorize: no independent accent; animate: hover transition covered in polish |
| 6 | `ItemRow.svelte` | normalize, distill, polish, colorize | animate: checkbox/hover covered by CSS transitions in polish |
| 7 | `ProgressItem.svelte` | normalize, distill, polish | colorize: status colors defined in normalize; animate: entrance handled by parent ProgressScreen |
| 8 | All 7 files | audit | — |

**No Go or Wails binding changes.** All commits are frontend-only. The branch is ready to merge when the full visual walkthrough in Task 8 Step 3 passes without issues.
