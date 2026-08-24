# Original View is a live iframe, and nothing more

Original View embeds the publisher's live URL in an iframe. This is understood to fail on every site that sends `X-Frame-Options` or CSP `frame-ancestors` — roughly a quarter of popular domains, skewed towards news — and those Articles fall back to Reader View plus an "open in new tab" action.

Three richer mechanisms were considered and rejected. **Reverse-proxy rendering** would work everywhere and republishes whole articles from our origin: no shipped reader does this, and the copyright exposure is real. **Server-side single-file snapshots** (`obelisk`, `monolith`) are always frameable and survive link rot, but cost 1.5-3 MB per Article and a storage-eviction subsystem. **Headless Chromium** buys fidelity on JS-built pages for a ~500 MB sidecar and 300-500 MB per render. The intended path to fidelity is instead the future native shell, where a webview loads the page as a top-level document and framing headers never apply.

Consequence: if a future reader is tempted to "fix" Original View with a proxy or a headless browser, that is a reversal of this decision, not an oversight in it.
