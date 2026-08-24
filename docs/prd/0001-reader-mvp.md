# Self-hosted RSS Reader — MVP

## Problem Statement

I follow a few hundred sites and have no single place to read them. Content arrives scattered across browser tabs, bookmarks I never revisit, and newsletters that bury my personal inbox. Hosted readers mean handing my reading history to someone else and paying rent on it.

When I do read, one view is never enough. Most articles are best stripped to text, but a meaningful share — heavy on images, interactive charts, or JavaScript-driven layout — are unreadable once extracted, and today my only option is to leave the reader entirely and lose my place.

I want to run this myself, on my own machine, with my data in a file I can back up. I read on a laptop browser now, but I want a desktop app later and my phone's home screen after that, and I don't want the first version to make either impossible.

## Solution

A single self-hosted binary that polls the Feeds I subscribe to, stores every Entry in one SQLite file, and presents them in a fast, keyboard-driven reading view.

I log in with a password. I paste a site's address and the app finds its Feed for me, or I import an OPML file and get my whole existing collection at once. Feeds sit in flat Groups so I can read just one part of my collection. The app checks Feeds on a schedule, politely, backing off when a publisher is slow or broken, and shows me when a Feed has been failing.

Each Entry can be read two ways. **Reader View** fetches the Article and renders its main text and images in the app's own typography. **Original View** shows the publisher's page as they laid it out, embedded in the app. Where a publisher forbids embedding, the app tells me so and offers to open the page in a new tab rather than showing me a blank frame.

Entries are Read, Starred, or Archived. Read is set when I open something (or not, if I turn that off in settings) and always overridable by hand. Starring is how I keep a thing — starred Entries and their extracted Articles are never cleaned up. Archiving is how I dismiss a thing — it leaves every view except the archive. Everything is reachable from the keyboard, and full-text search finds any Entry I've received.

## User Stories

**Deployment and configuration**

1. As a self-hoster, I want to run the whole app as a single binary with no runtime dependencies, so that I don't have to operate a database server or a language runtime.
2. As a self-hoster, I want to run the app as a single container with one mounted volume, so that my deployment is one file to back up.
3. As a self-hoster, I want all configuration to come from environment variables with working defaults, so that I can start the app with one setting.
4. As a self-hoster, I want the database schema to be created and migrated automatically on startup, so that upgrading is just replacing the binary.
5. As a self-hoster, I want structured logs at a configurable level, so that I can diagnose a misbehaving Feed without a debugger.
6. As a self-hoster, I want the app to refuse to fetch Feeds on private network addresses by default, so that a malicious Feed URL can't probe my LAN.
7. As a self-hoster, I want to override the listening port and data directory, so that it fits alongside other services I run.

**Authentication and access**

8. As the reader, I want to log in with a password, so that exposing the app doesn't expose my reading.
9. As the reader, I want my session to persist across browser restarts, so that I'm not logging in daily.
10. As the reader, I want to log out explicitly, so that I can end a session on a machine that isn't mine.
11. As the reader, I want repeated failed logins to be rate limited, so that a public instance can't be brute forced.
12. As the reader, I want to create a named device token and revoke it later, so that a future desktop app or third-party client can authenticate without my password.
13. As the reader, I want to see when each device token was last used, so that I can tell which one to revoke.

**Adding and managing Feeds**

