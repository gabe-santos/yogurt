<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import * as Alert from '$lib/components/ui/alert';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import * as Tabs from '$lib/components/ui/tabs';
	import CircleHelpIcon from '@lucide/svelte/icons/circle-help';
	import { onMount } from 'svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import {
		ApiError,
		addFeed,
		createGroup,
		deleteFeed,
		deleteGroup,
		getSettings,
		listEntries,
		listFeeds,
		listGroups,
		logOut,
		refreshFeeds,
		renameGroup,
		setEntryRead,
		setSettings,
		updateFeed
	} from '$lib/api';
	import type { Entry, Feed, Group } from '$lib/api';
	import EntryDrawer from '$lib/EntryDrawer.svelte';
	import { formatPublished } from '$lib/format';
	import HelpDialog from '$lib/HelpDialog.svelte';
	import { bindings, matches } from '$lib/keys';
	import type { Action } from '$lib/keys';

	/** Scope is what the reading list is narrowed to: a single Feed, a single
	 * Group, or (when undefined) every Feed. */
	type Scope = { type: 'feed'; id: number } | { type: 'group'; id: number };

	// The reading list is the server's, and this page mutates it as the reader
	// works, so it owns the copy rather than deriving one from a load function.
	let feeds = $state<Feed[]>([]);
	let groups = $state<Group[]>([]);
	let entries = $state<Entry[]>([]);
	let cursor = $state('');
	let scope = $state<Scope | undefined>(undefined);
	let filter = $state<'all' | 'unread'>('all');
	let loading = $state(true);

	// The current Entry is the keyboard's notion of position in the list,
	// independent of whether the reading drawer is open. Opening the drawer
	// follows it; j/k move it whether the drawer is open or not.
	let currentIndex = $state<number | undefined>(undefined);
	let openIndex = $state<number | undefined>(undefined);
	let helpOpen = $state(false);
	let markOnOpen = $state(true);
	// Entries the reader has declared unread by hand this session: mark-on-open
	// must never re-mark them Read just because j/k passed back through them.
	let manuallyUnread = $state<Set<number>>(new Set());

	let address = $state('');
	let subscribing = $state(false);
	let subscribeError = $state('');
	let notice = $state('');
	let busy = $state(false);

	let newGroupName = $state('');
	let creatingGroup = $state(false);
	let editingGroup = $state<number | undefined>(undefined);
	let groupNameDraft = $state('');
	let editingFeed = $state<number | undefined>(undefined);
	let feedTitleDraft = $state('');

	const scopedFeed = $derived.by(() => {
		const current = scope;
		return current && current.type === 'feed' ? feeds.find((f) => f.id === current.id) : undefined;
	});
	const scopedGroup = $derived.by(() => {
		const current = scope;
		return current && current.type === 'group' ? groups.find((g) => g.id === current.id) : undefined;
	});
	const scopeTitle = $derived(scopedFeed?.title ?? scopedGroup?.name ?? 'All Feeds');
	const openEntry = $derived(openIndex !== undefined ? entries[openIndex] : undefined);
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
		try {
			const [subscribed, page, settings, subscribedGroups] = await Promise.all([
				listFeeds(),
				listEntries(),
				getSettings(),
				listGroups()
			]);
			feeds = subscribed;
			entries = page.entries;
			cursor = page.next_cursor;
			markOnOpen = settings.mark_on_open;
			groups = subscribedGroups;
		} finally {
			loading = false;
		}
	});

	/** scopeQuery maps the current Scope onto listEntries' feed/group options,
	 * so reload and loadMore cannot drift on how a Scope becomes a query. */
	function scopeQuery(current: Scope | undefined) {
		return {
			feed: current?.type === 'feed' ? current.id : undefined,
			group: current?.type === 'group' ? current.id : undefined
		};
	}

	/** reportError shows an ApiError's own message, or a generic one for
	 * anything else (a network failure, a body the server never sent). */
	function reportError(cause: unknown) {
		notice = cause instanceof ApiError ? cause.message : 'Could not reach the server';
	}

	/** reload replaces the list with the first page of the current scope and
	 * filter, clearing keyboard position: the underlying list changed under it. */
	async function reload() {
		const page = await listEntries({ ...scopeQuery(scope), unread: filter === 'unread' });
		entries = page.entries;
		cursor = page.next_cursor;
		currentIndex = undefined;
		openIndex = undefined;
		manuallyUnread = new Set();
	}

	/** refreshCounts re-reads Feeds and Groups so their unread counts stay
	 * correct after an Entry's Read state, or the collection itself, changes. */
	async function refreshCounts() {
		const [nextFeeds, nextGroups] = await Promise.all([listFeeds(), listGroups()]);
		feeds = nextFeeds;
		groups = nextGroups;
	}

	async function subscribe(event: SubmitEvent) {
		event.preventDefault();
		subscribeError = '';
		notice = '';
		subscribing = true;
		try {
			const feed = await addFeed(address);
			address = '';
			await refreshCounts();
			scope = { type: 'feed', id: feed.id };
			await reload();
			notice = `Subscribed to ${feed.title}.`;
		} catch (cause) {
			subscribeError = cause instanceof ApiError ? cause.message : 'Could not reach the server';
		} finally {
			subscribing = false;
		}
	}

	async function refresh() {
		notice = '';
		busy = true;
		try {
			const failures = await refreshFeeds();
			await reload();
			await refreshCounts();
			notice =
				failures.length === 0
					? 'Every Feed is up to date.'
					: `${failures.length} Feed${failures.length === 1 ? '' : 's'} could not be read.`;
		} catch (cause) {
			reportError(cause);
		} finally {
			busy = false;
		}
	}

	async function scopeTo(next: Scope | undefined) {
		scope = next;
		busy = true;
		try {
			await reload();
		} finally {
			busy = false;
		}
	}

	async function setFilter(next: 'all' | 'unread') {
		if (filter === next) {
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
		busy = true;
		try {
			const page = await listEntries({
				...scopeQuery(scope),
				unread: filter === 'unread',
				cursor
			});
			entries = [...entries, ...page.entries];
			cursor = page.next_cursor;
		} finally {
			busy = false;
		}
	}

	async function signOut() {
		await logOut();
		await invalidateAll();
		await goto('/login');
	}

	/** applyRead sets an Entry's Read state optimistically, correcting the list
	 * if the server refuses it. A manual change is remembered so that j/k
	 * navigating back to an Entry the reader declared unread by hand does not
	 * let mark-on-open silently override it again. */
	async function applyRead(index: number, read: boolean, manual = false) {
		const previous = entries[index];
		entries[index] = { ...previous, read };
		if (manual && !read) {
			manuallyUnread.add(previous.id);
		} else if (manual) {
			manuallyUnread.delete(previous.id);
		}
		try {
			const stored = await setEntryRead(previous.id, read);
			entries[index] = stored;
			await refreshCounts();
		} catch {
			entries[index] = previous;
			notice = 'Could not update that Entry.';
		}
	}

	function moveCurrent(delta: number) {
		if (entries.length === 0) {
			return;
		}
		const base = currentIndex ?? (delta > 0 ? -1 : entries.length);
		currentIndex = Math.min(Math.max(base + delta, 0), entries.length - 1);
		if (openIndex !== undefined) {
			openIndex = currentIndex;
			maybeMarkOnOpen(currentIndex);
		}
	}

	function openCurrent() {
		if (entries.length === 0) {
			return;
		}
		if (currentIndex === undefined) {
			currentIndex = 0;
		}
		openIndex = currentIndex;
		maybeMarkOnOpen(openIndex);
	}

	function openEntryAt(index: number) {
		currentIndex = index;
		openIndex = index;
		maybeMarkOnOpen(index);
	}

	function closeDrawer() {
		openIndex = undefined;
	}

	function maybeMarkOnOpen(index: number) {
		const entry = entries[index];
		if (markOnOpen && !entry.read && !manuallyUnread.has(entry.id)) {
			void applyRead(index, true);
		}
	}

	function toggleReadCurrent() {
		const index = openIndex ?? currentIndex;
		if (index === undefined) {
			return;
		}
		void applyRead(index, !entries[index].read, true);
	}

	async function toggleMarkOnOpen() {
		const next = !markOnOpen;
		markOnOpen = next;
		try {
			const settings = await setSettings({ mark_on_open: next });
			markOnOpen = settings.mark_on_open;
		} catch {
			markOnOpen = !next;
			notice = 'Could not update your settings.';
		}
	}

	/** sortGroups matches the server's own order (default first, then by
	 * name), so a create or rename never leaves the sidebar out of step with
	 * what the next listGroups() would return. */
	function sortGroups(list: Group[]): Group[] {
		return [...list].sort((a, b) => {
			if (a.is_default !== b.is_default) {
				return a.is_default ? -1 : 1;
			}
			return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
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
			newGroupName = '';
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
		if (!confirm(`Delete the Group "${group.name}"? Its Feeds move to the default Group.`)) {
			return;
		}
		try {
			await deleteGroup(group.id);
			if (scope?.type === 'group' && scope.id === group.id) {
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
			if (scope?.type === 'feed' && scope.id === feed.id) {
				scope = undefined;
			}
			await refreshCounts();
			await reload();
		} catch (cause) {
			reportError(cause);
		}
	}

	function closeCurrent() {
		if (helpOpen) {
			helpOpen = false;
		} else if (openIndex !== undefined) {
			closeDrawer();
		}
	}

	// One dispatch table, keyed by the same action ids the binding table names,
	// so a new shortcut is a row in keys.ts plus one entry here rather than a
	// second switch that can drift from the first.
	const actions: Record<Action, () => void> = {
		next: () => moveCurrent(1),
		prev: () => moveCurrent(-1),
		open: openCurrent,
		close: closeCurrent,
		toggleRead: toggleReadCurrent,
		help: () => (helpOpen = true)
	};

	function isTypingTarget(target: EventTarget | null): boolean {
		if (!(target instanceof HTMLElement)) {
			return false;
		}
		return target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable;
	}

	/** isActivatable reports a focused control whose own Enter activation
	 * (a native click) must win over the global Enter binding, so tabbing to
	 * "Sign out" or the drawer's "Close" button and pressing Enter does not
	 * open the current Entry instead. */
	function isActivatable(target: EventTarget | null): boolean {
		return target instanceof HTMLElement && (target.tagName === 'BUTTON' || target.tagName === 'A');
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' && isActivatable(event.target)) {
			return;
		}
		if (isTypingTarget(event.target)) {
			return;
		}
		for (const binding of bindings) {
			if (!matches(binding, event)) {
				continue;
			}
			if (helpOpen && binding.action !== 'close') {
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
						aria-label="Keyboard shortcuts"
						onclick={() => (helpOpen = true)}
					>
						<CircleHelpIcon />
					</Button>
					<Button variant="outline" size="sm" onclick={signOut}>Sign out</Button>
				</div>
			</div>
		</Sidebar.Header>

		<Sidebar.Content>
			<Sidebar.Group>
				<Sidebar.GroupContent>
					<form class="flex flex-col gap-2" onsubmit={subscribe}>
						<Field.FieldGroup>
							<Field.Field>
								<Field.FieldLabel for="address">Feed or site address</Field.FieldLabel>
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
											if (event.key === 'Enter') saveGroupName(group.id);
											if (event.key === 'Escape') editingGroup = undefined;
										}}
									/>
								{:else}
									<Sidebar.MenuButton
										data-testid="group"
										isActive={scope?.type === 'group' && scope.id === group.id}
										aria-current={scope?.type === 'group' && scope.id === group.id}
										onclick={() => scopeTo({ type: 'group', id: group.id })}
									>
										<span class="truncate font-medium">{group.name}</span>
									</Sidebar.MenuButton>
									<Sidebar.MenuBadge>{group.unread_count}</Sidebar.MenuBadge>
								{/if}
							</Sidebar.MenuItem>
							{#if editingGroup !== group.id}
								<div class="flex items-center gap-2 px-2 pb-1 text-xs text-muted-foreground">
									<button type="button" class="hover:underline" onclick={() => startEditGroup(group)}>
										Rename
									</button>
									{#if !group.is_default}
										<button type="button" class="hover:underline" onclick={() => removeGroup(group)}>
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
													if (event.key === 'Enter') saveFeedTitle(feed.id);
													if (event.key === 'Escape') editingFeed = undefined;
												}}
											/>
										{:else}
											<Sidebar.MenuSubButton
												data-testid="feed"
												isActive={scope?.type === 'feed' && scope.id === feed.id}
												aria-current={scope?.type === 'feed' && scope.id === feed.id}
												onclick={() => scopeTo({ type: 'feed', id: feed.id })}
											>
												<span class="truncate {feed.suspended ? 'text-muted-foreground italic' : ''}">
													{feed.title}
												</span>
											</Sidebar.MenuSubButton>
											<Sidebar.MenuBadge>{feed.unread_count}</Sidebar.MenuBadge>
											<div class="flex flex-wrap items-center gap-2 px-2 pb-1 text-xs text-muted-foreground">
												<button type="button" class="hover:underline" onclick={() => startEditFeed(feed)}>
													Rename
												</button>
												<label class="sr-only" for={`move-feed-${feed.id}`}>Move {feed.title} to a Group</label>
												<select
													id={`move-feed-${feed.id}`}
													class="h-6 rounded border border-input bg-transparent text-xs"
													value={feed.group_id}
													onchange={(event) =>
														moveFeed(feed, Number((event.target as HTMLSelectElement).value))}
												>
													{#each groups as option (option.id)}
														<option value={option.id}>{option.name}</option>
													{/each}
												</select>
												<button type="button" class="hover:underline" onclick={() => toggleSuspend(feed)}>
													{feed.suspended ? 'Resume' : 'Suspend'}
												</button>
												<button type="button" class="hover:underline" onclick={() => removeFeed(feed)}>
													Delete
												</button>
											</div>
										{/if}
									</Sidebar.MenuSubItem>
								{/each}
							</Sidebar.MenuSub>
						{/each}
					</Sidebar.Menu>

					<form class="mt-2 flex items-center gap-2 px-2" onsubmit={submitNewGroup}>
						<Field.FieldLabel for="new-group" class="sr-only">New Group</Field.FieldLabel>
						<Input id="new-group" class="h-7 text-sm" placeholder="New Group" bind:value={newGroupName} />
						<Button type="submit" size="sm" variant="outline" disabled={creatingGroup}>Add</Button>
					</form>
				</Sidebar.GroupContent>
			</Sidebar.Group>
		</Sidebar.Content>

		<Sidebar.Footer>
			<label class="flex items-center gap-2 px-2 text-sm text-muted-foreground">
				<input type="checkbox" checked={markOnOpen} onchange={toggleMarkOnOpen} />
				Mark an Entry read when opened
			</label>
		</Sidebar.Footer>
	</Sidebar.Root>

	<Sidebar.Inset>
		<div class="mx-auto flex w-full max-w-3xl flex-col gap-4 p-6">
			<div class="flex items-center gap-2">
				<Sidebar.Trigger class="-ml-1" />
				<div class="flex flex-1 items-center justify-between gap-4">
					<h2 data-testid="scope" class="truncate text-lg font-medium">
						{scopeTitle}
					</h2>
					<Button variant="outline" size="sm" onclick={refresh} disabled={busy}>Refresh all</Button>
				</div>
			</div>

			<Tabs.Root value={filter} onValueChange={(value) => setFilter(value as 'all' | 'unread')}>
				<Tabs.List aria-label="Filter">
					<Tabs.Trigger value="all" data-testid="filter-all">All</Tabs.Trigger>
					<Tabs.Trigger value="unread" data-testid="filter-unread">Unread</Tabs.Trigger>
				</Tabs.List>
			</Tabs.Root>

			{#if notice}
				<p data-testid="notice" class="text-sm text-muted-foreground">{notice}</p>
			{/if}

			{#if loading}
				<p class="text-muted-foreground">Loading your Entries…</p>
			{:else if entries.length === 0}
				<p class="text-muted-foreground">
					{feeds.length === 0
						? 'No Feeds yet. Add one to start reading.'
						: filter === 'unread'
							? 'Nothing unread here.'
							: 'Nothing to read here yet.'}
				</p>
			{:else}
				<ul class="flex flex-col divide-y divide-border">
					{#each entries as entry, index (entry.id)}
						<li>
							<button
								type="button"
								data-testid="entry"
								aria-current={index === currentIndex}
								class="flex w-full flex-col gap-1 py-3 text-left hover:bg-accent/50 aria-[current=true]:bg-accent"
								onclick={() => openEntryAt(index)}
							>
								<span
									class="font-medium underline-offset-2 hover:underline"
									class:text-muted-foreground={entry.read}
								>
									{entry.title || entry.url}
								</span>
								<span class="text-xs text-muted-foreground">
									{entry.feed_title} · {formatPublished(entry.published_at)}{entry.read ? '' : ' · unread'}
								</span>
							</button>
						</li>
					{/each}
				</ul>

				{#if cursor}
					<Button variant="outline" size="sm" class="self-start" onclick={loadMore} disabled={busy}>
						Load more
					</Button>
				{/if}
			{/if}
		</div>
	</Sidebar.Inset>
</Sidebar.Provider>

{#if openEntry}
	<EntryDrawer
		entry={openEntry}
		hasPrev={(currentIndex ?? 0) > 0}
		hasNext={(currentIndex ?? 0) < entries.length - 1}
		onClose={closeDrawer}
		onPrev={() => moveCurrent(-1)}
		onNext={() => moveCurrent(1)}
		onToggleRead={toggleReadCurrent}
	/>
{/if}

{#if helpOpen}
	<HelpDialog onClose={() => (helpOpen = false)} />
{/if}
