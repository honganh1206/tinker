# Tinker Web UI Design System

This is the design guideline for the [`internal/web/`](../internal/web/) UI. It was derived from a study of the Modal (`modal.com`) front-end and tailored to Tinker's session-inspector layout. The system is dark-themed and **monochromatic green**, leveraging **Tailwind CSS v4**, **Svelte 5 (runes)**, and (when overlays are needed) **Melt UI**.

It features high-quality typography, glassmorphism for overlays, and a single primary green accent.

---

## 1. Design Tokens

A three-tier token architecture (Primitive → Semantic → Component) lives in [`internal/web/src/app.css`](../internal/web/src/app.css). Use semantic tokens in component CSS — never reference primitives directly outside the `:root` block.

### A. Color Palette (Primitives)
| Token | Value | Role |
| :--- | :--- | :--- |
| `green-100` | `#7fee64` | Primary brand green |
| `green-light` | `#ddffdc` | Secondary/Soft green — **default body text color** |
| `green-30` | `rgba(127, 238, 100, 0.30)` | Focus rings, accent borders, soft outlines |
| `gray-dark` | `#2f2f2f` | Surface/UI Gray |
| `gray-muted` | `#a3a3a3` | Text/Subtle Gray |
| `black` | `#000000` | Deepest Background |
| `ground` | `#181818` | Base Surface |
| `white` | `#ffffff` | Headings, hero titles |

### B. Semantic Tokens
| Token | Value | Mapping |
| :--- | :--- | :--- |
| `--color-action-primary` | `var(--green-100)` | Primary buttons, link hover, active states, brand accents |
| `--color-surface-deep` | `var(--black)` | Page background |
| `--color-surface-base` | `var(--ground)` | Cards, dropdown menus, detail panes |
| `--color-surface-raised` | `var(--gray-dark)` | Sidebar, code block backgrounds |
| `--color-border-subtle` | `rgba(255, 255, 255, 0.1)` | 1px dividers, component outlines |
| `--color-border-accent` | `rgba(221, 255, 220, 0.30)` | Light-green tinted borders (chips, message bubbles, empty card) |
| `--color-text-primary` | `var(--white)` | Headings only — heading-tier elements |
| `--color-text-body` | `var(--green-light)` | **Default body text** — applied at `<body>` level |
| `--color-text-link` | `var(--green-light)` | Nav links, sidebar links (hover → `--color-action-primary`) |
| `--color-text-secondary` | `rgba(221, 255, 220, 0.60)` | Captions, meta text, row meta |
| `--color-text-tertiary` | `rgba(221, 255, 220, 0.45)` | Disabled / placeholder text, system prompt rows |
| `--color-text-muted-neutral` | `var(--gray-muted)` | Editorial meta where green tint is undesirable |

> **Note on the text hierarchy:** Tinker's web UI is a **monochromatic green** system, not white-on-black. The `<body>` element sets `color: var(--color-text-body)` so paragraphs, nav links, sidebar items, and chrome inherit a desaturated mint (`#ddffdc`) by default. Pure white is reserved for explicit headings (the brand chip, page titles, `<h1>`/`<h2>`/`<h3>` inside markdown). Tinted alphas of `green-light` drive secondary, tertiary, and accent-border tiers.

### C. Typography
*   **Brand Display:** `Inter`, `system-ui`, `sans-serif` (Headings) — `var(--font-family-display)`
*   **Body/UI:** `Inter`, `system-ui`, `sans-serif` (Interface) — `var(--font-family-ui)`
*   **Monospace:** `Fira Mono`, `ui-monospace`, `Consolas`, `monospace` (Code blocks, session IDs, trace rows) — `var(--font-family-code)`

> Display and UI both alias to Inter today. The two tokens are distinct so a future heading-only typeface (e.g. degular-display) can be swapped in by editing one variable.

### D. Component Scales

**Spacing:** 4 px base. Multipliers: `1 (4px)`, `2 (8px)`, `3 (12px)`, `4 (16px)`, `5 (20px)`, `6 (24px)`, `8 (32px)`, `10 (40px)`. Exposed as `--space-1` … `--space-10`.

**Radii:** `sm: 4px`, `md: 6px`, `lg: 8px`, `xl: 12px`, `full: 9999px`. Exposed as `--radius-sm` … `--radius-full`.

**Motion:** `--motion-duration: 300ms` (default), `--motion-duration-fast: 150ms`, `--motion-ease: cubic-bezier(.4, 0, .2, 1)`. Use these on every transition for a consistent feel.

**Focus ring:** `0 0 0 3px var(--green-30)` via the global `:focus-visible` rule. Do not introduce per-component focus styling.

---

## 2. Component Catalog