14. As the reader, I want to paste a site's home page address and have the app discover its Feed, so that I don't have to hunt for the Feed URL myself.
15. As the reader, I want to paste a Feed address directly, so that discovery isn't in my way when I already know it.
16. As the reader, I want the app to validate a Feed before saving it and tell me plainly when it isn't one, so that I don't add something broken.
17. As the reader, I want to be warned when I add a Feed I already subscribe to, so that I don't get duplicates.
18. As the reader, I want to rename a Feed, so that a publisher's terrible title isn't permanent.
19. As the reader, I want to delete a Feed and have its Entries go with it, so that removing something actually removes it.
20. As the reader, I want to suspend a Feed without deleting it, so that a noisy site can go quiet while keeping its history.
21. As the reader, I want to create, rename and delete Groups, so that my collection has structure.
22. As the reader, I want every Feed to belong to exactly one Group, with a default Group for anything I haven't sorted, so that nothing is ever unreachable.
23. As the reader, I want deleting a Group to move its Feeds to the default Group rather than deleting them, so that a mis-click can't destroy my subscriptions.
24. As the reader, I want to import an OPML file, so that I can move my collection here from another reader in one step.
25. As the reader, I want OPML import to report what it added and what it skipped, so that I know whether it worked.
26. As the reader, I want nested OPML folders flattened predictably into Groups, so that an import from a deeply nested collection is still usable.
27. As the reader, I want to export my collection as OPML, so that I am never locked in.

**Feed polling**

28. As the reader, I want Feeds checked automatically on a schedule, so that new Entries arrive without me asking.
29. As the reader, I want the app to send conditional requests and treat an unchanged response as a successful check, so that I'm not wasting publishers' bandwidth or mine.
30. As the reader, I want the app to honour a publisher's caching and retry hints, so that my instance is a well-behaved client.
31. As the reader, I want a failing Feed to be retried progressively less often, so that a dead site doesn't get hammered forever.
32. As the reader, I want to see a Feed's last check time, last success, and last error, so that I can tell whether silence means "no news" or "broken".
33. As the reader, I want to trigger a refresh of one Feed on demand, so that I can pull a story I know just published.
34. As the reader, I want to trigger a refresh of everything on demand, so that I can catch up immediately after opening the app.
35. As the reader, I want manual refresh to bypass the backoff schedule, so that "refresh" means refresh.
36. As the reader, I want an Entry that appears twice in a Feed to be stored once, so that my unread count is truthful.
37. As the reader, I want an Entry that a publisher later edits to update in place rather than arrive again, so that my list isn't full of duplicates.
38. As the reader, I want concurrent fetching with a bounded limit, so that a refresh of hundreds of Feeds is quick without melting my connection.

**Reading**

39. As the reader, I want a list of Entries newest-first, so that the most relevant thing is at the top.
40. As the reader, I want to filter that list to All, Unread, or Starred, so that I can choose between triage and browsing.
41. As the reader, I want to scope the list to one Feed or one Group, so that I can read a single publication in isolation.
42. As the reader, I want unread counts per Feed and Group, so that I can see where the new material is.
43. As the reader, I want the list to page in more Entries as I scroll, so that a long backlog doesn't have to load at once.
44. As the reader, I want the list position and open Entry encoded in the URL, so that I can bookmark or reload without losing my place.
45. As the reader, I want to open an Entry and read the content the Feed supplied, so that a full-text Feed needs nothing further.
46. As the reader, I want to move to the next and previous Entry from within an open Entry, so that I can work through a backlog without returning to the list.
47. As the reader, I want to see which Feed an Entry came from and when it was published, so that I can judge it before reading.
48. As the reader, I want the publisher's own HTML rendered in consistent, readable typography, so that every Entry reads like it belongs to the same app.
49. As the reader, I want scripts and dangerous markup stripped from Entry and Article content, so that a hostile Feed can't attack my browser.
50. As the reader, I want invisible tracking pixels removed and referrer information withheld when images load, so that reading doesn't report back to the publisher.

**Reader View and Original View**

51. As the reader, I want to request Reader View for an Entry whose Feed only gave me a summary, so that I can read the whole thing without leaving the app.
52. As the reader, I want the extracted Article kept after the first fetch, so that reopening it is instant and doesn't hit the publisher again.
53. As the reader, I want to be told clearly when extraction fails, so that I can fall back rather than stare at an empty pane.
54. As the reader, I want to switch an Entry to Original View, so that an image-heavy or interactive Article looks the way its author intended.
55. As the reader, I want the app to know in advance that a publisher forbids embedding and offer "open in new tab" instead, so that I never see a blank frame and wonder if the app is broken.
56. As the reader, I want my choice between Reader View and Original View remembered as a preference, so that I'm not re-toggling on every Entry.
57. As the reader, I want embedded Original View content isolated from the app, so that a publisher's page can't reach my session.

