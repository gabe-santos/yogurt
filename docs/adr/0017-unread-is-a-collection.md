# Unread is a Collection

Supersedes ADR-0013.

Unread Only was a toggle in the Entry List header that narrowed whichever Collection the reader chose. Unread is now a Collection in the Collection List, between All Feeds and Starred: every Entry not yet Read, from every Feed. The unread count that sat beside All Feeds moves to it, so the one number the reader triages against labels the place that holds exactly those Entries. Each Feed keeps its own count.

The cost is the combination ADR-0013 was written to allow: there is no longer a way to see one Feed's unread Entries alone, or the Starred ones not yet read. That was accepted because the per-Feed counts and the unread dot on every row already show what is new in a Feed without narrowing it. The server's `unread` parameter is unchanged; only the client stopped composing it with the other Collections.

The `unread_only` setting is gone rather than repurposed. The app still opens on All Feeds, as it did before for anyone who had never turned the toggle on; a stored `unread_only` row is left in `settings` and never read. `u` now opens Unread, and pressing it there rebuilds the list, which under ADR-0014 is how the Entries read since it was built are cleared out.
