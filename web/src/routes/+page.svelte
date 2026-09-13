<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import AddFeedDialog from "$lib/AddFeedDialog.svelte";
  import * as Sidebar from "$lib/components/ui/sidebar";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import * as Select from "$lib/components/ui/select";
  import { Toggle } from "$lib/components/ui/toggle";
  import * as Tooltip from "$lib/components/ui/tooltip";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import IconSwap from "$lib/IconSwap.svelte";
  import { cn } from "$lib/utils.js";
  import ArrowDownWideNarrowIcon from "@lucide/svelte/icons/arrow-down-wide-narrow";
  import ArrowUpNarrowWideIcon from "@lucide/svelte/icons/arrow-up-narrow-wide";
  import ArchiveIcon from "@lucide/svelte/icons/archive";
  import ChevronsUpDownIcon from "@lucide/svelte/icons/chevrons-up-down";
  import CircleHelpIcon from "@lucide/svelte/icons/circle-help";
  import InboxIcon from "@lucide/svelte/icons/inbox";
  import KeyRoundIcon from "@lucide/svelte/icons/key-round";
  import LoaderCircleIcon from "@lucide/svelte/icons/loader-circle";
  import LogOutIcon from "@lucide/svelte/icons/log-out";
  import PlusIcon from "@lucide/svelte/icons/plus";
  import RssIcon from "@lucide/svelte/icons/rss";
  import SearchIcon from "@lucide/svelte/icons/search";
  import StarIcon from "@lucide/svelte/icons/star";
  import XIcon from "@lucide/svelte/icons/x";
  import { onMount } from "svelte";
  import { browser } from "$app/environment";
  import { goto, invalidateAll, replaceState } from "$app/navigation";
  import {
    deleteFeed,
    describeError,
    feedIconUrl,
    getSettings,
    listEntries,
    listFeeds,
    logOut,
    markEntriesRead,
    refreshFeeds,
    setEntryState,
    setSettings,
    updateFeed,
  } from "$lib/api";
  import type {
    Entry,
    EntryOrder,
    EntrySelectionOptions,
    EntryView,
    Feed,
    ReadingFont,
    SearchEntry,
    Settings,
  } from "$lib/api";
  import ReadingPane from "$lib/ReadingPane.svelte";
  import BookOpenIcon from "@lucide/svelte/icons/book-open";
  import CheckCheckIcon from "@lucide/svelte/icons/check-check";
  import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
  import ConfirmDialog from "$lib/ConfirmDialog.svelte";
  import EntryRow from "$lib/EntryRow.svelte";
  import FeedRow from "$lib/FeedRow.svelte";
  import DeviceTokensDialog from "$lib/DeviceTokensDialog.svelte";
  import HelpDialog from "$lib/HelpDialog.svelte";
  import SearchDialog from "$lib/SearchDialog.svelte";
  import { bindings, matches } from "$lib/keys";
  import type { Action } from "$lib/keys";

  /** Collection is what the Collection List selects: every Feed, one Feed,
   * everything Starred, or the archive. Unread Only narrows whichever
   * Collection the reader chose rather than being one of its own — see
   * docs/adr/0013-unread-only-is-a-modifier.md. */
  type Collection =
    | { type: "all" }
    | { type: "feed"; id: number }
    | { type: "starred" }
    | { type: "archive" };

  // The reading list is the server's, and this page mutates it as the reader
  // works, so it owns the copy rather than deriving one from a load function.
  let feeds = $state<Feed[]>([]);
  let entries = $state<Entry[]>([]);
  let cursor = $state("");
  let collection = $state<Collection>({ type: "all" });
  // Unread Only is the reader's preference and is restored from the server on
  // mount. The Collection is not: All Feeds is the right place to start.
  let unreadOnly = $state(false);
  let entryOrder = $state<EntryOrder>("newest");
  let loading = $state(true);

  // feedsByID resolves an Entry's Feed Icon from data this page already
  // holds, so the list costs no extra request to show one.
  const feedsByID = $derived.by(() => {
    const map = new Map<number, Feed>();
    for (const feed of feeds) map.set(feed.id, feed);
    return map;
  });

  function iconForEntry(entry: Entry): string | undefined {
    const feed = feedsByID.get(entry.feed_id);
    return feed ? feedIconUrl(feed) : undefined;
  }

  function compareEntries(a: Entry, b: Entry): number {
    const ascending =
      a.published_at.localeCompare(b.published_at) || a.id - b.id;
    return entryOrder === "oldest" ? ascending : -ascending;
  }

  // Selecting an Entry and opening it are one act: the Reading Pane always
  // shows the selected Entry. See docs/adr/0010-selection-is-opening.md.
  let selectedIndex = $state<number | undefined>(undefined);
  // Closing the Entry is not moving in the list. Before there is room for a
  // third column, backing out of the overlay clears the selection, and with
  // it every trace of where the reader had got to: j would start the list
  // again from the top. closedIndex keeps the row they were reading, and only
  // rebuilding the list forgets it.
  let closedIndex = $state<number | undefined>(undefined);
  /** position is the row the reader is at whether or not its Entry is open:
   * what j/k count from, and the Entry List's single tab stop. */
  const position = $derived(selectedIndex ?? closedIndex);
  // The keyboard carries the reader's position with it: the row j and k land
  // on takes focus, which is what puts the move into the accessibility tree
  // and leaves Tab continuing from where the reader actually is. A count
  // rather than a flag, because closing the overlay asks the same row that
  // was already the position to take focus back off the pane that covered
  // it: nothing about the row itself changes, so the request has to. Zero
  // asks for no focus at all — a click landed its own, and a list the reader
  // did not move through must not pull focus out of whatever rebuilt it.
  let focusRequest = $state(0);
  // The Reading Pane is a column of the layout once there is room for three,
  // and an overlay over the Entry List before that. "Room" is what this pane
  // pair actually has, not what the window has: the Collection List takes
  // 16rem out of the window when it is open and gives it all back when it is
  // collapsed, so a window query answers the question wrong by that much in
  // both directions. The element is measured instead, and the CSS beside it
  // asks the same question with `@container`/`@3xl` against the same box.
  // The list behind an overlay must not be reachable by tab, which is what
  // narrow decides.
  const twoColumnMin = 768;
  let inset = $state<HTMLElement | null>(null);
  // insetWidth is 0 only before the element exists. Nothing reads `narrow`
  // in that window: every consumer — the list's `inert`, Escape, and the
  // pane's own overlay behaviour — needs a selected Entry, and a selection
  // cannot exist before the Entries have loaded, which is several frames
  // after the effect below has measured. A window-width seed was the
  // alternative and it is wrong by the Collection List's 16rem in exactly
  // the band this measurement exists for.
  let insetWidth = $state(0);
  const narrow = $derived(insetWidth < twoColumnMin);
  // selectionRestored stops the empty selection the page starts with from
  // clearing the Entry named in the address bar before it has been read.
  let selectionRestored = $state(false);
  let helpOpen = $state(false);
  let deviceTokensOpen = $state(false);
  let searchOpen = $state(false);
  // The footer menu is a bits-ui DropdownMenu: it closes itself on Escape
  // without telling the window `keydown` listener below, so that listener
  // has to be told independently or Escape also runs the app's own "close
  // whatever is open" shortcut on top of the menu's own close — see
  // closeCurrent and onKeydown.
  let readerMenuOpen = $state(false);
  let markOnOpen = $state(true);
  // The only thing this app knows about who is signed in is which server
  // they are signed in to: there are no accounts, just one password
  // (docs/adr/0002-single-user.md). Read at init, so it is empty while
  // prerendering and filled on the client.
  const serverHost = browser ? location.host : "";

  // The view an Entry opens in belongs to the reader, not to an Entry: it is
  // stored on the server, so it survives both moving to the next Entry and
  // coming back tomorrow in another browser.
  let entryView = $state<EntryView>("feed");
  // The face an Entry is read in is the reader's too, and for the same reason:
  // it is a decision about reading, not about this Entry or this session.
  let readingFont = $state<ReadingFont>("sans");
  // Entries the reader has declared unread by hand this session: mark-on-open
  // must never re-mark them Read just because j/k passed back through them.
  let manuallyUnread = $state<Set<number>>(new Set());
  let pendingEntryIDs = $state<Set<number>>(new Set());

  let addFeedOpen = $state(false);
  let notice = $state("");
  let busy = $state(false);
  // busy blocks every list-changing action; refreshing is narrower, so only
  // the control the reader actually pressed reports that it is working.
  let refreshing = $state(false);

  let editingFeed = $state<number | undefined>(undefined);
  let feedTitleDraft = $state("");
  // Renaming is reached from a Feed's own context menu, so the field it opens
  // takes focus and selects the current name: the menu closed over it, and
  // nothing else would put a cursor there.
  let feedTitleInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    feedTitleInput?.select();
  });
  /** Removal is the one act this app cannot undo, so it is asked in the app's
   * own dialog rather than the browser's, and the consequence is named. */
  type Removal = {
    title: string;
    description: string;
    confirmLabel: string;
    run: () => Promise<void>;
  };
  let removal = $state<Removal | undefined>(undefined);

  const collectionFeed = $derived.by(() => {
    const current = collection;
    return current.type === "feed"
      ? feeds.find((f) => f.id === current.id)
      : undefined;
  });
  const collectionTitle = $derived.by(() => {
    switch (collection.type) {
      case "feed":
        return collectionFeed?.title ?? "All Feeds";
      case "starred":
        return "Starred";
      case "archive":
        return "Archive";
      default:
        return "All Feeds";
    }
  });
  // An Archived Entry is always Read, so Unread Only could only ever narrow
  // the archive to nothing: it is not offered there, and the server ignores
  // the parameter in that Collection anyway.
  const unreadOnlyOffered = $derived(collection.type !== "archive");
  const canMarkAllRead = $derived(collection.type !== "archive");
  // The unread total is the sum of the per-Feed counts every Feed already
  // carries, so the All Feeds badge costs no request of its own.
  const unreadTotal = $derived(
    feeds.reduce((sum, feed) => sum + feed.unread_count, 0),
  );
  /** emptyState says why the Entry List is empty and what fills it, composed
   * from the Collection and the toggle rather than written out per view. */
  const emptyState = $derived.by(() => {
    if (unreadOnly && unreadOnlyOffered) {
      return {
        title: "Nothing unread here.",
        detail: `Turn Unread off to see everything in ${collectionTitle}.`,
      };
    }
    switch (collection.type) {
      case "starred":
        return {
          title: "Nothing Starred here.",
          detail:
            "Star an Entry to keep it. Starred Entries are never cleaned up.",
        };
      case "archive":
        return {
          title: "The Archive is empty.",
          detail:
            "Archiving an Entry marks it Read and takes it out of every other Collection.",
        };
      default:
        return {
          title: "Nothing to read here yet.",
          detail: "New Entries appear here as your Feeds are checked.",
        };
    }
  });
  const selectedEntry = $derived(
    selectedIndex !== undefined ? entries[selectedIndex] : undefined,
  );
  const overlayUp = $derived(narrow && selectedEntry !== undefined);
  // The Feeds group is a flat, title-sorted list of every Feed: this stays in
  // step with a rename without a separate resort step.
  const sortedFeeds = $derived(
    [...feeds].sort((a, b) =>
      a.title.localeCompare(b.title, undefined, { sensitivity: "base" }),
    ),
  );

  // ResizeObserver rather than a window listener: the pane pair also changes
  // width when the Collection List opens or collapses, which resizes nothing.
  // The first measurement is taken synchronously here rather than waiting for
  // the observer's first delivery, which lands after this frame's layout: a
  // deep-linked Entry must not be able to mount its pane against a width
  // nobody has measured yet.
  $effect(() => {
    const measured = inset;
    if (!measured) return;
    insetWidth = measured.clientWidth;
    const observer = new ResizeObserver(([entry]) => {
      insetWidth = entry.contentRect.width;
    });
    observer.observe(measured);
    return () => observer.disconnect();
  });

  onMount(async () => {
    // An Entry named in the address bar is restored in place: the list loads
    // around it rather than from the top, so a reload leaves the reader where
    // they were with the rest of the list still under them.
    const deepLink = Number(
      new URL(window.location.href).searchParams.get("entry") ?? "",
    );
    try {
      // Unread Only decides what the first page even asks for, so the
      // preferences are read before the list rather than beside it.
      const [subscribed, settings] = await Promise.all([
        listFeeds(),
        getSettings(),
      ]);
      feeds = subscribed;
      markOnOpen = settings.mark_on_open;
      entryView = settings.entry_view;
      unreadOnly = settings.unread_only;
      readingFont = settings.reading_font;
      const page = await listEntries({
        ...selectionQuery(),
        ...(deepLink > 0 ? { around: deepLink } : {}),
        order: entryOrder,
      });
      entries = page.entries;
      cursor = page.next_cursor;
      if (deepLink > 0) {
        let index = entries.findIndex((entry) => entry.id === deepLink);
        if (index < 0) index = await restoreAnchor(deepLink);
        if (index >= 0) {
          selectedIndex = index;
          maybeMarkOnSelect(index);
        }
      }
    } finally {
      loading = false;
      selectionRestored = true;
    }
  });

  /** restoreAnchor puts an Entry the reader arrived at back into a page that
   * does not hold it. The server drops an `around` anchor falling outside the
   * selection, so an Entry that has since been Read would take the reader's
   * place away with it. Reports where it landed, or -1. */
  async function restoreAnchor(id: number): Promise<number> {
    try {
      const page = await listEntries({
        ...selectionQuery(collection, false),
        around: id,
        limit: 1,
        order: entryOrder,
      });
      const anchor = page.entries.find((entry) => entry.id === id);
      if (!anchor) return -1;
      entries = [...entries, anchor].sort(compareEntries);
      return entries.findIndex((entry) => entry.id === id);
    } catch {
      return -1;
    }
  }

  // The selected Entry is the one piece of reading position worth surviving a
  // reload, and it replaces rather than pushes: j down a list of forty would
  // otherwise leave forty steps for the back button to walk back out through.
  $effect(() => {
    if (!selectionRestored) return;
    const wanted = selectedEntry === undefined ? null : String(selectedEntry.id);
    const url = new URL(window.location.href);
    if (url.searchParams.get("entry") === wanted) return;
    if (wanted === null) url.searchParams.delete("entry");
    else url.searchParams.set("entry", wanted);
    replaceState(url, {});
  });

  // A notice reports what just happened. It is not state the reader has to
  // clear, and it must not sit above the reading list until some later action
  // happens to overwrite it.
  const noticeLifetime = 8000;
  $effect(() => {
    if (!notice) return;
    const timer = setTimeout(() => (notice = ""), noticeLifetime);
    return () => clearTimeout(timer);
  });

  /** selectionQuery is the single client mapping for both list reads and
   * mark-all-read, so bulk state cannot drift beyond the visible selection.
   * Unread Only is a modifier over the Collection rather than one of its own,
   * which is how the server reads these four parameters too. */
  function selectionQuery(
    current: Collection = collection,
    narrowed: boolean = unreadOnly,
  ): EntrySelectionOptions {
    return {
      feed: current.type === "feed" ? current.id : undefined,
      starred: current.type === "starred" ? true : undefined,
      archived: current.type === "archive" ? true : undefined,
      // The archive ignores this: an Archived Entry is always Read.
      unread: narrowed && current.type !== "archive" ? true : undefined,
    };
  }

  /** reportError shows an ApiError's own message, or a generic one for
   * anything else (a network failure, a body the server never sent). */
  function reportError(cause: unknown) {
    notice = describeError(cause, "Could not reach the server");
  }

  /** reload replaces the list with the first page of the current Collection,
   * clearing keyboard position: the underlying list changed under it. */
  async function reload() {
    const page = await listEntries({ ...selectionQuery(), order: entryOrder });
    entries = page.entries;
    cursor = page.next_cursor;
    selectedIndex = undefined;
    closedIndex = undefined;
    // Whatever rebuilt the list — a Collection, the order, Unread Only — is
    // what the reader is holding, so the new list must not take focus off it.
    focusRequest = 0;
    manuallyUnread = new Set();
  }

  /** refreshCounts re-reads Feeds so their unread counts stay correct after
   * an Entry's Read state, or the set of Feeds itself, changes. */
  async function refreshCounts() {
    feeds = await listFeeds();
  }

  async function handleFeedAdded(feed: Feed) {
    addFeedOpen = false;
    try {
      await refreshCounts();
      collection = { type: "feed", id: feed.id };
      await reload();
      notice = `Added Feed ${feed.title}.`;
    } catch (cause) {
      reportError(cause);
    }
  }

  async function refresh() {
    if (busy) return;
    notice = "";
    busy = true;
    refreshing = true;
    try {
      const failures = await refreshFeeds();
      await reload();
      await refreshCounts();
      notice =
        failures.length === 0
          ? "Every Feed is up to date."
          : `${failures.length} Feed${failures.length === 1 ? "" : "s"} could not be read.`;
    } catch (cause) {
      reportError(cause);
    } finally {
      refreshing = false;
      busy = false;
    }
  }

  async function selectCollection(next: Collection) {
    if (busy) return;
    collection = next;
    busy = true;
    try {
      await reload();
    } finally {
      busy = false;
    }
  }

  /** openSearchEntry opens a search result within its ordinary list: in the
   * Collection that Entry lives in, anchored at the Entry itself rather than
   * at the top of the list — not a standalone search-result view. */
  async function openSearchEntry(entry: SearchEntry) {
    searchOpen = false;
    const next: Collection = entry.archived
      ? { type: "archive" }
      : { type: "feed", id: entry.feed_id };
    collection = next;
    busy = true;
    try {
      const page = await listEntries({
        ...selectionQuery(next),
        around: entry.id,
        order: entryOrder,
      });
      // A Read Entry falls outside a narrowed selection, and the server drops
      // the anchor with it. The Entry the reader asked for belongs to the list
      // that is about to show it, so it is put back where it sorts.
      entries = page.entries.some((candidate) => candidate.id === entry.id)
        ? page.entries
        : [...page.entries, entry].sort(compareEntries);
      cursor = page.next_cursor;
      manuallyUnread = new Set();
      // This is a different list from the one the reader closed an Entry in.
      closedIndex = undefined;
      focusRequest = 0;
      const index = entries.findIndex((candidate) => candidate.id === entry.id);
      selectedIndex = index < 0 ? undefined : index;
      if (index >= 0) maybeMarkOnSelect(index);
    } catch (cause) {
      reportError(cause);
    } finally {
      busy = false;
    }
  }

  /** openSearchFeed navigates to a Feed a search matched by name. */
  function openSearchFeed(feed: Feed) {
    searchOpen = false;
    void selectCollection({ type: "feed", id: feed.id });
  }

  /** setUnreadOnly narrows the Entry List to unread Entries, or widens it back
   * again. The list is what the reader was after, so it is rebuilt at once and
   * the preference is remembered behind it. */
  async function setUnreadOnly(next: boolean) {
    if (busy || unreadOnly === next || !unreadOnlyOffered) return;
    const previous = preferences();
    unreadOnly = next;
    busy = true;
    try {
      await reload();
    } finally {
      busy = false;
    }
    void savePreferences({ ...previous, unread_only: next }, previous);
  }

  function toggleUnreadOnly() {
    void setUnreadOnly(!unreadOnly);
  }

  async function setEntryOrder(next: EntryOrder) {
    if (busy || entryOrder === next) return;
    entryOrder = next;
    busy = true;
    try {
      await reload();
    } finally {
      busy = false;
    }
  }

  async function loadMore() {
    if (!cursor) return;
    busy = true;
    try {
      const page = await listEntries({
        ...selectionQuery(),
        cursor,
        order: entryOrder,
      });
      entries = [...entries, ...page.entries];
      cursor = page.next_cursor;
    } catch (cause) {
      reportError(cause);
    } finally {
      busy = false;
    }
  }

  async function signOut() {
    await logOut();
    await invalidateAll();
    await goto("/login");
  }

  type EntryState = Pick<Entry, "read" | "starred" | "archived">;

  /** applyEntryState declares complete state optimistically. Rejection restores
   * only this Entry, so another Entry's concurrent success cannot be erased.
   *
   * State never takes an Entry out of the Entry List. The list is whatever the
   * last query returned, and only the reader rebuilds it — see
   * docs/adr/0014-the-entry-list-holds-still.md. */
  async function applyEntryState(
    index: number,
    state: EntryState,
    manualRead = false,
  ) {
    const previous = entries[index];
    if (busy || pendingEntryIDs.has(previous.id)) return;

    const previousManualUnread = manuallyUnread.has(previous.id);
    pendingEntryIDs = new Set([...pendingEntryIDs, previous.id]);
    entries[index] = {
      ...previous,
      ...state,
      read: state.archived ? true : state.read,
    };
    if (manualRead && !state.read) {
      manuallyUnread = new Set([...manuallyUnread, previous.id]);
    } else if (manualRead) {
      manuallyUnread = new Set(
        [...manuallyUnread].filter((id) => id !== previous.id),
      );
    }

    let stored: Entry;
    try {
      stored = await setEntryState(previous.id, state);
    } catch (cause) {
      pendingEntryIDs = new Set(
        [...pendingEntryIDs].filter((id) => id !== previous.id),
      );
      // The reader may have rebuilt the list while this was in flight, in
      // which case the Entry is no longer in it and there is nothing to undo.
      const existing = entries.findIndex((entry) => entry.id === previous.id);
      if (existing >= 0) entries[existing] = previous;
      if (previousManualUnread) {
        manuallyUnread = new Set([...manuallyUnread, previous.id]);
      } else {
        manuallyUnread = new Set(
          [...manuallyUnread].filter((id) => id !== previous.id),
        );
      }
      reportError(cause);
      return;
    }

    pendingEntryIDs = new Set(
      [...pendingEntryIDs].filter((id) => id !== stored.id),
    );
    const storedIndex = entries.findIndex((entry) => entry.id === stored.id);
    if (storedIndex >= 0) entries[storedIndex] = stored;
    try {
      await refreshCounts();
    } catch (cause) {
      reportError(cause);
    }
  }

  function applyRead(index: number, read: boolean, manual = false) {
    const entry = entries[index];
    return applyEntryState(
      index,
      { read, starred: entry.starred, archived: entry.archived },
      manual,
    );
  }

  async function moveSelection(delta: number) {
    if (busy || entries.length === 0) return;
    // Counted from where the reader is, which is the open Entry's row or, if
    // they closed it, the row it was on.
    const base = position ?? (delta > 0 ? -1 : entries.length);
    let target = base + delta;
    // The end of the loaded list is not the end of the reading list: j reaches
    // for the next page rather than stopping dead on the last row.
    if (target >= entries.length && cursor) {
      await loadMore();
    }
    if (entries.length === 0) return;
    closedIndex = undefined;
    focusRequest += 1;
    selectedIndex = Math.min(Math.max(target, 0), entries.length - 1);
    maybeMarkOnSelect(selectedIndex);
  }

  function selectEntryAt(index: number) {
    if (busy) return;
    closedIndex = undefined;
    focusRequest = 0;
    selectedIndex = index;
    maybeMarkOnSelect(index);
  }

  /** clearSelection backs out of the overlay the Reading Pane is before there
   * is room for a third column. With the room, there is nothing to back out
   * of and the pane keeps what it is showing.
   *
   * The overlay took focus when it opened and the list behind it was inert,
   * so closing it has to hand focus back; the row the Entry was on is where
   * the reader is, and where j continues from. */
  function clearSelection() {
    // Escape with nothing open reaches here too, and there is no position to
    // take: leaving the closed row alone is what keeps j continuing from it.
    if (busy || !narrow || selectedIndex === undefined) return;
    closedIndex = selectedIndex;
    focusRequest += 1;
    selectedIndex = undefined;
  }

  function maybeMarkOnSelect(index: number) {
    const entry = entries[index];
    if (entry && markOnOpen && !entry.read && !manuallyUnread.has(entry.id)) {
      void applyRead(index, true);
    }
  }

  function toggleReadAt(index: number) {
    const entry = entries[index];
    if (!entry || entry.archived) return;
    void applyRead(index, !entry.read, true);
  }

  function toggleReadCurrent() {
    if (selectedIndex !== undefined) toggleReadAt(selectedIndex);
  }

  function toggleStarAt(index: number) {
    const entry = entries[index];
    if (!entry) return;
    void applyEntryState(index, {
      read: entry.read,
      starred: !entry.starred,
      archived: entry.archived,
    });
  }

  function toggleStarCurrent() {
    if (selectedIndex !== undefined) toggleStarAt(selectedIndex);
  }

  function toggleArchiveAt(index: number) {
    const entry = entries[index];
    if (!entry) return;
    void applyEntryState(index, {
      read: true,
      starred: entry.starred,
      archived: !entry.archived,
    });
  }

  function toggleArchiveCurrent() {
    if (selectedIndex !== undefined) toggleArchiveAt(selectedIndex);
  }

  async function markAllRead() {
    if (busy || !canMarkAllRead) return;
    busy = true;
    const previousEntries = [...entries];
    const previousManualUnread = new Set(manuallyUnread);

    // Every loaded Entry is Read where it stands. Nothing is removed, so the
    // cursor and the reader's position still mean what they meant.
    entries = entries.map((entry) => ({ ...entry, read: true }));
    manuallyUnread = new Set();
    try {
      await markEntriesRead(selectionQuery());
    } catch (cause) {
      entries = previousEntries;
      manuallyUnread = previousManualUnread;
      reportError(cause);
      busy = false;
      return;
    }
    try {
      await refreshCounts();
      notice = "Marked everything here Read.";
    } catch (cause) {
      reportError(cause);
    } finally {
      busy = false;
    }
  }

  // Preferences are declared whole, so every change sends every field rather
  // than letting an omitted one fall back to a default the reader never chose.
  async function savePreferences(next: Settings, previous: Settings) {
    markOnOpen = next.mark_on_open;
    entryView = next.entry_view;
    unreadOnly = next.unread_only;
    readingFont = next.reading_font;
    try {
      const stored = await setSettings(next);
      markOnOpen = stored.mark_on_open;
      entryView = stored.entry_view;
      unreadOnly = stored.unread_only;
      readingFont = stored.reading_font;
    } catch {
      markOnOpen = previous.mark_on_open;
      entryView = previous.entry_view;
      unreadOnly = previous.unread_only;
      readingFont = previous.reading_font;
      notice = "Could not update your settings.";
      // A refused Unread Only would leave the list narrowed the way the toggle
      // no longer claims it is, so the list goes back with the toggle.
      if (next.unread_only !== previous.unread_only) {
        try {
          await reload();
        } catch (cause) {
          reportError(cause);
        }
      }
    }
  }

  function preferences(): Settings {
    return {
      mark_on_open: markOnOpen,
      entry_view: entryView,
      unread_only: unreadOnly,
      reading_font: readingFont,
    };
  }

  function toggleMarkOnOpen() {
    const previous = preferences();
    void savePreferences({ ...previous, mark_on_open: !markOnOpen }, previous);
  }

  function chooseEntryView(view: EntryView) {
    const previous = preferences();
    void savePreferences({ ...previous, entry_view: view }, previous);
  }

  function chooseReadingFont(font: ReadingFont) {
    const previous = preferences();
    void savePreferences({ ...previous, reading_font: font }, previous);
  }

  function startEditFeed(feed: Feed) {
    editingFeed = feed.id;
    feedTitleDraft = feed.title;
  }

  async function saveFeedTitle(id: number) {
    const title = feedTitleDraft.trim();
    editingFeed = undefined;
    const current = feeds.find((f) => f.id === id);
    if (!title || current?.title === title) {
      return;
    }
    try {
      const updated = await updateFeed(id, { title });
      feeds = feeds.map((f) => (f.id === id ? updated : f));
    } catch (cause) {
      reportError(cause);
    }
  }

  function removeFeed(feed: Feed) {
    removal = {
      title: `Delete "${feed.title}"?`,
      description:
        "Every Entry it carried is deleted with it, Starred ones included. Adding the Feed again starts it from scratch.",
      confirmLabel: "Delete Feed",
      run: async () => {
        await deleteFeed(feed.id);
        if (collection.type === "feed" && collection.id === feed.id) {
          collection = { type: "all" };
        }
        await refreshCounts();
        await reload();
      },
    };
  }

  /** confirmRemoval runs the staged removal and closes the dialog whether it
   * succeeded or not: a failure belongs in the status line, not behind a
   * dialog the reader now has to dismiss twice. */
  async function confirmRemoval() {
    const staged = removal;
    removal = undefined;
    if (!staged) return;
    try {
      await staged.run();
    } catch (cause) {
      reportError(cause);
    }
  }

  function closeCurrent() {
    if (readerMenuOpen) {
      readerMenuOpen = false;
    } else if (searchOpen) {
      searchOpen = false;
    } else if (helpOpen) {
      helpOpen = false;
    } else if (deviceTokensOpen) {
      deviceTokensOpen = false;
    } else if (addFeedOpen) {
      addFeedOpen = false;
    } else if (removal) {
      removal = undefined;
    } else {
      clearSelection();
    }
  }

  // One dispatch table, keyed by the same action ids the binding table names,
  // so a new shortcut is a row in keys.ts plus one entry here rather than a
  // second switch that can drift from the first.
  const actions: Record<Action, () => void> = {
    next: () => void moveSelection(1),
    prev: () => void moveSelection(-1),
    close: closeCurrent,
    toggleRead: toggleReadCurrent,
    toggleArchive: toggleArchiveCurrent,
    toggleUnreadOnly,
    help: () => (helpOpen = true),
    search: () => (searchOpen = true),
    addFeed: () => (addFeedOpen = true),
    refreshAll: () => void refresh(),
  };

  function isTypingTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) {
      return false;
    }
    return (
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      target.isContentEditable
    );
  }

  function onKeydown(event: KeyboardEvent) {
    if (isTypingTarget(event.target)) {
      return;
    }
    for (const binding of bindings) {
      if (!matches(binding, event)) {
        continue;
      }
      if (
        (helpOpen ||
          searchOpen ||
          deviceTokensOpen ||
          addFeedOpen ||
          removal ||
          readerMenuOpen) &&
        binding.action !== "close"
      ) {
        return;
      }
      actions[binding.action]();
      event.preventDefault();
      return;
    }
  }

  // A notice reports and leaves without moving the row the reader was
  // aiming at: the row stays in the layout at zero height and grows or
  // collapses around it instead of the whole list jumping. noticeVisible
  // drives the collapse; displayedNotice keeps the outgoing message on
  // screen through the collapse rather than blanking the instant `notice`
  // is cleared.
  const noticeVisible = $derived(notice !== "");
  let displayedNotice = $state("");
  $effect(() => {
    if (notice) displayedNotice = notice;
  });