**Entry state**

58. As the reader, I want an Entry marked Read when I open it, so that triage happens as a side effect of reading.
59. As the reader, I want to turn off mark-on-open in settings, so that I can browse without destroying my unread list.
60. As the reader, I want to mark any Entry read or unread by hand at any time, so that the automatic behaviour is never the last word.
61. As the reader, I want to mark everything in the current view read in one action, so that declaring bankruptcy on a backlog is one click.
62. As the reader, I want mark-all-read to respect my current filter and scope, so that it never marks more than I'm looking at.
63. As the reader, I want to star an Entry, so that I can keep something worth returning to.
64. As the reader, I want a view of just my Starred Entries, so that my keeps are a place I can browse.
65. As the reader, I want Starred Entries and their extracted Articles exempt from cleanup, so that keeping something means keeping it.
66. As the reader, I want to archive an Entry, so that something I'm finished with stops appearing in front of me.
67. As the reader, I want archiving to imply Read and to remove the Entry from every view except the archive, so that "archive" means gone.
68. As the reader, I want a view of Archived Entries, so that dismissing something isn't destroying it.
69. As the reader, I want state changes to appear instantly and to correct themselves if the server rejects them, so that the app never feels laggy or lies to me.
70. As the reader, I want old unstarred Entries cleaned up automatically after a configurable period, so that the database doesn't grow forever.

**Search**

71. As the reader, I want to full-text search the Entries I've received, so that I can find something I half-remember.
72. As the reader, I want search to cover titles and the content the Feed supplied, so that results are consistent and complete.
73. As the reader, I want to search my Feeds by name, so that search doubles as navigation in a large collection.
74. As the reader, I want a search result to open the Entry in its list context, so that finding something doesn't strand me.
75. As the reader, I want search reachable from a keyboard shortcut anywhere in the app, so that it's the fastest way to move around.

**Keyboard and layout**

76. As the reader, I want to move to the next and previous Entry from the keyboard, so that I can triage with one hand.
77. As the reader, I want to open and close an Entry from the keyboard, so that reading never requires the mouse.
78. As the reader, I want to toggle Read, Starred and Archived from the keyboard, so that triage is as fast as I can type.
79. As the reader, I want to jump between Unread, All and Starred views from the keyboard, so that changing mode is instant.
80. As the reader, I want a discoverable list of every shortcut, so that I don't have to read the source to learn the app.
81. As the reader, I want the layout to work on a phone browser, so that the eventual home-screen app has something worth installing.
82. As the reader, I want light and dark themes following my system preference, so that reading at night doesn't hurt.

## Implementation Decisions

**Overall shape.** One process running two long-lived services over one SQLite file: an HTTP API and a Feed pull worker. The compiled frontend is embedded into the binary and served by it. Deployment is one binary or one container with a single data volume. Per ADR-0006 the stack is Go plus SQLite via a pure-Go driver (no cgo), with a static SvelteKit SPA; per ADR-0001 the Feed engine is ours, with fusion (`github.com/0x2E/fusion`) as the behavioural reference for pull scheduling, fetch state and search.

**Modules.** Config from environment; HTTP handlers with auth middleware; a store owning SQL and embedded migrations; a pull package doing fetch, parse and upsert; a separate pure pull-policy package computing schedule decisions; an auth package for password, session and device tokens; an extraction package fetching and cleaning Articles; a search package over the FTS index; an OPML import/export package; and the frontend SPA.

