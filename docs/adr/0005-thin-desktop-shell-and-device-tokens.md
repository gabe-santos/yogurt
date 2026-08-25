# The desktop app is a thin shell, so the API accepts device tokens

The future desktop app (per ADR-0007, Electron — this ADR's original Tauri choice is superseded) is a window onto a running server. It ships no database and no feed poller.

The rejected alternative was bundling the Go binary as a sidecar with its own SQLite file, which would allow reading with the server unreachable. That produces two divergent databases and demands exactly the sync-and-conflict machinery declined in ADR-0004, so it is out; supporting both modes is out for the same reason, doubled.

The consequence lands in the MVP, before any desktop code exists. A shell's origin is not the server's origin, so a `SameSite=Lax` session cookie is never sent and cookie auth cannot work for it. The API therefore accepts a second credential: a long-lived device token in an `Authorization` header, created and revocable from settings, stored hashed. Cookies remain the browser's path. The same mechanism serves the deferred Fever API, which needs a per-client credential of its own.