### Buttons
| Variant | Styles | Interaction |
| :--- | :--- | :--- |
| **Marketing** | Pill-shape, `green-100` or `dark` background, large padding. | 150 ms `ease-out` transition on hover. |
| **Primary** | `radius-md`, high-contrast text, standard UI sizing. | Focus ring inherited from `:focus-visible` (3 px `green-30`). |
| **Icon** | 28 × 28 px circle, transparent border default, `--color-text-secondary`. | On hover: `bg: var(--color-accent-tint)`, `color: var(--color-action-primary)`, border becomes `--color-border-subtle`. |
| **Social** | 40 × 40 px circle, 1 px light-green border. | 10 % opacity light-green fill on hover. |

### Sidebar Navigation
*   **Structure:** Fixed-width left column (`280px`) on `--color-surface-raised`. Brand mark + wordmark at the top, search/combobox, then sections (Sessions, MCP, Settings, …).
*   **Brand mark:** Sleek — bare app icon (22 × 22 px, `--radius-md`) plus wordmark in `var(--font-family-display)` at 0.98 rem / 600 weight. **No pill chrome.** Wordmark inherits `--color-text-body` (light-green).
*   **Section labels:** `0.66rem`, weight 600, uppercase, `0.08em` letter-spacing, `--color-text-tertiary`.
*   **Nav rows (`.row`):** 6 px radius, 8 px / 12 px / 14 px padding, light-green inherited text. Hover applies `bg: var(--color-row-hover)`. Active item shows a 2 px `--green-100` left bar (a `::before` pseudo-element) — never a solid lime fill.

### Overlays (Dropdowns & Modals)
*   **Visual Style:** Glassmorphic — `backdrop-filter: blur(12px)` over a 50–85 % opacity `--color-surface-base`, 1 px `--color-border-subtle`, `--radius-xl`, `box-shadow: 0 20px 60px rgba(0,0,0,0.5)`.
*   **Behavior:** Triggered by **Melt UI** (the runes-first `melt` package). Focus-aware item selection with `--color-action-primary` highlights. Wrap every Melt builder in `src/lib/ui/{Dialog,Combobox,Tabs,Tooltip}.svelte` so the glass tokens live in one place.

### Chat Bubbles
*   **Agent bubble:** `--color-surface-base` background, `--color-border-subtle` border, `--radius-lg`, `--font-family-ui`. Body text inherits `--color-text-body`.
*   **User bubble:** Same shape, with `--color-accent-tint-soft` background and `--green-30` border. Avatar on the right, lime fill, black initials.
*   **Agent avatar:** 32 × 32 px, transparent background, contains the app icon (`/icon-192.png`). Rendered with `object-fit: cover` inside the circular avatar.

### Trace Rows
Single-line records inside a thread, distinguished by tier:
| Class | Tier | Example |
| :--- | :--- | :--- |
| `.trace-row--tool` | Secondary text + thin left rule | `Wrench` icon + tool invocation |
| `.trace-row--result` | Action color (green-100) | `CornerDownRight` icon + tool result |
| `.trace-row--system` | Tertiary, collapsible | "system prompt · N tokens" toggle |
| `.trace-row--fallback` | Tertiary | Unknown record types |

### Text & Link Hierarchy

A monochromatic green system. Every level beyond explicit headings is a tint of `green-light`.

| Tier | Token / Utility | Used for |
| :--- | :--- | :--- |
| **Heading** | `var(--color-text-primary)` / `text-white` | Brand wordmark, page titles, `.md h1` / `h2` / `h3` |
| **Body** | `var(--color-text-body)` (inherited) | Paragraphs, sidebar nav labels, row titles, message-bubble copy |
| **Secondary** | `var(--color-text-secondary)` | Row meta, captions, "$N tokens" labels |
| **Tertiary** | `var(--color-text-tertiary)` | Placeholders, disabled states, collapsed system rows |
| **Editorial neutral** | `var(--color-text-muted-neutral)` | Timestamps or other metadata where the green tint would feel playful (rare in Tinker) |
| **Action** | `var(--color-action-primary)` | Hover state for any link, active-row indicator, tool-result rows, brand accents |
| **Link rule** | Default = body color, hover = action color | Chrome links (sidebar nav) inherit body via the global `a { color: inherit }` rule. Inline prose links inside `.md` start at `--color-action-primary` for emphasis. |

### Data & Code
*   **Minimal Tables:** `border-collapse: separate`, muted text, ~5 % white header background. (Used sparingly — Tinker is not table-heavy.)
*   **Code Blocks (`.md pre`, `.md code`):** 1 px `--color-border-subtle`, `--color-surface-raised` background, `--radius-md` for blocks / `--radius-sm` for inline. Diff line hooks: `.md .diff-add` (green-tinted, green left bar), `.md .diff-del` (red-tinted, red left bar). Future Shiki integration drops in via these classes.

