# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

One user: the person who runs the instance, who is also the person who built it. There is no second audience, no invite flow, and no notion of an account other than his own (`docs/adr/0002-single-user.md`). Read, Starred and Archived are therefore columns on the Entry itself, not per-user rows.

He follows publications directly rather than through an algorithmic feed, and his reading is split across two distinct sessions:

- **Triage.** A fast pass over what arrived. Keyboard-driven, often short, often on a phone browser. The output is a decision per Entry — keep it, dismiss it, or leave it — not the reading itself.
- **Reading.** A separate, longer session at a desk, spent actually reading a small number of pieces properly.

Both are first-class. Neither may be compromised to serve the other; a change that makes triage faster at the cost of the reading session (or the reverse) is a regression, not a tradeoff.

## Product Purpose

Collect the feeds one person follows into one place, and present each item either as extracted text or as the publisher's own page.

Success is depth: a year in, he reads *more of what he deliberately chose to follow*. The failure mode to design against is this app becoming another surface he skims and abandons. Coverage, throughput and the unread count are instruments, not the goal — a cleared backlog that produced no real reading is a failure.

## Positioning

Two things a neighbouring reader could not truthfully copy at once:

1. **Both views, in one app, as a first-class choice.** Reader View re-renders an Article's main text and images in the app's own typography; Original View embeds the publisher's page as they laid it out. Most Articles read best stripped; a meaningful share — image-heavy, interactive, JavaScript-driven — are unreadable once extracted, and losing your place to a browser tab is the failure this exists to remove. The choice is remembered as a preference rather than re-asked per Entry.
2. **Self-hosted, single binary, one file.** One process, no runtime dependencies, no database server, all state in one SQLite file the owner backs up himself. Reading history is not rented (`docs/adr/0001-own-feed-engine.md`, `docs/adr/0006-go-sqlite-sveltekit-single-binary.md`).

Because Original View is an embed and roughly a quarter of popular publishers forbid embedding, **Reader View carries the product**. Extraction quality is the load-bearing quality attribute.

## Operating Context

Today: a laptop browser, plus a phone browser for triage — the layout is required to work there (`docs/prd/0001-reader-mvp.md`, story 81). The instance runs on the owner's own machine.

The reading surface is three panes over one Entry: a **Feed List**, an **Entry List**, and a **Reading Pane**. Below the width for three columns, the Reading Pane is the same component rendered as an overlay over the Entry List — deliberately not a Sheet, so that every future reading feature is built and tested once rather than twice (`docs/adr/0009-three-panes-one-reading-surface.md`).

Because the Reading Pane is permanently populated, **selection is opening**: Read is set the moment an Entry reaches the pane, gated by the `mark_on_open` setting. A consequence that looks like a bug and is not: under the Unread filter, the Entry being read stays in the list until the reader moves off it. Only the reader's own triage — Archive, Star, a manual Mark read — removes an Entry from a view (`docs/adr/0010-selection-is-opening.md`).

Planned order of later phases, each parked rather than in flight: an Electron desktop shell (`docs/adr/0007-electron-over-tauri-for-desktop-shell.md`), then a phone home-screen app gated on TLS. The desktop shell is what upgrades Original View to a top-level webview where embedding restrictions do not apply. Neither shell changes the design language: the shell is thin and the UI inside it is this same web SPA (`docs/adr/0005-thin-desktop-shell-and-device-tokens.md`) — hence Platform `web`, not `adaptive`.

## Capabilities and Constraints

Vocabulary is fixed and load-bearing. `CONTEXT.md` is the authority: Feed, Feed Icon, Entry, Article, Preview Image, Excerpt, Snippet, Feed View, Reader View, Original View, Feed List, Entry List, Reading Pane, Read, Starred, Archived. It also lists, per term, the words to avoid — "sidebar", "drawer", "thumbnail", "bookmark", "folder", "item" and the rest. Copy, labels, component names and tests use the domain word.

Confirmed behaviour, in the reader's terms:

