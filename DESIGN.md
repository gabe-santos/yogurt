---
name: RSS Reader
description: A clean, lightweight, single-user feed reader with thoughtful UI details.
colors:
  background: "oklch(1 0 0)"
  foreground: "oklch(0.141 0.005 285.823)"
  card: "oklch(1 0 0)"
  card-foreground: "oklch(0.141 0.005 285.823)"
  popover: "oklch(1 0 0)"
  popover-foreground: "oklch(0.141 0.005 285.823)"
  primary: "oklch(0.21 0.006 285.885)"
  primary-foreground: "oklch(0.985 0 0)"
  secondary: "oklch(0.967 0.001 286.375)"
  secondary-foreground: "oklch(0.21 0.006 285.885)"
  muted: "oklch(0.967 0.001 286.375)"
  muted-foreground: "oklch(0.552 0.016 285.938)"
  accent: "oklch(0.967 0.001 286.375)"
  accent-foreground: "oklch(0.21 0.006 285.885)"
  destructive: "oklch(0.577 0.245 27.325)"
  border: "oklch(0.92 0.004 286.32)"
  input: "oklch(0.92 0.004 286.32)"
  ring: "oklch(0.705 0.015 286.067)"
typography:
  headline:
    fontFamily: "Geist Variable, sans-serif | Literata Variable, Georgia, serif (reading_font)"
    fontSize: "1.875rem"
    fontWeight: 600
    lineHeight: 1.25
  title:
    fontFamily: "Geist Variable, sans-serif"
    fontSize: "1rem"
    fontWeight: 500
    lineHeight: 1.5
  reading-body:
    fontFamily: "Geist Variable, sans-serif | Literata Variable, Georgia, serif (reading_font)"
    fontSize: "1.125rem"
    fontWeight: 400
    lineHeight: 1.625
  body:
    fontFamily: "Geist Variable, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Geist Variable, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.5
  mono-label:
    fontFamily: "ui-monospace, monospace"
    fontSize: "0.75rem"
    fontWeight: 400
rounded:
  sm: "4.32px"
  md: "5.76px"
  lg: "7.2px"
  xl: "10.08px"
  2xl: "12.96px"
  3xl: "15.84px"
  4xl: "18.72px"
spacing:
  chrome: "12px"
  row: "12px"
  reading-column: "24px"
  control-height: "32px"
  header-height: "48px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-foreground}"
    rounded: "{rounded.2xl}"
    height: "{spacing.control-height}"
    padding: "0 12px"
  button-primary-hover:
    backgroundColor: "{colors.primary}"
  button-outline:
    backgroundColor: "{colors.background}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.2xl}"
    height: "{spacing.control-height}"
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.foreground}"
    rounded: "{rounded.2xl}"
    height: "{spacing.control-height}"
  button-destructive:
    backgroundColor: "{colors.destructive}"
    textColor: "{colors.destructive}"
    rounded: "{rounded.2xl}"
    height: "{spacing.control-height}"
  input:
    backgroundColor: "{colors.input}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.2xl}"
    height: "{spacing.control-height}"
---

# Design System: RSS Reader

## Overview

**Creative North Star: "Clean, Lightweight, Thoughtful"**

This is the incumbent shadcn-svelte `rhea` system, on the `zinc` base color, refined without changing its identity: no brand hue has been introduced. The palette is neutral gray plus destructive red. Controls stay softly rounded against flat, bordered chrome; elevation is reserved for temporary floating surfaces. Geist Variable remains the interface voice. The Reading Pane alone can switch its own text to Literata Variable, a reader-chosen serif with optical sizing and real italics.

This suits the product truth in `PRODUCT.md`: success here is depth of reading, not a distinctive storefront. A single, private reader has no audience to persuade — the interface disappears during fast keyboard triage, then gives the longer reading session a deliberate typeface and measure without turning the product into a magazine imitation.

**Key Characteristics:**
- Achromatic by default: gray scale plus red-for-destructive, nothing else.
- Soft, consistent rounding (~13px controls, up to ~19-24px floating surfaces) against otherwise flat, bordered chrome.
- One interface typeface (Geist Variable); one optional Reading Font (Literata Variable); no display face.
- Borders carry structure; shadow is reserved for things that float above content.
- Component geometry and color remain the `rhea`/`zinc` foundation; reading typography is the deliberate override.