**Schema.** Groups; Feeds (unique on URL, one Group, a suspend flag, reserved `kind` column for the deferred newsletter source type per ADR-0001's successor work); a per-Feed fetch-state row keyed by Feed holding validators, cache hints, next-check time, last status, last error and consecutive failure count; Entries (unique per Feed on the publisher's identifier, with Read, Starred and Archived flags plus timestamps); Articles (extracted HTML, an embedding-allowed flag, fetch time, keyed by URL); an FTS virtual table over Entry title and Feed-supplied content kept in sync by triggers; sessions; device tokens stored hashed; and settings.

**Explicit SQL, no ORM,** and cascade behaviour written into store transactions rather than relying on database-level side effects, so that deletion order is readable in one place.

**API contract.** Session login and logout; Groups and Feeds CRUD plus validate, batch create and refresh; Entry list with opaque cursor pagination; Article retrieval that extracts on first request and serves the stored copy afterwards; unified search; OPML import and export; device token create, list and revoke. Per ADR-0004 reads are delta-shaped — a `since` cursor returning changed Entries plus tombstones for cleaned-up ones — and mutations are idempotent state declarations carrying the desired Read, Starred and Archived values rather than toggles. Bulk mark-read takes the same filter and scope parameters as the list endpoint so that it cannot affect more than the caller is looking at.

**Authentication.** Password supplied by configuration, hashed at startup, never stored in plaintext. Browsers authenticate with an HttpOnly, SameSite=Lax session cookie, Secure over HTTPS. Per ADR-0005 the API also accepts a hashed long-lived device token in an Authorization header, because a future desktop shell's origin differs from the server's and cookies will not be sent; the mechanism ships in the MVP with no consumer.

**Pull behaviour.** Conditional requests using stored validators, with an unchanged response counted as a successful check. Next check time is the strictest of the configured interval, the publisher's retry hint, and the publisher's freshness hints, capped globally; on failure it is the strictest of interval, retry hint and exponential backoff on consecutive failures, capped the same way. Failure counters and next-check time update in a single transaction. Freshness metadata is only refreshed on successful checks. Suspended Feeds are skipped unconditionally; manual refresh bypasses skip logic entirely. Feed fetches run through an HTTP client that blocks private and link-local destinations by default.

**Extraction and rendering.** Articles are extracted on demand when the reader asks for Reader View, not on ingest, and the result is stored and reused. The same fetch inspects response headers for embedding restrictions and records whether Original View may embed the page, so the UI never has to guess by watching a frame fail. All Entry and Article HTML is sanitised server-side against an allowlist; one-pixel tracking images are stripped; images carry a no-referrer policy; insecure image sources are upgraded or dropped. There is no image proxy in the MVP.

**Original View.** Per ADR-0003, an embed of the live publisher URL, isolated so that it cannot reach the app's origin or session, with a fallback affordance to open the page in a new tab where embedding is refused. No reverse proxy, no stored page snapshots, no headless browser — the path to fidelity is the future native shell.

**Retention.** A periodic cleanup removes Entries older than a configurable age, exempting Starred; Archived Entries expire on the normal schedule. An extracted Article outlives its Entry only when that Entry is Starred. Cleanup emits tombstones consistent with the delta contract.

**Frontend.** A static SPA with client-side routing, no server-side rendering and no framework server, per ADR-0006. Major UI state lives in the URL: filter as a path segment, Feed or Group scope as path segments, open Entry as a search parameter. Server data goes through a query cache providing cursor pagination, optimistic mutation with rollback, and invalidation after mutations; transient UI state stays local. Components come from the shadcn-svelte registry on Tailwind. Keyboard handling is global and derived from a single binding table that also drives the help dialog.

## Testing Decisions

**What makes a good test here.** A test drives the app the way a caller does and asserts on what the caller can observe. It names a behaviour, not a function. It does not reach into storage to check a column, does not assert on log output, and does not mock our own modules — the only test doubles are for things outside the process: publishers and the clock. Every test is deterministic and hermetic: no real network, no wall-clock sleeps, no shared database.