- Feeds are discovered from a site address or added by Feed URL, validated before saving, renameable, deletable, and importable/exportable as OPML.
- Polling is scheduled, conditional, and politely backed off per publisher hints; a Feed's last check, last success and last error are visible so that silence is distinguishable from breakage.
- The Entry List is newest-first, filterable to All / Unread / Starred, scopeable to a Feed or Group, with per-Feed and per-Group unread counts and cursor pagination.
- Three states only: Read, Starred, Archived. Starring *is* keeping — there is no separate read-later, and Starred Entries plus their extracted Articles are exempt from cleanup. Archiving implies Read and removes the Entry from every view except the archive.
- Everything is operable from the keyboard, from one binding table that also drives the help dialog. Full-text search covers Entry titles and Feed-supplied content, and doubles as navigation by matching Feed names.
- Major UI state lives in the URL — filter, Feed/Group scope, open Entry — so any position is bookmarkable and survives reload.
- State changes appear instantly and roll back if the server rejects them. Reads are delta-shaped; mutations are idempotent state declarations, not toggles (`docs/adr/0004-online-first-with-delta-api.md`).
- All Entry and Article HTML is sanitised server-side; tracking pixels are stripped and images carry a no-referrer policy. Original View is an isolated embed of the live publisher URL with an explicit "open in new tab" fallback where embedding is refused — never a blank frame (`docs/adr/0003-original-view-is-an-iframe.md`).
- Retention removes old unstarred Entries automatically on a configurable schedule.

Hard constraints:

- One binary, one SQLite file, environment-variable configuration with working defaults, schema migrated on startup. Static SPA: no SSR, no framework server.
- Online-first. There is no offline mode, no service worker, and no installability work in scope.
- Out of scope and not to be reintroduced by implication: multiple users or roles, offline reading, Fever/Google Reader API compatibility, newsletter ingestion, podcasts, an image proxy, reverse-proxy rendering, stored page snapshots, headless-browser rendering, per-site extraction rules.

## Brand Commitments

**The product is named Yogurt.** It is one word, with no "Reader" or "RSS" suffix: "Reader" is the person the app serves (and a word already used in Reader View), and Feeds are not only RSS. Where a descriptor is needed, write it in lowercase prose ("Yogurt, a feed reader"). Identifiers use lowercase `yogurt`: the repo, Go module, binary, `YOGURT_*` environment variables, session cookie, database file and outbound User-Agent.

No logo, wordmark, brand voice or identity constraint has been established.

## Evidence on Hand

- **Real product truth, written down and unusually complete:** `CONTEXT.md` (domain language), `docs/prd/0001-reader-mvp.md` (82 user stories plus implementation and testing decisions), `docs/adr/0001`–`0012`.
- **A running instance with real but small data:** `data/yogurt.db` holds 11 Feeds, 662 Entries, 2 unread, 0 Starred (checked 2026-09-24). The "few hundred sites" in the PRD is the intended scale, not the present state — so any layout claim about large collections or heavy Starred use is untested against real data and must not be presented as observed.
- **Tests as behavioural evidence:** Go API tests (`internal/apitest`), a pure pull-policy suite, and thin Playwright journeys in `web/e2e/` (`reading`, `responsive`, `views`, `unread`) against the real binary with a fake publisher.
- **Absent, and not to be fabricated:** no users besides the owner, no usage analytics, no testimonials, no press, no benchmarks, no pricing, no public deployment, no logo or brand assets, and no hosted service.

## Product Principles

1. **Reading is the point; triage is in service of it.** The unread count is an instrument, never a scoreboard. Prefer the change that produces more real reading over the one that empties the list faster.
2. **Two sessions, one app.** Triage and reading are distinct modes with distinct rhythms and often distinct devices. Serve both; never buy speed in one with friction in the other.
3. **The domain word wins.** `CONTEXT.md`'s vocabulary governs copy, labels, components and tests. If a term is missing, add it there rather than coining a synonym in the UI.
4. **Build the reading surface once.** One component with two containers, not a wide implementation and a narrow one. Anything built twice will drift and will be tested half as well.
5. **Never a dead end.** When extraction fails, embedding is refused, or a Feed breaks, say so plainly and offer the way out. A blank frame, a silent failure, or an ambiguous empty state is a defect.
6. **The owner keeps his data and his place.** One file to back up, position in the URL, no lock-in, no dependence on anyone else's service staying alive.

## Accessibility & Inclusion

- **Full keyboard operability is a product requirement, not an enhancement.** Next/previous Entry, open/close, toggle Read/Starred/Archived, and switching between Unread/All/Starred are all reachable from the keyboard, and every shortcut is discoverable in a help dialog rather than only in the source.
- **Light and dark both ship, following the system preference,** because reading happens at night.
- The narrow-width Reading Pane overlay marks the Entry List `inert` to keep tab order inside the pane; that containment is a requirement of the pattern, not an implementation detail.
- No further person-specific accessibility need has been established. Standard practice applies; nothing here licenses skipping it.
