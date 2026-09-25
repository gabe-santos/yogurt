# The Desktop App carries its own Instance

Supersedes ADR-0005's thin-shell decision. Its device-token reasoning still holds for a Desktop App that connects to a Server, which is deferred (#58).

Yogurt ships two ways from one codebase: a Server, self-hosted and used through the Web App, and a Desktop App that bundles the Go binary with its own SQLite database and runs both on the Reader's computer. ADR-0005 rejected that bundled backend because one Reader with two databases needs sync. That objection only applies when one Reader uses both at once. So a Desktop App shows only its own Instance and never syncs with a Server, and a Reader moves between the two by copying the database file. The Desktop App's own Instance has no password. Instead its backend listens on loopback only and accepts only a secret the Desktop App generates, because otherwise any web page the Reader visits could reach it.

## Consequences

- Feeds are pulled only while the Desktop App runs. After a long absence, Entries that fell off a Feed before the next pull are lost. That was accepted over a background process.
- Phones reach Yogurt only through a Server.
- Because the database file now moves between Instances, an older Yogurt must refuse to open a database that a newer one has migrated.