## Colors

Pure OKLCH neutral scale (chroma ≈ 0–0.02, hue ≈ 286) plus a single chromatic outlier for destructive actions. There is no Secondary or Tertiary in the Material sense — `secondary`, `muted`, and `accent` are the same neutral value used for different structural roles.

### Primary
- **Ink** (`oklch(0.21 0.006 285.885)` light / `oklch(0.92 0.004 286.32)` dark): near-black on light, near-white on dark. Used on `button-primary`, and as the unread-Entry dot — the only two places "primary" carries meaning rather than decoration.

### Neutral
- **Paper** (`oklch(1 0 0)` background/card/popover, light): the reading surface itself.
- **Ink** (`oklch(0.141 0.005 285.823)` foreground, light): body and heading text.
- **Fog** (`oklch(0.967 0.001 286.375)` secondary/muted/accent, light): row hover, active tab, secondary-button fill.
- **Ash** (`oklch(0.552 0.016 285.938)` muted-foreground, light): metadata — Feed name, timestamp, excerpt text, unstarred/unarchived affordances.
- **Hairline** (`oklch(0.92 0.004 286.32)` border/input, light): every structural divider — pane borders, `divide-y` Entry List rows, chrome bottom-borders.

Dark mode inverts lightness on the same near-zero-chroma hue (`app.css:45-79`); it is not a separate palette, and no color role changes meaning between the two.

### Named Rules
**The No-Hue Rule.** No brand accent color exists anywhere in this codebase today. `--primary` is ink, not a hue. Introducing one is a deliberate future decision (a `colorize` pass), not something to infer from a neighboring reader app or a generic default. Confirmed with the product owner: stay achromatic until a real reason to depart is named.

**The Red-Means-Loss Rule.** `destructive` (`oklch(0.577 0.245 27.325)` light / `oklch(0.704 0.191 22.216)` dark) is the only chromatic token in the system. It appears solely on genuinely destructive or error affordances — never as emphasis, never as a second accent.

## Typography

**Interface Font:** Geist Variable (with `sans-serif` fallback) — every pane, control, label and dialog, without exception.
**Reading Font:** the reader's choice of Geist Variable or Literata Variable (with `Georgia, serif` fallback), stored server-side as the `reading_font` preference and applied only to the Reading Pane's own text.
**Label/Mono Font:** system `ui-monospace` (the `<kbd>` shortcut glyphs in the Help dialog only)

**Character:** A single, quiet grotesque runs the interface — headline, title and label are the same face at different sizes and weights, so nothing in the chrome competes with the Entry being read. The one place a second family is permitted is the text of the Entry itself, and only because the reader asked for it. No display face, no letter-spacing or uppercase transforms anywhere in the UI.

Both faces are bundled (`@fontsource-variable/*`, latin subset ~29KB sans / ~110KB serif) and served from the binary; nothing is fetched from a CDN. Each ships a **drawn italic**, not a synthesised slant — extracted Articles are full of `<em>`, so the italic stylesheet is imported alongside the upright for both.

### Hierarchy
- **Headline** (600, 1.875rem/30px, 1.25 line-height): the open Entry's own title at the top of the Reading Pane. Set in the chosen Reading Font. The one place type is allowed to be loud.
- **Title** (500, 1rem/16px, 1.5 line-height): the Entry List's scope heading, the Reading Pane's collapsed sticky-bar title, and each Entry row's own title line. Always the Interface Font.
- **Reading Body** (400, 1.125rem/18px, 1.625 line-height "relaxed"): Reader View and Feed View content, in the chosen Reading Font. Prose headings step to 24px (`h2`) and 20px (`h3`) above it; paragraphs, lists and blockquotes are separated by 16px of space and never also indented.
- **Label** (400, 0.75rem/12px, 1.5 line-height): Feed name, timestamp, Excerpt text, filter tabs, all metadata rows — including the Reading Pane's own metadata line under the headline, which is chrome and stays in the Interface Font. Always paired with the Ash (`muted-foreground`) color, never Ink at this size.

### Named Rules
**The One Interface Voice Rule.** Geist Variable is the only typeface in the interface. Every control, label, pane and dialog is set in it, and a second family in the chrome is a defect. This rule stops at the Reading Pane's own text and nowhere earlier.

