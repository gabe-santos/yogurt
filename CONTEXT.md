# Yogurt

A self-hosted app for one Reader, which collects the feeds they follow into one place and presents each item either as extracted text or as the original web page.

## Language

### People

**Reader**:
The one person this app serves, who follows the Feeds and owns every Read, Starred, and Archived state.
_Avoid_: User, owner, account

### Sources

**Feed**:
The document at a URL that a publisher updates with new items, in RSS, Atom, or JSON Feed form, together with the reader's own name for it. There is exactly one Feed per URL.
_Avoid_: Channel, source, stream, subscription

**Feed Icon**:
The small square image that identifies a Feed, shown beside the publisher's name on every Entry from that Feed. One per Feed, independent of any Entry's contents.
_Avoid_: Favicon, avatar, logo, feed image

### Reading

**Entry**:
One item in a Feed, as the publisher supplied it: title, link, timestamp, and whatever body text the Feed carried. The unit that is read, starred, and listed.
_Avoid_: Item, post, story, article (when the Feed's own item is meant)

**Article**:
The document at an Entry's link — the publisher's own web page, distinct from the Entry that points at it.
_Avoid_: Page, content, full text

**Preview Image**:
A single representative image for one Entry, derived from its Article rather than supplied by the Feed. Absent whenever the Article offers none.
_Avoid_: Thumbnail, hero, cover, og image, social image

**Excerpt**:
The opening of an Entry's body as plain text, shown beneath its title in the Entry List so a list can be scanned without opening anything.
_Avoid_: Preview, summary, teaser, snippet, description

**Snippet**:
The fragment of an Entry that a search matched, positioned at the match rather than at the Entry's start.
_Avoid_: Excerpt, highlight, result text

**Feed View**:
An Entry's own body text, exactly as the Feed supplied it, without reaching for the Article at all.
_Avoid_: Feed content, raw view, summary, from the feed

**Reader View**:
An Article reduced to its main text and images and re-rendered in this app's own markup.
_Avoid_: Reader mode, readability, extracted view

**Original View**:
An Article shown as the publisher laid it out, inside the app rather than in a separate browser.
_Avoid_: Web view, browser mode, in-app browser

### Layout

**Collection**:
One thing the Collection List selects, and so what the Entry List is showing: every Feed, one Feed, everything Starred, or the archive. A Collection is a place Entries are found, never a state they are in.
_Avoid_: Filter, view, tab, smart folder

**Collection List**:
The leftmost column, holding every Collection: All Feeds, Starred, the archive, and each Feed by name.
_Avoid_: Sidebar, nav, feed tree, feed list

**Unread Only**:
A setting that narrows the Entry List to the Entries in the chosen Collection that are not Read. It is a way of looking at a Collection rather than a Collection of its own, so it applies to whichever one the reader chose, and belongs to the reader rather than to any single Collection.
_Avoid_: Unread filter, unread view, inbox, unread tab

**Entry List**:
The middle column, holding the Entries of the chosen Collection as they stood when it was last built.
_Avoid_: Sidebar, inbox, article list, list pane

**Reading Pane**:
The rightmost and largest column, holding one Entry in whichever view the reader chose.
_Avoid_: Content pane, detail pane, preview, drawer

**Reading Font**:
A setting naming the typeface the Reading Pane sets an Entry's own headline and text in — the sans the rest of the interface uses, or the serif. It reaches the app-owned headline plus Reader View and Feed View prose; it never restyles Original View's embedded Article. Like Unread Only it belongs to the reader rather than to any Entry.
_Avoid_: Font setting, typeface toggle, serif mode, theme

### State

**Read**:
Set once the reader has seen an Entry's contents. Always settable and unsettable by hand, and by default set when an Entry becomes the Entry in the Reading Pane.
_Avoid_: Seen, viewed, opened

**Starred**:
Marks an Entry the reader wants to keep and return to. Starring is how something is kept, so there is no separate save-for-later.
_Avoid_: Bookmark, favourite, saved, read-later

**Archived**:
Marks an Entry the reader is finished with and does not want to encounter again. An Archived Entry is Read, and is found in no Collection but the archive.
_Avoid_: Dismissed, hidden, trashed, done
