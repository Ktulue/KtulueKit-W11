# KtulueKit-W11 — Claude Project Instructions

## Design Context

This file captures design context for the KtulueKit Suite so all future sessions maintain a consistent visual language across all three apps.

### Suite Overview

KtulueKit is a 3-app suite — **W11 → Migration → Cleanup** — each handling a distinct phase of the Windows setup/migration lifecycle. All three are separate codebases but share a unified design system. Design decisions made here apply to all three suite apps.

Inspired by Chris Titus Tech's WinUtil. Built with Wails (Go) + Svelte + WebView2 as a single embedded `.exe`.

### Users

Primarily Josh (personal power tool). A secondary audience of developers who might fork it for their own stacks — so structure and tokens should be legible enough for someone else to pick up and customize, even if that's not the primary driver of design decisions.

### Brand Personality

**Craftsman's tool.** Someone clearly cared about building this right. Clean, considered, feels good to use. The UI is not an afterthought — it reflects intentional decisions — but it never draws attention to itself over the task at hand.

Three words: **Intentional. Clean. Reliable.**

### Aesthetic Direction

- **Dark theme, always.** Background `#1a1a1a`, header/footer `#111`. Dark — but not oppressive. Breathing room via spacing, not lightness.
- **Windows blue accent** (`#0e7fd4`) for all primary CTAs. Single accent color across the suite.
- **Nunito** as the primary typeface — friendly, clean, readable at small sizes.
- **Inspired by WinUtil**, not cloned from it. That dark, utilitarian installer aesthetic is the right direction.
- **Not** corporate/bloated, not playful/bubbly, not generic Bootstrap-blue-on-white.

### Design Principles

1. **Crafted, not generated** — every component should feel intentional, not default. Avoid anything that looks like an out-of-the-box template.
2. **Dark but breathable** — dark palette with consistent spacing and contrast. Not a pure black void; the eye needs places to rest.
3. **Suite-consistent** — one design language across W11, Migration, and Cleanup. Tokens, spacing, type scale, and component patterns must be portable.
4. **Utility-first** — UI serves the task. No decorative elements that don't communicate something. Every visual element earns its place.
5. **Fork-ready** — CSS custom properties (not hardcoded hex), component conventions, and clear structure so another dev can understand and extend it without reverse-engineering everything.

### Current Design Tokens (W11 — source of truth for suite)

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

> **Note:** These tokens are currently hardcoded in component `<style>` blocks. Future work should centralize these into a shared `tokens.css` or `style.css` so the suite apps can import the same baseline.