**The Measure-Follows-the-Face Rule.** The reading column is sized in characters, not pixels: `--container-reading-sans` (39.5rem) and `--container-reading-serif` (40.625rem) each hold ~72 characters of their own face at 18px, measured rather than guessed. A shared cap would give the two faces different measures, and a shared `ch` cap would be worse — `ch` is the width of a zero, and the ratio of average character to zero runs 0.68 in Geist against 0.80 in Literata. A new reading face gets its own measured token.

**The Font-Does-Its-Own-Work Rule.** The `tracking-*` scale exists because Geist has no optical-size axis and needs headings pulled tight by hand. Literata has one, is drawn correctly at every size, and therefore takes none of that scale. Never apply the tracking tokens to a face with an `opsz` axis.

**The Dark-Serif Compensation.** Light text on a dark surface reads thinner than the same text inverted, and a serif's thin strokes show it first. On dark, the reading serif takes 420 of its 200–900 weight axis and 1.75 line-height. Geist's strokes are uniform enough to need neither, and gets neither.

## Layout

Three fixed structural regions at desktop width (`lg:` and up, 1024px Tailwind breakpoint): a collapsible Feed List sidebar, a fixed `w-88` (352px) Entry List column, and a Reading Pane that fills the remaining width. Below `lg`, the Reading Pane becomes a full-bleed `absolute inset-0` overlay above the Entry List rather than a second column — one component, two containers, never two implementations (ADR-0009).

The page itself does not scroll (`h-svh`, `overflow-hidden` on the shell); each pane owns its own internal scroll region, so the chrome around it — header bars, filter tabs — stays fixed while content moves underneath.

**Spacing rhythm:** chrome padding (headers, list gutters) sits at 12px (`p-3`/`px-3`); Entry rows use 12px vertical rhythm (`py-3`) separated by 1px hairline dividers (`divide-y divide-border`), never a gap plus rounded card. The reading column uses 24px horizontal padding and an 18px body: the measured sans/serif caps are 632px and 650px respectively, each yielding ~74 characters per line in real Article copy. Controls are uniformly 32px tall (`h-8`); header bars are 48px (`h-12`).

## Elevation & Depth

Structural chrome is flat: panes, headers, and the sidebar are separated by 1px borders, never shadows. Shadow is reserved exclusively for surfaces that temporarily float above the rest of the interface — dialogs, sheets, dropdown/menu content — and disappears the instant that surface closes.

### Shadow Vocabulary
- **Overlay** (`shadow-xl`): Dialog, Sheet, and menu/dropdown content. The signal that this surface is temporarily on top of everything else.
- **Floating sidebar ring** (`shadow-sm` + `ring-1 ring-sidebar-border`): only the `floating`/`inset` Sidebar variants, which visually detach the sidebar from the page edge.

### Named Rules
**The Temporary-Only Rule.** Shadow means "this is above everything else right now," never permanent surface styling. The three-pane reading layout itself — Feed List, Entry List, Reading Pane — stays flat at every width; only its transient overlay form (the narrow-width Reading Pane) uses `z-30` stacking, and even that relies on background + border, not a shadow, to read as "in front."

## Shapes

Soft, uniform rounding scaled from one base token (`--radius: 0.45rem`/7.2px) rather than per-component values: buttons, inputs, and tabs round at ~13px (`--radius-2xl`); dialogs cap at 24px (`min(--radius-4xl, 24px)`); small chips like the Feed Icon monogram round at ~4px (`--radius-sm`). Borders are 1px and hairline-colored throughout; no component uses a heavier or colored border. Nothing in the interface uses a hard, unrounded corner outside the Entry List's own edge-to-edge container.

## Components

### Buttons
- **Shape:** rounded-2xl (~13px), 32px tall by default (`xs`/`sm`/`lg`/icon sizes scale 24–36px).
- **Primary:** Ink background, inverted (paper) text, `hover:bg-primary/80`.
- **Secondary:** Fog background, Ink text.
- **Outline:** transparent background, hairline border, Fog hover fill.
- **Ghost:** transparent at rest, Fog hover fill — the default for every icon-only control in both header bars.
- **Destructive:** red at 10% opacity fill, red text — never a solid red fill; the softness is deliberate restraint on the app's one chromatic token.
- **Hover / Active:** hover swaps to the role's fill color; pressed state nudges 1px down (`active:translate-y-px`) rather than scaling or shadowing — a tactile "pressed" cue with no added depth.