**Seam 1 — the HTTP API (primary).** Boot the real application against a temporary database with an injected clock, and point it at an in-process HTTP server that plays the publisher. Drive real requests. This is the highest available seam and it is where nearly all behaviour is proven: Feed creation, discovery and validation; conditional requests and unchanged-response handling; duplicate and edited Entries; backoff progression across simulated failures; suspend and manual refresh; list filtering, scoping and cursor pagination; mark-read scoping; state transitions including archive implying read; idempotent replay of the same state mutation; delta reads returning tombstones after cleanup; retention exempting Starred; extraction on first Article request and reuse thereafter; recording of embedding permission; sanitisation of hostile markup; search behaviour including index updates on Entry change and deletion; OPML round-trip including nested-folder flattening; cookie versus device-token authentication; token revocation; and login rate limiting. Blocking of private-network Feed URLs is tested here too, since it is observable as a rejected request.

**Seam 2 — the pull-policy function (pure).** Next-check computation is a pure function of current time, configuration, publisher hints and failure count. It is tested directly with table-driven cases: interval versus retry hint versus freshness hint precedence, backoff growth, the global cap, and the difference between success and failure branches. This seam exists because proving time arithmetic through HTTP would need either sleeps or a contrived clock dance for every case.

**Seam 3 — the browser (thin).** Playwright against the real binary with the fake publisher, covering only journeys that do not exist below the UI: logging in, adding a Feed and seeing its Entries, opening an Entry and reading it, switching to Original View and getting the fallback when embedding is refused, keyboard navigation and triage, and search. A handful of journeys, not a mirror of the API suite.

**Not tested.** No unit tests over the store, no tests of individual handlers with a mocked store, no component tests, no snapshot tests of markup. Adding a fourth seam requires justifying why the behaviour cannot be observed at one of these three.

**Prior art.** There is no code in this repository yet, so the reference is fusion, whose backend keeps pull-scheduling policy in a dedicated pure package tested in isolation and exercises the rest through the API. Its release checklist — build, full test run, migration bootstrap on an empty database, and an API smoke path of login, create Feed, refresh, search — is the model for this project's verification.

## Out of Scope

Multiple users, registration, invitations, roles and OIDC (ADR-0002). Offline reading, service worker, web manifest and any home-screen installation work — including the TLS termination it requires. The desktop shell itself; only the device-token mechanism it will need ships now (ADR-0005). Fever and Google Reader API compatibility. Newsletter ingestion, generated email addresses, inbound mail and everything downstream of it; only a reserved Feed-kind column anticipates it. Podcasts and audio playback, YouTube and social timelines, and scraping sites that publish no Feed. Reverse-proxy rendering, stored page snapshots and headless-browser rendering (ADR-0003). An image proxy. Curated per-site extraction rules. Prefetching extraction ahead of the reader. Tags, nested Groups, and any sharing, recommendation or public-page feature. AI features of any kind.

## Further Notes

Because Original View is an embed and roughly a quarter of popular publishers forbid embedding, **Reader View carries the product**. Extraction quality is the load-bearing quality attribute of this MVP; the follow-up levers are curated per-site rules and per-Feed full-content-on-ingest, both deliberately deferred.

The ordering of later phases is web, then desktop shell, then home-screen app. This is fortunate rather than accidental: the desktop shell is what upgrades Original View to a top-level webview, where embedding restrictions do not apply, so the weakest part of the MVP is the first thing the next phase fixes. Two prerequisites are parked rather than forgotten: TLS before any home-screen installation, and a public domain with mail records before newsletters.

Search covers only what publishers put in their Feeds, so truncated Feeds are findable by summary alone. This is a known, accepted limitation; indexing extracted Articles was rejected because extraction is on demand, which would make coverage silently uneven.