### Cards (Empty State, Settings)
*   `--color-surface-base` background, `--color-border-accent` border (warm light-green tint), `--radius-xl`.
*   `var(--font-family-display)` for any title; body content inherits `--color-text-body`.
*   Optional accent: an 8 px `--color-action-primary` dot with a `--green-30` halo via `box-shadow: 0 0 0 3px var(--green-30)`.

---

## 3. Interaction & Layout Patterns

### Layout
*   **Macro Layout:** Two-column shell — fixed sidebar (`280px`) + flexible detail pane. Page background is `--color-surface-deep` (#000), sidebar is `--color-surface-raised`, detail pane is `--color-surface-base`.
*   **Reading container:** Inside the detail pane, a `max-width: 800px` inner wrapper (`.session-detail-inner`) caps thread reading width — analogous to `marketing-container` on content sites.
*   **Breakpoints:** Standard Tailwind scale (`md:`, `lg:`, `xl:`). The shell collapses to stacked rows below 900 px, with the sidebar capped at `40vh` and a soft bottom border.

### Interaction Behaviors
*   **Preloading:** Once on SvelteKit, use `data-sveltekit-preload-data="hover"` on top-level nav links so `/sessions`, `/mcp`, `/settings` feel instantaneous.
*   **Smooth Motion:** Every transition uses `var(--motion-duration)` or `var(--motion-duration-fast)` paired with `var(--motion-ease)`. Do not introduce ad-hoc easings.
*   **Streaming feedback:** When a chat record is streaming, signal it with subtle motion (e.g. fading text or a pulsing accent dot) rather than spinners — keeps the dark surface calm.

---

## 4. Cleanup & Maintenance Rules

These are lessons captured from rebuilding the Tinker web UI; treat them as PR-review checklist items.

1.  **Single border token:** All 1 px dividers and component outlines use `--color-border-subtle`. Light-green tinted accents use `--color-border-accent`. Never reach for raw hex (`#1F1F1F`, `#2A2A2A`) or alpha shorthands like `rgba(255,255,255,0.1)` directly — go through the semantic token.
2.  **Unified radii:** Always reference `--radius-sm/md/lg/xl/full`. No literal `border-radius: 8px` anywhere outside `:root`.
3.  **Semantic font mapping:** Never apply `font-family` inline (`font-[var(--mono)]`, `font-mono`, etc.). Use `.font-code` or define a token-driven class. Heading-tier elements use `var(--font-family-display)`.
4.  **Text color tier tokens:** Don't reach for `var(--color-text-primary)` (white) on chrome — it should appear only on heading-tier rules. Prefer inheriting `--color-text-body` from `<body>`. Audit periodically with `rg 'color:\s*#fff' internal/web/src/` (must return zero matches in component CSS).
5.  **Editorial neutral escape hatch:** `--color-text-muted-neutral` is the *intentional* opt-out from the green tier (use sparingly — typically only when displaying neutral timestamps next to colored content). Do not reach for pure white instead.
6.  **Glass tokens stay in one place:** Every glassmorphic surface (dropdown, dialog, tooltip) reads from the same backdrop / surface / border / radius tokens. Wrap Melt builders in `src/lib/ui/*` so a token update propagates everywhere.
7.  **Motion variables, not magic numbers:** `transition: ... 150ms cubic-bezier(.4,0,.2,1)` is a smell — it should be `transition: ... var(--motion-duration-fast) var(--motion-ease)`.

---

## 5. Reference Implementation

The current state of the system lives in:

*   [`internal/web/src/app.css`](../internal/web/src/app.css) — every token plus all global / component CSS
*   [`internal/web/src/components/SessionList.svelte`](../internal/web/src/components/SessionList.svelte) — sidebar pattern (brand chip, search, list rows)
*   [`internal/web/src/components/SessionDetail.svelte`](../internal/web/src/components/SessionDetail.svelte) — page-title pattern + thread container
*   [`internal/web/src/components/StepTrace.svelte`](../internal/web/src/components/StepTrace.svelte) — chat bubbles + trace-row patterns
*   [`internal/web/index.html`](../internal/web/index.html) — favicon set + font preconnects
*   [`internal/web/public/`](../internal/web/public/) — favicon + 192/512 icons (the same app icon used inline as the brand mark and agent avatar)

The historical redesign that produced this state is documented in [`docs/plans/2026-05-02-web-ui-modal-redesign.md`](plans/2026-05-02-web-ui-modal-redesign.md). The Modal-derived extract this guide is based on remains in [`modal.com/design-system.md`](../modal.com/design-system.md) for cross-reference.