### Inputs / Fields
- **Style:** Fog background at 50% opacity (`bg-input/50`), transparent border, rounded-2xl, same 32px height as buttons so a search field and its adjacent button share a baseline.
- **Focus:** border shifts to `ring` color plus a 3px ring at 30% opacity — no glow, no background change.
- **Error:** border and ring both shift to destructive at reduced opacity, matching the button's restrained-red treatment.

### Navigation (Feed List / sidebar)
- **Style:** flat at rest, Fog hover/active fill, no border between items — hierarchy comes from indentation and grouping, not dividers.
- **Active state:** background fill, not a colored indicator bar or icon-color change — consistent with the No-Hue Rule.

### Entry Row (signature component)
The list's core unit, and deliberately not a card: no border, no radius, no shadow — just a full-width row separated from its neighbors by a 1px hairline (`divide-y`). A 6px unread dot in Ink is the only differentiator between read and unread besides text weight (unread titles stay full-weight Ink; read titles drop to Ash). Feed Icon (16px, rounded-sm, monogram fallback), Feed name, and timestamp share the 12px Label row; Star/Archive glyphs float right, shown only when set. Title is Title-weight (500) at body size; a two-line-clamped Excerpt below it is Label-sized and Ash-colored, and the row skips that line entirely rather than reserving empty space when an Entry has no Excerpt.

### Reading Pane (signature component)
Two states of one component, never two implementations: a static third column at desktop width, a full-bleed `inset-0` overlay below it, with the Entry List beneath it marked `inert` so tab order never leaks out of the overlay. A 48px header holds back-navigation (mobile only), a title that only appears once the real headline has scrolled out of view, the three-way view switcher (Feed / Reader / Original), and the Star/Archive/Read/open-in-new-tab controls — all `ghost` icon buttons at `icon-sm`, grouped with a single hairline divider between “view” and “state” controls. Below it, Reader/Feed View use the chosen face's measured reading cap (632px Geist / 650px Literata); Original View is full width and keeps the publisher's own typography.

## Do's and Don'ts

### Do:
- **Do** keep the palette achromatic: Ink, Paper, Fog, Ash, Hairline, and destructive red are the entire vocabulary. Adding a role means reaching for one of these first.
- **Do** use `ghost` buttons for every icon-only control in header/toolbar contexts; reserve `outline` for content-area secondary actions (Load more, Open in a new tab) and `default`/primary for the single most-committal action on a screen.
- **Do** separate structural surfaces with 1px hairline borders, not shadows.
- **Do** use shadow only on content that floats above the page and disappears when dismissed (dialogs, sheets, menus).
- **Do** build one component with two containers (pane vs. overlay) for anything that must work at every width, per ADR-0009 — never a second, parallel implementation for narrow screens.
- **Do** cap Reader/Feed View prose with the reading-measure token for the chosen face (`max-w-reading-sans` / `max-w-reading-serif`); leave Original View full-width, since it renders the publisher's own layout.
- **Do** use the domain vocabulary from `CONTEXT.md` in every label, test id, and component name (Feed List, Entry List, Reading Pane, Starred, Archived — never "sidebar," "drawer," or "bookmark").

### Don't:
- **Don't** introduce a brand accent hue without an explicit decision to depart from the No-Hue Rule — it is not a placeholder waiting to be filled in.
- **Don't** put a solid destructive-red fill on a button; the system's one chromatic token stays at reduced opacity everywhere it appears.
- **Don't** wrap Entry rows in cards, add per-row radius, or add per-row shadow — the list is one continuous hairline-divided column, not a stack of cards.
- **Don't** reach for a second typeface in the interface. Chrome hierarchy comes from Geist Variable's own weight and size steps (400/500/600 at 12/16/30px), not from mixing faces; the Reading Font is the single, reader-chosen exception and never leaves the Reading Pane's own text.
- **Don't** hard-code a pixel width for the reading column, or share one across faces — see the Measure-Follows-the-Face Rule.
- **Don't** uppercase or letter-space labels; none of the incumbent UI does, and Excerpt/metadata rows read as plain sentence case throughout.