</script>

<svelte:window onkeydown={onKeydown} />

<Sidebar.Provider>
  <Sidebar.Root>
    <!-- Search stays the icon it has always been, in its own bar: the menu at
         the foot took the app-level controls and nothing else. -->
    <Sidebar.Header>
      <div class="flex items-center justify-end gap-2 ps-2">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Search"
          data-testid="open-search"
          onclick={() => (searchOpen = true)}
        >
          <SearchIcon />
        </Button>
      </div>
    </Sidebar.Header>

    <Sidebar.Content>
      <Sidebar.Group>
        <Sidebar.GroupContent>
          <Sidebar.Menu>
            <Sidebar.MenuItem>
              <!-- The unread count is absolutely positioned chrome, so the
                   name has to be told to stop before it. -->
              <Sidebar.MenuButton
                data-testid="collection-all"
                class={unreadTotal > 0 ? "pr-10" : undefined}
                isActive={collection.type === "all"}
                aria-current={collection.type === "all"}
                onclick={() => selectCollection({ type: "all" })}
              >
                <InboxIcon strokeWidth={1.5} />
                <span class="truncate">All Feeds</span>
              </Sidebar.MenuButton>
              {#if unreadTotal > 0}
                <Sidebar.MenuBadge class="top-1 tabular-nums">
                  {unreadTotal}
                </Sidebar.MenuBadge>
              {/if}
            </Sidebar.MenuItem>
            <Sidebar.MenuItem>
              <Sidebar.MenuButton
                data-testid="collection-starred"
                isActive={collection.type === "starred"}
                aria-current={collection.type === "starred"}
                onclick={() => selectCollection({ type: "starred" })}
              >
                <StarIcon strokeWidth={1.5} />
                <span class="truncate">Starred</span>
              </Sidebar.MenuButton>
            </Sidebar.MenuItem>
            <Sidebar.MenuItem>
              <Sidebar.MenuButton
                data-testid="collection-archive"
                isActive={collection.type === "archive"}
                aria-current={collection.type === "archive"}
                onclick={() => selectCollection({ type: "archive" })}
              >
                <ArchiveIcon strokeWidth={1.5} />
                <span class="truncate">Archive</span>
              </Sidebar.MenuButton>
            </Sidebar.MenuItem>
          </Sidebar.Menu>
        </Sidebar.GroupContent>
      </Sidebar.Group>

      <Sidebar.Group>
        <Sidebar.GroupLabel>Feeds</Sidebar.GroupLabel>
        <Sidebar.GroupAction aria-label="Add Feed" onclick={() => (addFeedOpen = true)}>
          <PlusIcon />
        </Sidebar.GroupAction>
        <Sidebar.GroupContent>
          <Sidebar.Menu>
            {#each sortedFeeds as feed (feed.id)}
              <Sidebar.MenuItem>
                {#if editingFeed === feed.id}
                  <Input
                    bind:ref={feedTitleInput}
                    class="h-8 text-sm"
                    aria-label={`Rename the Feed ${feed.title}`}
                    bind:value={feedTitleDraft}
                    onblur={() => saveFeedTitle(feed.id)}
                    onkeydown={(event) => {
                      if (event.key === "Enter") saveFeedTitle(feed.id);
                      if (event.key === "Escape") editingFeed = undefined;
                    }}
                  />
                {:else}
                  <FeedRow
                    {feed}
                    isActive={collection.type === "feed" &&
                      collection.id === feed.id}
                    onSelect={() => selectCollection({ type: "feed", id: feed.id })}
                    onRename={() => startEditFeed(feed)}
                    onDelete={() => removeFeed(feed)}
                  />
                {/if}
              </Sidebar.MenuItem>
            {/each}
          </Sidebar.Menu>
        </Sidebar.GroupContent>
      </Sidebar.Group>
    </Sidebar.Content>

    <!-- Everything the app knows about itself sits in one row at the foot of
         the Collection List: which server this is, the one preference, the
         two dialogs, and the way out. The menu matches the trigger's width,
         so it opens upward over the list rather than beside it, and the same
         placement works inside the mobile sheet. -->
    <Sidebar.Footer>
      <Sidebar.Menu>
        <Sidebar.MenuItem>
          <DropdownMenu.Root bind:open={readerMenuOpen}>
            <DropdownMenu.Trigger>
              {#snippet child({ props })}
                <Sidebar.MenuButton {...props} size="lg" data-testid="reader-menu">
                  <span
                    class="flex aspect-square size-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground"
                  >
                    <RssIcon strokeWidth={1.5} class="size-4" />
                  </span>
                  <!-- The host is the whole of this app's notion of identity,
                       and a long one truncates rather than growing the row. -->
                  <span class="grid flex-1 leading-tight">
                    <span class="truncate font-medium">Reader</span>
                    <span class="truncate text-xs text-muted-foreground">{serverHost}</span>
                  </span>
                  <ChevronsUpDownIcon strokeWidth={1.5} class="size-4 shrink-0" />
                </Sidebar.MenuButton>
              {/snippet}
            </DropdownMenu.Trigger>
            <DropdownMenu.Content side="top" align="start" data-testid="reader-menu-content">
              <DropdownMenu.Group>
                <!-- A preference is declared whole, so no control that writes
                     one is live until the stored preferences have arrived: a
                     click before then would send this page's defaults as if
                     the reader had chosen them. The menu stays open on toggle
                     so the tick lands where the reader is looking. -->
                <DropdownMenu.CheckboxItem
                  checked={markOnOpen}
                  disabled={loading}
                  closeOnSelect={false}
                  onCheckedChange={() => toggleMarkOnOpen()}
                >
                  Mark Read on open
                </DropdownMenu.CheckboxItem>
              </DropdownMenu.Group>
              <DropdownMenu.Separator />
              <DropdownMenu.Group>
                <DropdownMenu.Item onSelect={() => (helpOpen = true)}>
                  <CircleHelpIcon strokeWidth={1.5} />
                  Keyboard shortcuts
                  <DropdownMenu.Shortcut aria-hidden="true">?</DropdownMenu.Shortcut>
                </DropdownMenu.Item>
                <DropdownMenu.Item onSelect={() => (deviceTokensOpen = true)}>
                  <KeyRoundIcon strokeWidth={1.5} />
                  Device tokens
                </DropdownMenu.Item>
              </DropdownMenu.Group>
              <DropdownMenu.Separator />
              <DropdownMenu.Group>
                <DropdownMenu.Item onSelect={signOut}>
                  <LogOutIcon strokeWidth={1.5} />
                  Sign out
                </DropdownMenu.Item>
              </DropdownMenu.Group>
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        </Sidebar.MenuItem>
      </Sidebar.Menu>
    </Sidebar.Footer>
  </Sidebar.Root>

  <!-- `@container` is what every `@3xl:` below asks: the pane pair is sized
       against this box, so the layout changes when the room changes rather
       than when the window does. -->
  <Sidebar.Inset
    bind:ref={inset}
    class="@container relative h-svh flex-row overflow-hidden"
  >
    <div
      data-testid="entry-list"
      class="flex h-full w-full flex-col overflow-hidden @3xl:w-88 @3xl:shrink-0"
      inert={overlayUp}
    >
      <Tooltip.Provider delayDuration={400}>
        <!-- Identity leads, actions trail, and the two are one gap apart
             against the half-gap inside the action group. The Collection's
             own name keeps a floor: the controls are all `shrink-0`, so
             without one the title is the only thing that can give, and it
             gives all of it — "All…" on the narrowest phone. The floor is
             5rem because that is what the 22rem column leaves once the
             controls have taken theirs; below the width where both fit, the
             actions take a second row instead. -->
        <div
          class="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-2 border-b border-border p-3"
        >
          <Sidebar.Trigger class="-ms-1 shrink-0 @max-3xl:size-9" />
          <h1
            data-testid="collection"
            class="min-w-20 flex-1 truncate px-1 text-base font-medium"
          >
            {collectionTitle}
          </h1>
          <div class="flex shrink-0 items-center gap-1">
            {#if unreadOnlyOffered}
              <!-- Pressed fills the pill with the colour the unread dot on
                   every row is already drawn in, so the control and the mark
                   it selects on read as the same thing. The label never
                   changes: it names what the toggle controls, never what
                   pressing it would do next. -->
              <Toggle
                data-testid="unread-only"
                variant="outline"
                class="shrink-0 aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground aria-pressed:hover:bg-primary/90 aria-pressed:hover:text-primary-foreground @max-3xl:h-9"
                pressed={unreadOnly}
                onPressedChange={(next) => void setUnreadOnly(next)}
                disabled={busy || loading}
              >
                Unread
              </Toggle>
            {/if}
            <Label for="entry-order" class="sr-only">
              Sort Entries by publish date
            </Label>
            <Select.Root
              type="single"
              value={entryOrder}
              onValueChange={(value) =>
                void setEntryOrder(value as EntryOrder)}
              disabled={busy}
            >
              <!-- The order is the least-touched control in a header that now
                   also carries Unread, so it keeps its name for the
                   accessibility tree and gives the width back to the
                   Collection's own title. -->
              <Select.Trigger
                id="entry-order"
                data-testid="entry-order"
                aria-label="Sort Entries by publish date"
                class="shrink-0 @max-3xl:h-9"
              >
                {#if entryOrder === "newest"}
                  <ArrowDownWideNarrowIcon strokeWidth={1.5} />
                {:else}
                  <ArrowUpNarrowWideIcon strokeWidth={1.5} />
                {/if}
                <span class="sr-only">
                  {entryOrder === "newest" ? "Newest first" : "Oldest first"}
                </span>
              </Select.Trigger>
              <Select.Content>
                <Select.Group>
                  <Select.Item value="newest" label="Newest first">
                    Newest first
                  </Select.Item>
                  <Select.Item value="oldest" label="Oldest first">
                    Oldest first
                  </Select.Item>
                </Select.Group>
              </Select.Content>
            </Select.Root>
            {#if canMarkAllRead}
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon-sm"
                      class="@max-3xl:size-9"
                      aria-label="Mark all read"
                      onclick={markAllRead}
                      disabled={busy || entries.length === 0}
                    >
                      <CheckCheckIcon />
                    </Button>
                  {/snippet}
                </Tooltip.Trigger>
                <Tooltip.Content>Mark all read</Tooltip.Content>
              </Tooltip.Root>
            {/if}
            <Tooltip.Root>
              <Tooltip.Trigger>
                {#snippet child({ props })}
                  <Button
                    {...props}
                    variant="ghost"
                    size="icon-sm"
                    class="@max-3xl:size-9"
                    aria-label="Refresh all"
                    onclick={refresh}
                    disabled={busy}
                  >
                    <IconSwap active={refreshing}>
                      {#snippet on()}
                        <LoaderCircleIcon class="animate-spin" />
                      {/snippet}
                      {#snippet off()}
                        <RefreshCwIcon />
                      {/snippet}
                    </IconSwap>
                  </Button>
                {/snippet}
              </Tooltip.Trigger>
              <Tooltip.Content>Refresh all</Tooltip.Content>
            </Tooltip.Root>
          </div>
        </div>
      </Tooltip.Provider>

      <!-- A notice belongs to the chrome, not to the reading list: it reports
           and leaves, without moving the row the reader was aiming at. The
           row stays mounted at zero height so both directions can animate;
           only its test id and the dismiss button's tab stop come and go
           with the notice, which is what keeps it out of the tab order and
           out of the DOM-query specs while collapsed. -->
      <div
        class={cn(
          "grid shrink-0 transition-[grid-template-rows] duration-200 ease-out motion-reduce:duration-0",
          noticeVisible ? "grid-rows-[1fr]" : "grid-rows-[0fr]",
        )}
      >
        <div class="overflow-hidden">
          <div
            data-testid={noticeVisible ? "notice" : undefined}
            role="status"
            class="flex items-start gap-2 border-b border-border bg-muted/50 py-2 pe-1.5 ps-3 text-xs text-muted-foreground"
          >
            <span
              class={cn(
                "min-w-0 flex-1 pt-0.5 transition-opacity duration-150 ease-out",
                !noticeVisible && "opacity-0",
              )}
            >
              {displayedNotice}
            </span>
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label="Dismiss"
              tabindex={noticeVisible ? 0 : -1}
              aria-hidden={!noticeVisible}
              onclick={() => (notice = "")}
            >
              <XIcon />
            </Button>
          </div>
        </div>
      </div>

      <div class="flex-1 overflow-y-auto scrollbar-hover">
        {#if loading}
          <!-- The shape of the list that is coming, so the first rows land in
               place instead of replacing a sentence. -->
          <ul class="flex flex-col divide-y divide-border" aria-hidden="true">
            {#each [0, 1, 2, 3, 4, 5] as placeholder (placeholder)}
              <li class="flex flex-col gap-2 px-3 py-3">
                <Skeleton class="h-3 w-32 rounded-md" />
                <Skeleton class="h-4 w-full rounded-md" />
                <Skeleton class="h-3 w-3/4 rounded-md" />
              </li>
            {/each}
          </ul>
          <p class="sr-only">Loading your Entries…</p>
        {:else if entries.length === 0}
          <div class="flex flex-col items-center gap-2 px-6 py-16 text-center">
            <InboxIcon
              class="size-6 text-muted-foreground/60"
              aria-hidden="true"
            />
            <p class="text-sm font-medium">
              {feeds.length === 0 ? "No Feeds yet." : emptyState.title}
            </p>
            <p class="max-w-56 text-xs leading-snug text-muted-foreground">
              {feeds.length === 0
                ? "Subscribe to a Feed and its Entries collect here."
                : emptyState.detail}
            </p>
            {#if feeds.length === 0}
              <!-- The first screen a self-hoster sees has to carry the act
                   itself: the + it used to point at is beside Feeds in the
                   Collection List, which before there is room for a column is
                   a sheet behind the trigger and so not on the screen at all. -->
              <Button
                variant="outline"
                size="sm"
                class="mt-1"
                data-testid="add-first-feed"
                onclick={() => (addFeedOpen = true)}
              >
                <PlusIcon data-icon="inline-start" />
                Add your first Feed
              </Button>
            {/if}
          </div>
        {:else}
          <ul class="flex flex-col divide-y divide-border">
            {#each entries as entry, index (entry.id)}
              <EntryRow
                {entry}
                isCurrent={index === selectedIndex}
                iconUrl={iconForEntry(entry)}
                tabbable={index === (position ?? 0)}
                {focusRequest}
                disabled={busy || pendingEntryIDs.has(entry.id)}
                onClick={() => selectEntryAt(index)}
                onToggleRead={() => toggleReadAt(index)}
                onToggleStar={() => toggleStarAt(index)}
                onToggleArchive={() => toggleArchiveAt(index)}
              />

            {/each}
          </ul>

          {#if cursor}
            <div class="p-3">
              <Button
                variant="outline"
                size="sm"
                onclick={loadMore}
                disabled={busy}
              >
                {#if busy}
                  <LoaderCircleIcon
                    class="animate-spin"
                    data-icon="inline-start"
                  />
                {/if}
                Load more
              </Button>
            </div>
          {/if}
        {/if}
      </div>
    </div>

    {#if selectedEntry}
      <ReadingPane
        entry={selectedEntry}
        busy={busy || pendingEntryIDs.has(selectedEntry.id)}
        view={entryView}
        {readingFont}
        iconUrl={iconForEntry(selectedEntry)}
        overlay={narrow}
        onClose={clearSelection}
        onView={chooseEntryView}
        onFontChange={chooseReadingFont}
        onToggleRead={toggleReadCurrent}
        onToggleStar={toggleStarCurrent}
        onToggleArchive={toggleArchiveCurrent}
      />
    {:else}
      <div
        data-testid="reading-pane-empty"
        class="hidden flex-1 flex-col items-center justify-center gap-3 border-s border-border px-6 text-center @3xl:flex"
      >
        <BookOpenIcon
          class="size-6 text-muted-foreground/60"
          aria-hidden="true"
        />
        <p class="text-sm font-medium">Pick an Entry to read it here.</p>
        <p class="text-xs text-muted-foreground">
          <kbd
            class="rounded-md border border-border bg-muted px-1.5 py-0.5 font-mono"
            >j</kbd
          >
          and
          <kbd
            class="rounded-md border border-border bg-muted px-1.5 py-0.5 font-mono"
            >k</kbd
          >
          move through the list.
          <kbd
            class="rounded-md border border-border bg-muted px-1.5 py-0.5 font-mono"
            >?</kbd
          > lists every shortcut.
        </p>
      </div>
    {/if}
  </Sidebar.Inset>
</Sidebar.Provider>

{#if helpOpen}
  <HelpDialog onClose={() => (helpOpen = false)} />
{/if}

{#if addFeedOpen}
  <AddFeedDialog
    onClose={() => (addFeedOpen = false)}
    onFeedAdded={handleFeedAdded}
  />
{/if}

{#if deviceTokensOpen}
  <DeviceTokensDialog onClose={() => (deviceTokensOpen = false)} />
{/if}

{#if searchOpen}
  <SearchDialog
    onClose={() => (searchOpen = false)}
    onSelectEntry={openSearchEntry}
    onSelectFeed={openSearchFeed}
  />
{/if}

{#if removal}
  <ConfirmDialog
    title={removal.title}
    description={removal.description}
    confirmLabel={removal.confirmLabel}
    onConfirm={confirmRemoval}
    onCancel={() => (removal = undefined)}
  />
{/if}
