<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import * as Field from "$lib/components/ui/field";
  import * as Alert from "$lib/components/ui/alert";
  import * as Sidebar from "$lib/components/ui/sidebar";
  import * as Tabs from "$lib/components/ui/tabs";
  import CircleHelpIcon from "@lucide/svelte/icons/circle-help";
  import KeyRoundIcon from "@lucide/svelte/icons/key-round";
  import SearchIcon from "@lucide/svelte/icons/search";
  import { onMount } from "svelte";
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
    EntrySelectionOptions,
    EntryView,
    Feed,
    Group,
    SearchEntry,
    Settings,
  } from "$lib/api";
  import ReadingPane from "$lib/ReadingPane.svelte";
  import { IsMobile } from "$lib/hooks/is-mobile.svelte.js";
  import CheckCheckIcon from "@lucide/svelte/icons/check-check";
  import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
  import EntryRow from "$lib/EntryRow.svelte";
  import FeedIcon from "$lib/FeedIcon.svelte";
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
    canMarkAllRead: boolean;
  };
  const filterDefinitions: Record<Filter, FilterDefinition> = {
    all: {
      query: {},
      includes: (entry) => !entry.archived,
      empty: "Nothing to read here yet.",
      canMarkAllRead: true,
    },
    unread: {
      query: { unread: true },
      includes: (entry) => !entry.archived && !entry.read,
      empty: "Nothing unread here.",
      canMarkAllRead: true,
    },
    starred: {
      query: { starred: true },
      includes: (entry) => !entry.archived && entry.starred,
      empty: "Nothing Starred here.",
      canMarkAllRead: true,
    },
    archive: {
      query: { archived: true },
      includes: (entry) => entry.archived,
      empty: "The Archive is empty.",
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

  let newGroupName = $state("");
  let creatingGroup = $state(false);
  let editingGroup = $state<number | undefined>(undefined);
  let groupNameDraft = $state("");
  let editingFeed = $state<number | undefined>(undefined);
  let feedTitleDraft = $state("");

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
        listEntries(deepLink > 0 ? { around: deepLink } : {}),
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
    const page = await listEntries(selectionQuery());
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

  async function loadMore() {
    if (!cursor) return;
    busy = true;
    try {
      const page = await listEntries({ ...selectionQuery(), cursor });
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
            entries = [...entries, previous].sort(
              (a, b) => b.published_at.localeCompare(a.published_at) || b.id - a.id,
            );
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

  function toggleReadCurrent() {
    const index = selectedIndex;
    if (index === undefined || entries[index].archived) {
      return;
    }
    void applyRead(index, !entries[index].read, true);
  }

  function toggleStarCurrent() {
    const index = selectedIndex;
    if (index === undefined) {
      return;
    }
    const entry = entries[index];
    void applyEntryState(index, {
      read: entry.read,
      starred: !entry.starred,
      archived: entry.archived,
    });
  }

  function archiveCurrent() {
    const index = selectedIndex;
    if (index === undefined || entries[index].archived) {
      return;
    }
    const entry = entries[index];
    void applyEntryState(index, {
      read: true,
      starred: entry.starred,
      archived: true,
    });
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

  async function removeGroup(group: Group) {
    if (
      !confirm(
        `Delete the Group "${group.name}"? Its Feeds move to the default Group.`,
      )
    ) {
      return;
    }
    try {
      await deleteGroup(group.id);
      if (scope?.type === "group" && scope.id === group.id) {
        scope = undefined;
      }
      await refreshCounts();
      await reload();
    } catch (cause) {
      reportError(cause);
    }
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

  async function removeFeed(feed: Feed) {
    if (!confirm(`Delete "${feed.title}" and every Entry it carried?`)) {
      return;
    }
    try {
      await deleteFeed(feed.id);
      if (scope?.type === "feed" && scope.id === feed.id) {
        scope = undefined;
      }
      await refreshCounts();
      await reload();
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
      if ((helpOpen || searchOpen || deviceTokensOpen) && binding.action !== "close") {
        return;
      }
      actions[binding.action]();
      event.preventDefault();
      return;
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<Sidebar.Provider>
  <Sidebar.Root>
    <Sidebar.Header>
      <div class="flex items-center justify-between gap-2 px-2">
        <h1 class="text-lg font-semibold">Reader</h1>
        <div class="flex items-center gap-1">
          <Button
            variant="outline"
            size="icon-sm"
            aria-label="Search"
            data-testid="open-search"
            onclick={() => (searchOpen = true)}
          >
            <SearchIcon />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            aria-label="Device tokens"
            onclick={() => (deviceTokensOpen = true)}
          >
            <KeyRoundIcon />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            aria-label="Keyboard shortcuts"
            onclick={() => (helpOpen = true)}
          >
            <CircleHelpIcon />
          </Button>
          <Button variant="outline" size="sm" onclick={signOut}>Sign out</Button
          >
        </div>
      </div>
    </Sidebar.Header>

    <Sidebar.Content>
      <Sidebar.Group>
        <Sidebar.GroupContent>
          <form class="flex flex-col gap-2" onsubmit={subscribe}>
            <Field.FieldGroup>
              <Field.Field>
                <Field.FieldLabel for="address"
                  >Feed or site address</Field.FieldLabel
                >
                <Input
                  id="address"
                  name="address"
                  type="url"
                  required
                  placeholder="https://example.com"
                  bind:value={address}
                />
              </Field.Field>
            </Field.FieldGroup>
            <Button type="submit" disabled={subscribing}>Subscribe</Button>
            {#if subscribeError}
              <Alert.Root variant="destructive">
                <Alert.Description>{subscribeError}</Alert.Description>
              </Alert.Root>
            {/if}
          </form>
        </Sidebar.GroupContent>
      </Sidebar.Group>

      <Sidebar.Group>
        <Sidebar.GroupLabel>Feeds</Sidebar.GroupLabel>
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
                    class="h-7 text-sm"
                    bind:value={groupNameDraft}
                    onblur={() => saveGroupName(group.id)}
                    onkeydown={(event) => {
                      if (event.key === "Enter") saveGroupName(group.id);
                      if (event.key === "Escape") editingGroup = undefined;
                    }}
                  />
                {:else}
                  <Sidebar.MenuButton
                    data-testid="group"
                    isActive={scope?.type === "group" && scope.id === group.id}
                    aria-current={scope?.type === "group" &&
                      scope.id === group.id}
                    onclick={() => scopeTo({ type: "group", id: group.id })}
                  >
                    <span class="truncate font-medium">{group.name}</span>
                  </Sidebar.MenuButton>
                  <Sidebar.MenuBadge>{group.unread_count}</Sidebar.MenuBadge>
                {/if}
              </Sidebar.MenuItem>
              {#if editingGroup !== group.id}
                <div
                  class="flex items-center gap-2 px-2 pb-1 text-xs text-muted-foreground"
                >
                  <button
                    type="button"
                    class="hover:underline"
                    onclick={() => startEditGroup(group)}
                  >
                    Rename
                  </button>
                  {#if !group.is_default}
                    <button
                      type="button"
                      class="hover:underline"
                      onclick={() => removeGroup(group)}
                    >
                      Delete
                    </button>
                  {/if}
                </div>
              {/if}

              <Sidebar.MenuSub>
                {#each feedsByGroup.get(group.id) ?? [] as feed (feed.id)}
                  <Sidebar.MenuSubItem>
                    {#if editingFeed === feed.id}
                      <Input
                        class="h-7 text-sm"
                        bind:value={feedTitleDraft}
                        onblur={() => saveFeedTitle(feed.id)}
                        onkeydown={(event) => {
                          if (event.key === "Enter") saveFeedTitle(feed.id);
                          if (event.key === "Escape") editingFeed = undefined;
                        }}
                      />
                    {:else}
                      <Sidebar.MenuSubButton
                        data-testid="feed"
                        isActive={scope?.type === "feed" &&
                          scope.id === feed.id}
                        aria-current={scope?.type === "feed" &&
                          scope.id === feed.id}
                        onclick={() => scopeTo({ type: "feed", id: feed.id })}
                      >
                        <FeedIcon feedTitle={feed.title} iconUrl={feedIconUrl(feed)} />
                        <span
                          class="truncate {feed.suspended
                            ? 'text-muted-foreground italic'
                            : ''}"
                        >
                          {feed.title}
                        </span>
                        {#if feed.last_error}
                          <span
                            class="text-destructive"
                            title={`Failing since ${feed.last_checked_at ? formatPublished(feed.last_checked_at) : "unknown"}: ${feed.last_error}`}
                          >
                            ⚠
                          </span>
                        {/if}
                      </Sidebar.MenuSubButton>
                      <Sidebar.MenuBadge>{feed.unread_count}</Sidebar.MenuBadge>
                      <div
                        class="flex flex-wrap items-center gap-2 px-2 pb-1 text-xs text-muted-foreground"
                      >
                        <button
                          type="button"
                          class="hover:underline"
                          onclick={() => startEditFeed(feed)}
                        >
                          Rename
                        </button>
                        <label class="sr-only" for={`move-feed-${feed.id}`}
                          >Move {feed.title} to a Group</label
                        >
                        <select
                          id={`move-feed-${feed.id}`}
                          class="h-6 rounded border border-input bg-transparent text-xs"
                          value={feed.group_id}
                          onchange={(event) =>
                            moveFeed(
                              feed,
                              Number((event.target as HTMLSelectElement).value),
                            )}
                        >
                          {#each groups as option (option.id)}
                            <option value={option.id}>{option.name}</option>
                          {/each}
                        </select>
                        <button
                          type="button"
                          class="hover:underline"
                          onclick={() => toggleSuspend(feed)}
                        >
                          {feed.suspended ? "Resume" : "Suspend"}
                        </button>
                        <button
                          type="button"
                          class="hover:underline"
                          onclick={() => removeFeed(feed)}
                        >
                          Delete
                        </button>
                        <span
                          class={feed.last_error ? "text-destructive" : ""}
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
                        </span>
                      </div>
                    {/if}
                  </Sidebar.MenuSubItem>
                {/each}
              </Sidebar.MenuSub>
            {/each}
          </Sidebar.Menu>

          <form
            class="mt-2 flex items-center gap-2 px-2"
            onsubmit={submitNewGroup}
          >
            <Field.FieldLabel for="new-group" class="sr-only"
              >New Group</Field.FieldLabel
            >
            <Input
              id="new-group"
              class="h-7 text-sm"
              placeholder="New Group"
              bind:value={newGroupName}
            />
            <Button
              type="submit"
              size="sm"
              variant="outline"
              disabled={creatingGroup}>Add</Button
            >
          </form>
        </Sidebar.GroupContent>
      </Sidebar.Group>
    </Sidebar.Content>

    <Sidebar.Footer>
      <label class="flex items-center gap-2 px-2 text-sm text-muted-foreground">
        <input
          type="checkbox"
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
      <div class="flex shrink-0 flex-col gap-2 border-b border-border p-3">
        <div class="flex items-center gap-2">
          <Sidebar.Trigger class="-ml-1" />
          <h2
            data-testid="scope"
            class="min-w-0 flex-1 truncate text-base font-medium"
          >
            {scopeTitle}
          </h2>
          {#if filterDefinitions[filter].canMarkAllRead}
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label="Mark all read"
              title="Mark all read"
              onclick={markAllRead}
              disabled={busy || entries.length === 0}
            >
              <CheckCheckIcon />
            </Button>
          {/if}
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Refresh all"
            title="Refresh all"
            onclick={refresh}
            disabled={busy}
          >
            <RefreshCwIcon />
          </Button>
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

      <div class="flex-1 overflow-y-auto">
        {#if notice}
          <p
            data-testid="notice"
            class="px-3 py-2 text-sm text-muted-foreground"
          >
            {notice}
          </p>
        {/if}

        {#if loading}
          <p class="p-3 text-muted-foreground">Loading your Entries…</p>
        {:else if entries.length === 0}
          <p class="p-3 text-muted-foreground">
            {feeds.length === 0
              ? "No Feeds yet. Add one to start reading."
              : filterDefinitions[filter].empty}
          </p>
        {:else}
          <ul class="flex flex-col divide-y divide-border">
            {#each entries as entry, index (entry.id)}
              <EntryRow
                {entry}
                isCurrent={index === selectedIndex}
                iconUrl={iconForEntry(entry)}
                onClick={() => selectEntryAt(index)}
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
        onClose={clearSelection}
        onView={chooseEntryView}
        onToggleRead={toggleReadCurrent}
        onToggleStar={toggleStarCurrent}
        onArchive={archiveCurrent}
      />
    {:else}
      <div
        data-testid="reading-pane-empty"
        class="hidden flex-1 items-center justify-center border-l border-border text-muted-foreground lg:flex"
      >
        Pick an Entry to read it here.
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
