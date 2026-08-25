# Electron, not Tauri, for the desktop shell

Supersedes the framework choice in ADR-0005; its device-token and thin-shell reasoning stands unchanged.

ADR-0005 picked Tauri on footprint: ~2.5 MB versus Electron's ~85 MB installer, 58-75% less memory, system webview. That comparison never weighed the one capability the desktop shell exists to deliver — Original View per ADR-0003 needs a live, user-navigated, runtime-arbitrary third-party origin embedded inside the app's own layout, immune to the publisher's `X-Frame-Options`/`frame-ancestors`. Electron's `<webview>` tag is a decade-old, proven mechanism for exactly this — it's what raindrop.io ships in production for its own desktop client. Tauri's equivalent (multiwebview, Tauri 2.x) is newer, thinner in track record for this specific pattern, and its behaviour is only as consistent as whatever WebView2/WKWebView/WebKitGTK each OS happens to ship, not a single engine under our control.

Electron's cost is real and now ours to own: a bundled Chromium means a larger installer, higher idle memory, and a self-owned patch cadence for Chromium CVEs instead of inheriting the OS's webview updates. Accepted for now because the alternative is building the one feature this shell exists for on a less-proven mechanism.

Revisit if Tauri's multiwebview matures to parity for arbitrary live-origin embedding, or if Electron's maintenance cost (patch cadence, installer size) proves heavier in practice than this tradeoff assumed.
