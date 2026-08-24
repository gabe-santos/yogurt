# RSS Reader

A self-hosted reader for one person, which collects the feeds they follow into one place and presents each item either as extracted text or as the original web page.

## Language

### Sources

**Feed**:
The document at a URL that a publisher updates with new items, in RSS, Atom, or JSON Feed form, together with the reader's own name for it. There is exactly one Feed per URL.
_Avoid_: Channel, source, stream, subscription

**Group**:
A named set of Feeds, used to scope reading to one part of the collection. A Feed belongs to exactly one Group, and Groups do not nest.
_Avoid_: Folder, category, tag, collection

### Reading

**Entry**:
One item in a Feed, as the publisher supplied it: title, link, timestamp, and whatever body text the Feed carried. The unit that is read, starred, and listed.
_Avoid_: Item, post, story, article (when the Feed's own item is meant)

**Article**:
The document at an Entry's link — the publisher's own web page, distinct from the Entry that points at it.
_Avoid_: Page, content, full text

**Reader View**:
An Article reduced to its main text and images and re-rendered in this app's own markup.
_Avoid_: Reader mode, readability, extracted view

**Original View**:
An Article shown as the publisher laid it out, inside the app rather than in a separate browser.
_Avoid_: Web view, browser mode, in-app browser

### State

**Read**:
Set once the reader has seen an Entry's contents. Always settable and unsettable by hand, and by default set when an Entry is opened.
_Avoid_: Seen, viewed, opened

**Starred**:
Marks an Entry the reader wants to keep and return to. Starring is how something is kept, so there is no separate save-for-later.
_Avoid_: Bookmark, favourite, saved, read-later

**Archived**:
Marks an Entry the reader is finished with and does not want to encounter again. An Archived Entry is Read, and appears in no view except the archive.
_Avoid_: Dismissed, hidden, trashed, done
