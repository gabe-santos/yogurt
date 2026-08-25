<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import * as Alert from "$lib/components/ui/alert";
  import * as Sidebar from "$lib/components/ui/sidebar";
  import * as Select from "$lib/components/ui/select";
  import * as Tabs from "$lib/components/ui/tabs";
  import * as Tooltip from "$lib/components/ui/tooltip";
  import { Skeleton } from "$lib/components/ui/skeleton";
  import IconSwap from "$lib/IconSwap.svelte";
  import CircleHelpIcon from "@lucide/svelte/icons/circle-help";
  import InboxIcon from "@lucide/svelte/icons/inbox";
  import KeyRoundIcon from "@lucide/svelte/icons/key-round";
  import LoaderCircleIcon from "@lucide/svelte/icons/loader-circle";
  import PlusIcon from "@lucide/svelte/icons/plus";
  import SearchIcon from "@lucide/svelte/icons/search";
  import Settings2Icon from "@lucide/svelte/icons/settings-2";
  import XIcon from "@lucide/svelte/icons/x";
  import { onMount } from "svelte";
  import { prefersReducedMotion } from "svelte/motion";
  import { fly } from "svelte/transition";
  import { cubicOut } from "svelte/easing";
  import { goto, invalidateAll, replaceState } from "$app/navigation";
  import {
    ApiError,
    addFeed,
    createGroup,
    deleteFeed,
    deleteGroup,
    feedIconUrl,
    getSettings,
    listEntries,
    listFeeds,
    listGroups,
    logOut,
    markEntriesRead,
    refreshFeeds,
    renameGroup,
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
    Group,
    SearchEntry,
    Settings,
  } from "$lib/api";
  import ReadingPane from "$lib/ReadingPane.svelte";
  import { IsMobile } from "$lib/hooks/is-mobile.svelte.js";
  import BookOpenIcon from "@lucide/svelte/icons/book-open";
  import CheckCheckIcon from "@lucide/svelte/icons/check-check";
  import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
  import ConfirmDialog from "$lib/ConfirmDialog.svelte";
  import EntryRow from "$lib/EntryRow.svelte";
  import FeedRow from "$lib/FeedRow.svelte";
  import GroupRow from "$lib/GroupRow.svelte";
  import { formatPublished } from "$lib/format";
  import DeviceTokensDialog from "$lib/DeviceTokensDialog.svelte";
  import HelpDialog from "$lib/HelpDialog.svelte";
  import SearchDialog from "$lib/SearchDialog.svelte";
  import { bindings, matches } from "$lib/keys";
  import type { Action } from "$lib/keys";

  /** Scope is what the reading list is narrowed to: a single Feed, a single
   * Group, or (when undefined) every Feed. */
  type Scope = { type: "feed"; id: number } | { type: "group"; id: number };
  type Filter = "all" | "unread" | "starred" | "archive";
  type FilterDefinition = {
    query: EntrySelectionOptions;
    includes: (entry: Entry) => boolean;
    empty: string;
    /** emptyDetail says why the view is empty and what fills it, so an empty
     * Entry List is never just an absence the reader has to interpret. */
    emptyDetail: string;
    canMarkAllRead: boolean;
  };
  const filterDefinitions: Record<Filter, FilterDefinition> = {
    all: {
      query: {},
      includes: (entry) => !entry.archived,
      empty: "Nothing to read here yet.",
      emptyDetail: "New Entries appear here as your Feeds are checked.",
      canMarkAllRead: true,
    },
    unread: {
      query: { unread: true },
      includes: (entry) => !entry.archived && !entry.read,
      empty: "Nothing unread here.",
      emptyDetail: "Everything in this view has been read.",
      canMarkAllRead: true,
    },
    starred: {
      query: { starred: true },
      includes: (entry) => !entry.archived && entry.starred,
      empty: "Nothing Starred here.",
      emptyDetail:
        "Star an Entry to keep it. Starred Entries are never cleaned up.",
      canMarkAllRead: true,
    },
    archive: {
      query: { archived: true },
      includes: (entry) => entry.archived,
      empty: "The Archive is empty.",
      emptyDetail:
        "Archiving an Entry marks it Read and removes it from every other view.",
      canMarkAllRead: false,
    },
  };

  // The reading list is the server's, and this page mutates it as the reader
  // works, so it owns the copy rather than deriving one from a load function.
  let feeds = $state<Feed[]>([]);
  let groups = $state<Group[]>([]);
  let entries = $state<Entry[]>([]);
  let cursor = $state("");
  let scope = $state<Scope | undefined>(undefined);
  let filter = $state<Filter>("all");
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
  // shows the selected Entry, so a single index is the whole notion of
  // position. See docs/adr/0010-selection-is-opening.md.
  let selectedIndex = $state<number | undefined>(undefined);
  // The Reading Pane is a column of the layout once there is room for three,
  // and an overlay over the Entry List before that. The list behind an
  // overlay must not be reachable by tab, which is what narrow decides.
  const narrow = new IsMobile(1024);
  // selectionRestored stops the empty selection the page starts with from
  // clearing the Entry named in the address bar before it has been read.
  let selectionRestored = $state(false);
  let helpOpen = $state(false);
  let deviceTokensOpen = $state(false);
  let searchOpen = $state(false);
  let markOnOpen = $state(true);
  // The view an Entry opens in belongs to the reader, not to an Entry: it is
  // stored on the server, so it survives both moving to the next Entry and
  // coming back tomorrow in another browser.
  let entryView = $state<EntryView>("feed");
  // Entries the reader has declared unread by hand this session: mark-on-open
  // must never re-mark them Read just because j/k passed back through them.
  let manuallyUnread = $state<Set<number>>(new Set());
  let pendingEntryIDs = $state<Set<number>>(new Set());

  let address = $state("");
  let subscribing = $state(false);
  let subscribeError = $state("");
  let notice = $state("");
  let busy = $state(false);
  // busy blocks every list-changing action; refreshing is narrower, so only
  // the control the reader actually pressed reports that it is working.
  let refreshing = $state(false);

  let newGroupName = $state("");
  let creatingGroup = $state(false);
  let editingGroup = $state<number | undefined>(undefined);
  let groupNameDraft = $state("");
  let editingFeed = $state<number | undefined>(undefined);
  let feedTitleDraft = $state("");
  // Renaming is reached from a Feed's or Group's own context menu, so the
  // field it opens takes focus and selects the current name: the menu closed
  // over it, and nothing else would put a cursor there.
  let groupNameInput = $state<HTMLInputElement | null>(null);
  let feedTitleInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    groupNameInput?.select();
  });
  $effect(() => {
    feedTitleInput?.select();
  });
  // Managing the collection — renaming, moving, suspending, deleting — is rare
  // next to reading it, so the Feed List is navigation at rest and reveals its
  // controls only when the reader asks for them. Without this, every Feed cost
  // four rows of chrome and the list stopped being scannable.
  let managing = $state(false);
  /** Removal is the one act this app cannot undo, so it is asked in the app's
   * own dialog rather than the browser's, and the consequence is named. */
  type Removal = {
    title: string;
    description: string;
    confirmLabel: string;
    run: () => Promise<void>;
  };
  let removal = $state<Removal | undefined>(undefined);

  const scopedFeed = $derived.by(() => {
    const current = scope;
    return current && current.type === "feed"
      ? feeds.find((f) => f.id === current.id)
      : undefined;
  });
  const scopedGroup = $derived.by(() => {
    const current = scope;
    return current && current.type === "group"
      ? groups.find((g) => g.id === current.id)
      : undefined;
  });
  const scopeTitle = $derived(
    scopedFeed?.title ?? scopedGroup?.name ?? "All Feeds",
  );
  const selectedEntry = $derived(
    selectedIndex !== undefined ? entries[selectedIndex] : undefined,
  );
  const overlayUp = $derived(narrow.current && selectedEntry !== undefined);
  const feedsByGroup = $derived.by(() => {
    const map = new Map<number, Feed[]>();
    for (const feed of feeds) {
      const list = map.get(feed.group_id) ?? [];
      list.push(feed);
      map.set(feed.group_id, list);
    }
    return map;
  });

  onMount(async () => {
    // An Entry named in the address bar is restored in place: the list loads
    // around it rather than from the top, so a reload leaves the reader where
    // they were with the rest of the list still under them.
    const deepLink = Number(
      new URL(window.location.href).searchParams.get("entry") ?? "",
    );
    try {
      const [subscribed, page, settings, subscribedGroups] = await Promise.all([
        listFeeds(),
        listEntries({
          ...(deepLink > 0 ? { around: deepLink } : {}),
          order: entryOrder,
        }),
        getSettings(),
        listGroups(),
      ]);
      feeds = subscribed;
      entries = page.entries;
      cursor = page.next_cursor;
      markOnOpen = settings.mark_on_open;
      entryView = settings.entry_view;
      groups = subscribedGroups;
      if (deepLink > 0) {
        const index = entries.findIndex((entry) => entry.id === deepLink);
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
   * mark-all-read, so bulk state cannot drift beyond the visible selection. */
  function selectionQuery(
    currentScope: Scope | undefined = scope,
    currentFilter: Filter = filter,
  ): EntrySelectionOptions {
    return {
      feed: currentScope?.type === "feed" ? currentScope.id : undefined,
      group: currentScope?.type === "group" ? currentScope.id : undefined,
      ...filterDefinitions[currentFilter].query,
    };
  }

  /** reportError shows an ApiError's own message, or a generic one for
   * anything else (a network failure, a body the server never sent). */
  function reportError(cause: unknown) {
    notice =
      cause instanceof ApiError ? cause.message : "Could not reach the server";
  }

  /** reload replaces the list with the first page of the current scope and
   * filter, clearing keyboard position: the underlying list changed under it. */
  async function reload() {
    const page = await listEntries({ ...selectionQuery(), order: entryOrder });
    entries = page.entries;
    cursor = page.next_cursor;
    selectedIndex = undefined;
    manuallyUnread = new Set();
  }

  /** refreshCounts re-reads Feeds and Groups so their unread counts stay
   * correct after an Entry's Read state, or the collection itself, changes. */
  async function refreshCounts() {
    const [nextFeeds, nextGroups] = await Promise.all([
      listFeeds(),
      listGroups(),
    ]);
    feeds = nextFeeds;
    groups = nextGroups;
  }

  async function subscribe(event: SubmitEvent) {
    event.preventDefault();
    subscribeError = "";
    notice = "";
    subscribing = true;
    try {
      const feed = await addFeed(address);
      address = "";
      await refreshCounts();
      scope = { type: "feed", id: feed.id };
      await reload();
      notice = `Subscribed to ${feed.title}.`;
    } catch (cause) {
      subscribeError =
        cause instanceof ApiError
          ? cause.message
          : "Could not reach the server";
    } finally {
      subscribing = false;
    }
  }

  async function refresh() {
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

  async function scopeTo(next: Scope | undefined) {
    if (busy) return;
    scope = next;
    busy = true;
    try {
      await reload();
    } finally {
      busy = false;
    }
  }

  /** openSearchEntry opens a search result within its ordinary list: scoped
   * to its own Feed, in the filter that view normally lives in, anchored at
   * the Entry itself rather than the top of the list — not a standalone
   * search-result view. */
  async function openSearchEntry(entry: SearchEntry) {
    searchOpen = false;
    const nextScope: Scope = { type: "feed", id: entry.feed_id };
    const nextFilter: Filter = entry.archived ? "archive" : "all";
    scope = nextScope;
    filter = nextFilter;
    busy = true;
    try {
      const page = await listEntries({
        ...selectionQuery(nextScope, nextFilter),
        around: entry.id,
        order: entryOrder,
      });
      entries = page.entries;
      cursor = page.next_cursor;
      manuallyUnread = new Set();
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
    void scopeTo({ type: "feed", id: feed.id });
  }

  async function setFilter(next: Filter) {
    if (busy || filter === next) {
      return;
    }
    filter = next;
    busy = true;
    try {
      await reload();
    } finally {
      busy = false;
    }
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
   * only this Entry, so another Entry's concurrent success cannot be erased. */
  async function applyEntryState(
    index: number,
    state: EntryState,
    manualRead = false,
    // advance separates the reader's own triage from the Read the Reading Pane
    // sets by itself. Triage takes an Entry out of a view it no longer belongs
    // to and moves on; the automatic Read must not, or selecting an Entry in
    // the Unread filter would empty the list one arrival at a time. The Entry
    // being read stays until the reader leaves it, in leave() below.
    advance = true,
  ) {
    const previous = entries[index];
    if (busy || pendingEntryIDs.has(previous.id)) return;

    const previousManualUnread = manuallyUnread.has(previous.id);
    const mutationFilter = filter;
    const mutationScope = scope;
    pendingEntryIDs = new Set([...pendingEntryIDs, previous.id]);
    const optimistic = {
      ...previous,
      ...state,
      read: state.archived ? true : state.read,
    };

    let arrived: number | undefined;
    if (advance && !filterDefinitions[filter].includes(optimistic)) {
      entries = entries.filter((entry) => entry.id !== previous.id);
      if (selectedIndex !== undefined) {
        if (entries.length === 0) {
          selectedIndex = undefined;
        } else if (selectedIndex === index) {
          // The next Entry has slid into the triaged one's place. On the last
          // row there is no next, so the new last row takes the selection
          // rather than the Reading Pane emptying itself.
          selectedIndex = Math.min(index, entries.length - 1);
          arrived = selectedIndex;
        } else if (selectedIndex > index) {
          selectedIndex--;
        }
      }
    } else {
      entries[index] = optimistic;
    }
    if (manualRead && !state.read) {
      manuallyUnread = new Set([...manuallyUnread, previous.id]);
    } else if (manualRead) {
      manuallyUnread = new Set([...manuallyUnread].filter((id) => id !== previous.id));
    }
    // Whatever the triage moved the reader on to is in the Reading Pane now,
    // and is Read on the same terms as any other arrival there.
    if (arrived !== undefined) maybeMarkOnSelect(arrived);

    let stored: Entry;
    try {
      stored = await setEntryState(previous.id, state);
    } catch (cause) {
      pendingEntryIDs = new Set([...pendingEntryIDs].filter((id) => id !== previous.id));
      if (filter !== mutationFilter || scope !== mutationScope) {
        await reload();
      } else {
        const activeSelectedID =
          selectedIndex === undefined ? undefined : entries[selectedIndex]?.id;
        const existingIndex = entries.findIndex((entry) => entry.id === previous.id);
        if (filterDefinitions[filter].includes(previous)) {
          if (existingIndex >= 0) {
            entries[existingIndex] = previous;
          } else {
            entries = [...entries, previous].sort(compareEntries);
          }
        } else if (existingIndex >= 0) {
          entries = entries.filter((entry) => entry.id !== previous.id);
        }
        // Restored by identity, never by index: the list has shifted under the
        // reader, and whatever the Reading Pane is showing now has to keep
        // showing rather than be replaced by the Entry that came back.
        const selected =
          activeSelectedID === undefined
            ? -1
            : entries.findIndex((entry) => entry.id === activeSelectedID);
        selectedIndex = selected < 0 ? undefined : selected;
      }
      if (previousManualUnread) {
        manuallyUnread = new Set([...manuallyUnread, previous.id]);
      } else {
        manuallyUnread = new Set([...manuallyUnread].filter((id) => id !== previous.id));
      }
      reportError(cause);
      return;
    }

    pendingEntryIDs = new Set([...pendingEntryIDs].filter((id) => id !== stored.id));
    if (filter !== mutationFilter || scope !== mutationScope) {
      await reload();
    } else {
      const storedIndex = entries.findIndex((entry) => entry.id === stored.id);
      if (storedIndex >= 0) {
        if (!advance || filterDefinitions[filter].includes(stored)) {
          entries[storedIndex] = stored;
        } else {
          entries = entries.filter((entry) => entry.id !== stored.id);
        }
      }
    }
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
      manual,
    );
  }

  /** leave drops the Entry the reader is moving away from when the Read it
   * earned in the Reading Pane has left it outside the current filter. An
   * Entry stays put while it is being read and goes when the reader goes,
   * which keeps the Unread filter honest without the list shifting under the
   * Entry still on screen. Reports whether it removed anything. */
  function leave(leaving: number | undefined, staying: number | undefined): boolean {
    if (leaving === undefined || leaving === staying) return false;
    const entry = entries[leaving];
    if (!entry || pendingEntryIDs.has(entry.id)) return false;
    if (filterDefinitions[filter].includes(entry)) return false;
    entries = entries.filter((candidate) => candidate.id !== entry.id);
    return true;
  }

  async function moveSelection(delta: number) {
    if (busy || entries.length === 0) return;
    const base = selectedIndex ?? (delta > 0 ? -1 : entries.length);
    let target = base + delta;
    // The end of the loaded list is not the end of the reading list: j reaches
    // for the next page rather than stopping dead on the last row.
    if (target >= entries.length && cursor) {
      await loadMore();
    }
    if (entries.length === 0) return;
    const leaving = selectedIndex;
    target = Math.min(Math.max(target, 0), entries.length - 1);
    if (leave(leaving, target) && leaving !== undefined && target > leaving) {
      target -= 1;
    }
    selectedIndex = target;
    maybeMarkOnSelect(target);
  }

  function selectEntryAt(index: number) {
    if (busy) return;
    const leaving = selectedIndex;
    let target = index;
    if (leave(leaving, target) && leaving !== undefined && target > leaving) {
      target -= 1;
    }
    selectedIndex = target;
    maybeMarkOnSelect(target);
  }

  /** clearSelection backs out of the overlay the Reading Pane is before there
   * is room for a third column. With the room, there is nothing to back out
   * of and the pane keeps what it is showing. */
  function clearSelection() {
    if (busy || !narrow.current) return;
    leave(selectedIndex, undefined);
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

  function archiveAt(index: number) {
    const entry = entries[index];
    if (!entry || entry.archived) return;
    void applyEntryState(index, {
      read: true,
      starred: entry.starred,
      archived: true,
    });
  }

  function archiveCurrent() {
    if (selectedIndex !== undefined) archiveAt(selectedIndex);
  }

  async function markAllRead() {
    if (busy || !filterDefinitions[filter].canMarkAllRead) return;
    busy = true;
    const previousEntries = [...entries];
    const previousCursor = cursor;
    const previousSelected = selectedIndex;
    const previousManualUnread = new Set(manuallyUnread);

    entries = entries
      .map((entry) => ({ ...entry, read: true }))
      .filter(filterDefinitions[filter].includes);
    if (entries.length !== previousEntries.length) {
      cursor = "";
      selectedIndex = undefined;
    }
    manuallyUnread = new Set();
    try {
      await markEntriesRead(selectionQuery());
    } catch (cause) {
      entries = previousEntries;
      cursor = previousCursor;
      selectedIndex = previousSelected;
      manuallyUnread = previousManualUnread;
      reportError(cause);
      busy = false;
      return;
    }
    try {
      await refreshCounts();
      notice = "Marked this view Read.";
    } catch (cause) {
      reportError(cause);
    } finally {
      busy = false;
    }
  }

  // Preferences are declared whole, so every change sends both fields rather
  // than letting an omitted one fall back to a default the reader never chose.
  async function savePreferences(next: Settings, previous: Settings) {
    markOnOpen = next.mark_on_open;
    entryView = next.entry_view;
    try {
      const stored = await setSettings(next);
      markOnOpen = stored.mark_on_open;
      entryView = stored.entry_view;
    } catch {
      markOnOpen = previous.mark_on_open;
      entryView = previous.entry_view;
      notice = "Could not update your settings.";
    }
  }

  function preferences(): Settings {
    return { mark_on_open: markOnOpen, entry_view: entryView };
  }

  function toggleMarkOnOpen() {
    const previous = preferences();
    void savePreferences({ ...previous, mark_on_open: !markOnOpen }, previous);
  }

  function chooseEntryView(view: EntryView) {
    const previous = preferences();
    void savePreferences({ ...previous, entry_view: view }, previous);
  }

  /** sortGroups matches the server's own order (default first, then by
   * name), so a create or rename never leaves the sidebar out of step with
   * what the next listGroups() would return. */
  function sortGroups(list: Group[]): Group[] {
    return [...list].sort((a, b) => {
      if (a.is_default !== b.is_default) {
        return a.is_default ? -1 : 1;
      }
      return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
    });
  }

  async function submitNewGroup(event: SubmitEvent) {
    event.preventDefault();
    const name = newGroupName.trim();
    if (!name) {
      return;
    }
    creatingGroup = true;
    try {
      const group = await createGroup(name);
      groups = sortGroups([...groups, group]);
      newGroupName = "";
    } catch (cause) {
      reportError(cause);
    } finally {
      creatingGroup = false;
    }
  }

  function startEditGroup(group: Group) {
    editingGroup = group.id;
    groupNameDraft = group.name;
  }

  async function saveGroupName(id: number) {
    const name = groupNameDraft.trim();
    editingGroup = undefined;
    const current = groups.find((g) => g.id === id);
    if (!name || current?.name === name) {
      return;
    }
    try {
      const updated = await renameGroup(id, name);
      groups = sortGroups(groups.map((g) => (g.id === id ? updated : g)));
    } catch (cause) {
      reportError(cause);
    }
  }

  function removeGroup(group: Group) {
    removal = {
      title: `Delete the Group "${group.name}"?`,
      description:
        "Its Feeds move to the default Group and keep their Entries. The Group itself is gone for good.",
      confirmLabel: "Delete Group",
      run: async () => {
        await deleteGroup(group.id);
        if (scope?.type === "group" && scope.id === group.id) {
          scope = undefined;
        }
        await refreshCounts();
        await reload();
      },
    };
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

  async function moveFeed(feed: Feed, groupID: number) {
    try {
      const updated = await updateFeed(feed.id, { group_id: groupID });
      feeds = feeds.map((f) => (f.id === feed.id ? updated : f));
      await refreshCounts();
    } catch (cause) {
      reportError(cause);
    }
  }

  async function toggleSuspend(feed: Feed) {
    try {
      const updated = await updateFeed(feed.id, { suspended: !feed.suspended });
      feeds = feeds.map((f) => (f.id === feed.id ? updated : f));
    } catch (cause) {
      reportError(cause);
    }
  }

  function removeFeed(feed: Feed) {
    removal = {
      title: `Delete "${feed.title}"?`,
      description:
        "Every Entry it carried is deleted with it, Starred ones included. Suspend the Feed instead to stop checking it and keep what you have.",
      confirmLabel: "Delete Feed",
      run: async () => {
        await deleteFeed(feed.id);
        if (scope?.type === "feed" && scope.id === feed.id) {
          scope = undefined;
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
    if (searchOpen) {
      searchOpen = false;
    } else if (helpOpen) {
      helpOpen = false;
    } else if (deviceTokensOpen) {
      deviceTokensOpen = false;
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
    help: () => (helpOpen = true),
    search: () => (searchOpen = true),
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
        (helpOpen || searchOpen || deviceTokensOpen || removal) &&
        binding.action !== "close"
      ) {
        return;
      }
      actions[binding.action]();
      event.preventDefault();
      return;
    }
  }

  // A notice reports and leaves: the enter is soft, the exit softer and
  // shorter, and neither runs for a reader who asked for less motion.
  const noticeIn = $derived(
    prefersReducedMotion.current
      ? { duration: 0 }
      : { y: -8, duration: 200, easing: cubicOut },
  );
  const noticeOut = $derived(
    prefersReducedMotion.current
      ? { duration: 0 }
      : { y: -12, duration: 150, easing: cubicOut },
  );
</script>

<svelte:window onkeydown={onKeydown} />

<Sidebar.Provider>
  <Sidebar.Root>
    <Sidebar.Header>
      <div class="flex items-center justify-between gap-2 pl-2">
        <h1 class="text-base font-semibold">Reader</h1>
        <div class="flex items-center gap-0.5">
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Search"
            data-testid="open-search"
            onclick={() => (searchOpen = true)}
          >
            <SearchIcon />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Device tokens"
            onclick={() => (deviceTokensOpen = true)}
          >
            <KeyRoundIcon />
          </Button>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Keyboard shortcuts"
            onclick={() => (helpOpen = true)}
          >
            <CircleHelpIcon />
          </Button>
          <Button variant="ghost" size="sm" onclick={signOut}>Sign out</Button>
        </div>
      </div>

      <!-- Adding a Feed is one line: a field and the act, at the size of every
           other control. It used to be the loudest element in the app. -->
      <form class="flex flex-col gap-1.5 px-2" onsubmit={subscribe}>
        <Label for="address" class="text-xs font-normal text-muted-foreground">
          Feed or site address
        </Label>
        <div class="flex items-center gap-1.5">
          <Input
            id="address"
            name="address"
            type="url"
            required
            class="min-w-0 flex-1"
            placeholder="https://example.com"
            bind:value={address}
          />
          <Button
            type="submit"
            variant="outline"
            size="icon"
            aria-label="Subscribe"
            disabled={subscribing}
          >
            <IconSwap active={subscribing}>
              {#snippet on()}
                <LoaderCircleIcon class="animate-spin" />
              {/snippet}
              {#snippet off()}
                <PlusIcon />
              {/snippet}
            </IconSwap>
          </Button>
        </div>
        {#if subscribeError}
          <Alert.Root variant="destructive" class="px-0 py-1">
            <Alert.Description>{subscribeError}</Alert.Description>
          </Alert.Root>
        {/if}
      </form>
    </Sidebar.Header>

    <Sidebar.Content>
      <Sidebar.Group>
        <Sidebar.GroupLabel>Feeds</Sidebar.GroupLabel>
        <Sidebar.GroupAction
          aria-label={managing ? "Done managing Feeds" : "Manage Feeds"}
          aria-pressed={managing}
          data-testid="manage-feeds"
          class="aria-pressed:bg-sidebar-accent aria-pressed:text-sidebar-accent-foreground"
          onclick={() => (managing = !managing)}
        >
          <Settings2Icon />
        </Sidebar.GroupAction>
        <Sidebar.GroupContent>
          <Sidebar.Menu>
            <Sidebar.MenuItem>
              <Sidebar.MenuButton
                isActive={scope === undefined}
                aria-current={scope === undefined}
                onclick={() => scopeTo(undefined)}
              >
                All Feeds
              </Sidebar.MenuButton>
            </Sidebar.MenuItem>

            {#each groups as group (group.id)}
              <Sidebar.MenuItem>
                {#if editingGroup === group.id}
                  <Input
                    bind:ref={groupNameInput}
                    class="h-8 text-sm"
                    aria-label={`Rename the Group ${group.name}`}
                    bind:value={groupNameDraft}
                    onblur={() => saveGroupName(group.id)}
                    onkeydown={(event) => {
                      if (event.key === "Enter") saveGroupName(group.id);
                      if (event.key === "Escape") editingGroup = undefined;
                    }}
                  />
                {:else}
                  <GroupRow
                    {group}
                    isActive={scope?.type === "group" && scope.id === group.id}
                    onSelect={() => scopeTo({ type: "group", id: group.id })}
                    onRename={() => startEditGroup(group)}
                    onDelete={() => removeGroup(group)}
                  />
                {/if}
              </Sidebar.MenuItem>

              {#if managing && editingGroup !== group.id}
                <div class="flex items-center gap-1.5 px-2 py-1.5">
                  <Button
                    variant="outline"
                    size="xs"
                    onclick={() => startEditGroup(group)}
                  >
                    Rename
                  </Button>
                  {#if !group.is_default}
                    <Button
                      variant="destructive"
                      size="xs"
                      onclick={() => removeGroup(group)}
                    >
                      Delete
                    </Button>
                  {/if}
                </div>
              {/if}

              <Sidebar.MenuSub>
                {#each feedsByGroup.get(group.id) ?? [] as feed (feed.id)}
                  <Sidebar.MenuSubItem>
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
                        {groups}
                        isActive={scope?.type === "feed" &&
                          scope.id === feed.id}
                        onSelect={() => scopeTo({ type: "feed", id: feed.id })}
                        onRename={() => startEditFeed(feed)}
                        onToggleSuspend={() => toggleSuspend(feed)}
                        onMove={(groupID) => moveFeed(feed, groupID)}
                        onDelete={() => removeFeed(feed)}
                      />

                      {#if managing}
                        <div
                          class="mt-1 mb-1.5 flex flex-col gap-2 rounded-[calc(var(--radius)*1.8_+_8px)] bg-sidebar-accent/60 px-2 py-2 text-xs text-muted-foreground"
                        >
                          <!-- Two reversible acts side by side; the one that
                               cannot be undone sits alone at the bottom, where
                               nothing is next to it to be hit by mistake. -->
                          <div class="grid grid-cols-2 gap-1.5">
                            <Button
                              variant="outline"
                              size="xs"
                              onclick={() => startEditFeed(feed)}
                            >
                              Rename
                            </Button>
                            <Button
                              variant="outline"
                              size="xs"
                              onclick={() => toggleSuspend(feed)}
                            >
                              {feed.suspended ? "Resume" : "Suspend"}
                            </Button>
                          </div>
                          <label class="sr-only" for={`move-feed-${feed.id}`}>
                            Move {feed.title} to a Group
                          </label>
                          <select
                            id={`move-feed-${feed.id}`}
                            class="h-7 w-full min-w-0 rounded-2xl border border-transparent bg-input/50 px-2 text-xs text-foreground outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30"
                            value={feed.group_id}
                            onchange={(event) =>
                              moveFeed(
                                feed,
                                Number(
                                  (event.target as HTMLSelectElement).value,
                                ),
                              )}
                          >
                            {#each groups as option (option.id)}
                              <option value={option.id}>{option.name}</option>
                            {/each}
                          </select>
                          <!-- Silence and breakage read alike in a Feed List, so
                               the last check is stated rather than inferred. -->
                          <p
                            class={feed.last_error
                              ? "text-destructive"
                              : ""}
                            title={feed.last_checked_at
                              ? `Last checked ${formatPublished(feed.last_checked_at)}`
                              : "Not checked yet"}
                          >
                            {#if feed.last_error}
                              Failing: {feed.last_error}
                            {:else if feed.last_success_at}
                              Checked {formatPublished(feed.last_success_at)}
                            {:else}
                              Not checked yet
                            {/if}
                          </p>
                          <Button
                            variant="destructive"
                            size="xs"
                            class="w-full"
                            onclick={() => removeFeed(feed)}
                          >
                            Delete
                          </Button>
                        </div>
                      {/if}
                    {/if}
                  </Sidebar.MenuSubItem>
                {/each}
              </Sidebar.MenuSub>
            {/each}
          </Sidebar.Menu>

          {#if managing}
            <form
              class="mt-2 flex items-center gap-1.5 px-2"
              onsubmit={submitNewGroup}
            >
              <Label for="new-group" class="sr-only">New Group</Label>
              <Input
                id="new-group"
                class="h-7 min-w-0 flex-1 text-xs"
                placeholder="New Group"
                bind:value={newGroupName}
              />
              <Button
                type="submit"
                size="xs"
                variant="outline"
                disabled={creatingGroup}
              >
                Add
              </Button>
            </form>
          {/if}
        </Sidebar.GroupContent>
      </Sidebar.Group>
    </Sidebar.Content>

    <Sidebar.Footer>
      <label
        class="flex cursor-pointer items-start gap-2.5 rounded-xl px-2 py-2 text-xs leading-snug text-muted-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
      >
        <input
          type="checkbox"
          class="mt-px size-3.5 shrink-0 accent-primary"
          checked={markOnOpen}
          onchange={toggleMarkOnOpen}
        />
        Mark an Entry read when it opens in the Reading Pane
      </label>
    </Sidebar.Footer>
  </Sidebar.Root>

  <Sidebar.Inset class="relative h-svh flex-row overflow-hidden">
    <div
      data-testid="entry-list"
      class="flex h-full w-full flex-col overflow-hidden lg:w-88 lg:shrink-0"
      inert={overlayUp}
    >
      <Tooltip.Provider delayDuration={400}>
        <div class="flex shrink-0 flex-col gap-2 border-b border-border p-3">
          <div class="flex items-center gap-1">
            <Sidebar.Trigger class="-ml-1 shrink-0 max-lg:size-9" />
            <h2
              data-testid="scope"
              class="min-w-0 flex-1 truncate px-1 text-base font-medium"
            >
              {scopeTitle}
            </h2>
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
              <Select.Trigger
                id="entry-order"
                data-testid="entry-order"
                aria-label="Sort Entries by publish date"
                class="shrink-0 max-lg:h-9"
              >
                {entryOrder === "newest" ? "Newest first" : "Oldest first"}
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


            {#if filterDefinitions[filter].canMarkAllRead}
              <Tooltip.Root>
                <Tooltip.Trigger>
                  {#snippet child({ props })}
                    <Button
                      {...props}
                      variant="ghost"
                      size="icon-sm"
                      class="max-lg:size-9"
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
                    class="max-lg:size-9"
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

          <Tabs.Root
            value={filter}
            onValueChange={(value) => setFilter(value as Filter)}
          >
            <Tabs.List aria-label="Filter" class="w-full">
              <Tabs.Trigger value="all" data-testid="filter-all">All</Tabs.Trigger>
              <Tabs.Trigger value="unread" data-testid="filter-unread">
                Unread
              </Tabs.Trigger>
              <Tabs.Trigger value="starred" data-testid="filter-starred">
                Starred
              </Tabs.Trigger>
              <Tabs.Trigger value="archive" data-testid="filter-archive">
                Archive
              </Tabs.Trigger>
            </Tabs.List>
          </Tabs.Root>
        </div>
      </Tooltip.Provider>

      <!-- A notice belongs to the chrome, not to the reading list: it reports
           and leaves, without moving the row the reader was aiming at. -->
      {#if notice}
        <div
          data-testid="notice"
          role="status"
          class="flex shrink-0 items-start gap-2 border-b border-border bg-muted/50 py-2 pr-1.5 pl-3 text-xs text-muted-foreground"
          in:fly={noticeIn}
          out:fly={noticeOut}
        >
          <span class="min-w-0 flex-1 pt-0.5">{notice}</span>
          <Button
            variant="ghost"
            size="icon-xs"
            aria-label="Dismiss"
            onclick={() => (notice = "")}
          >
            <XIcon />
          </Button>
        </div>
      {/if}

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
              {feeds.length === 0
                ? "No Feeds yet."
                : filterDefinitions[filter].empty}
            </p>
            <p class="max-w-56 text-xs leading-snug text-muted-foreground">
              {feeds.length === 0
                ? "Add one with the address field at the top of the Feed List."
                : filterDefinitions[filter].emptyDetail}
            </p>
          </div>
        {:else}
          <ul class="flex flex-col divide-y divide-border">
            {#each entries as entry, index (entry.id)}
              <EntryRow
                {entry}
                isCurrent={index === selectedIndex}
                iconUrl={iconForEntry(entry)}
                tabbable={index === (selectedIndex ?? 0)}
                disabled={busy || pendingEntryIDs.has(entry.id)}
                onClick={() => selectEntryAt(index)}
                onToggleRead={() => toggleReadAt(index)}
                onToggleStar={() => toggleStarAt(index)}
                onArchive={() => archiveAt(index)}
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
        iconUrl={iconForEntry(selectedEntry)}
        onClose={clearSelection}
        onView={chooseEntryView}
        onToggleRead={toggleReadCurrent}
        onToggleStar={toggleStarCurrent}
        onArchive={archiveCurrent}
      />
    {:else}
      <div
        data-testid="reading-pane-empty"
        class="hidden flex-1 flex-col items-center justify-center gap-3 border-l border-border px-6 text-center lg:flex"
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
