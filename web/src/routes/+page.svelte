<script lang="ts">
	import { onMount } from 'svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import {
		ApiError,
		addFeed,
		getSettings,
		listEntries,
		listFeeds,
		logOut,
		refreshFeeds,
		setEntryRead,
		setSettings
	} from '$lib/api';
	import type { Entry, Feed } from '$lib/api';
	import EntryDrawer from '$lib/EntryDrawer.svelte';
	import { formatPublished } from '$lib/format';
	import HelpDialog from '$lib/HelpDialog.svelte';
	import { bindings, matches } from '$lib/keys';
	import type { Action } from '$lib/keys';

	// The reading list is the server's, and this page mutates it as the reader
	// works, so it owns the copy rather than deriving one from a load function.
	let feeds = $state<Feed[]>([]);
	let entries = $state<Entry[]>([]);
	let cursor = $state('');
	let scope = $state<number | undefined>(undefined);
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

	const scopedFeed = $derived(feeds.find((feed) => feed.id === scope));
	const openEntry = $derived(openIndex !== undefined ? entries[openIndex] : undefined);

	onMount(async () => {
		try {
			const [subscribed, page, settings] = await Promise.all([
				listFeeds(),
				listEntries(),
				getSettings()
			]);
			feeds = subscribed;
			entries = page.entries;
			cursor = page.next_cursor;
			markOnOpen = settings.mark_on_open;
		} finally {
			loading = false;
		}
	});

	/** reload replaces the list with the first page of the current scope and
	 * filter, clearing keyboard position: the underlying list changed under it. */
	async function reload() {
		const page = await listEntries({ feed: scope, unread: filter === 'unread' });
		entries = page.entries;
		cursor = page.next_cursor;
		currentIndex = undefined;
		openIndex = undefined;
		manuallyUnread = new Set();
	}

	async function subscribe(event: SubmitEvent) {
		event.preventDefault();
		subscribeError = '';
		notice = '';
		subscribing = true;
		try {
			const feed = await addFeed(address);
			address = '';
			feeds = [...feeds, feed].sort((a, b) => a.title.localeCompare(b.title));
			scope = feed.id;
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
			notice =
				failures.length === 0
					? 'Every Feed is up to date.'
					: `${failures.length} Feed${failures.length === 1 ? '' : 's'} could not be read.`;
		} catch (cause) {
			notice = cause instanceof ApiError ? cause.message : 'Could not reach the server';
		} finally {
			busy = false;
		}
	}

	async function scopeTo(feedID: number | undefined) {
		scope = feedID;
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
			const page = await listEntries({ feed: scope, unread: filter === 'unread', cursor });
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

<div class="mx-auto flex max-w-5xl flex-col gap-6 p-6 sm:flex-row sm:gap-10">
	<aside class="flex w-full shrink-0 flex-col gap-4 sm:w-64">
		<header class="flex items-center justify-between gap-4">
			<h1 class="text-2xl font-semibold">Reader</h1>
			<div class="flex items-center gap-2">
				<button
					type="button"
					aria-label="Keyboard shortcuts"
					class="rounded border border-neutral-300 px-2 py-1.5 text-sm hover:bg-neutral-100 dark:border-neutral-700 dark:hover:bg-neutral-800"
					onclick={() => (helpOpen = true)}
				>
					?
				</button>
				<button
					type="button"
					class="rounded border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
					onclick={signOut}
				>
					Sign out
				</button>
			</div>
		</header>

		<form class="flex flex-col gap-2" onsubmit={subscribe}>
			<label class="flex flex-col gap-1 text-sm" for="address">
				Feed or site address
				<input
					id="address"
					name="address"
					type="url"
					required
					placeholder="https://example.com"
					bind:value={address}
					class="rounded border border-neutral-300 px-3 py-2 text-base dark:border-neutral-700 dark:bg-neutral-900"
				/>
			</label>
			<button
				type="submit"
				disabled={subscribing}
				class="rounded bg-neutral-900 px-3 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
			>
				Subscribe
			</button>
			{#if subscribeError}
				<p role="alert" class="text-sm text-red-600 dark:text-red-400">{subscribeError}</p>
			{/if}
		</form>

		<nav class="flex flex-col gap-1" aria-label="Feeds">
			<button
				type="button"
				aria-current={scope === undefined}
				class="rounded px-2 py-1.5 text-left text-sm hover:bg-neutral-100 aria-[current=true]:bg-neutral-100 aria-[current=true]:font-medium dark:hover:bg-neutral-800 dark:aria-[current=true]:bg-neutral-800"
				onclick={() => scopeTo(undefined)}
			>
				All Feeds
			</button>
			{#each feeds as feed (feed.id)}
				<button
					type="button"
					data-testid="feed"
					aria-current={scope === feed.id}
					class="truncate rounded px-2 py-1.5 text-left text-sm hover:bg-neutral-100 aria-[current=true]:bg-neutral-100 aria-[current=true]:font-medium dark:hover:bg-neutral-800 dark:aria-[current=true]:bg-neutral-800"
					onclick={() => scopeTo(feed.id)}
				>
					{feed.title}
				</button>
			{/each}
		</nav>

		<label class="flex items-center gap-2 text-sm text-neutral-600 dark:text-neutral-400">
			<input type="checkbox" checked={markOnOpen} onchange={toggleMarkOnOpen} />
			Mark an Entry read when opened
		</label>
	</aside>

	<main class="flex min-w-0 grow flex-col gap-4">
		<div class="flex items-center justify-between gap-4">
			<h2 data-testid="scope" class="truncate text-lg font-medium">
				{scopedFeed ? scopedFeed.title : 'All Feeds'}
			</h2>
			<button
				type="button"
				class="rounded border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
				onclick={refresh}
				disabled={busy}
			>
				Refresh all
			</button>
		</div>

		<div class="flex gap-1" role="tablist" aria-label="Filter">
			<button
				type="button"
				data-testid="filter-all"
				role="tab"
				aria-selected={filter === 'all'}
				class="rounded px-3 py-1 text-sm hover:bg-neutral-100 aria-[selected=true]:bg-neutral-100 aria-[selected=true]:font-medium dark:hover:bg-neutral-800 dark:aria-[selected=true]:bg-neutral-800"
				onclick={() => setFilter('all')}
			>
				All
			</button>
			<button
				type="button"
				data-testid="filter-unread"
				role="tab"
				aria-selected={filter === 'unread'}
				class="rounded px-3 py-1 text-sm hover:bg-neutral-100 aria-[selected=true]:bg-neutral-100 aria-[selected=true]:font-medium dark:hover:bg-neutral-800 dark:aria-[selected=true]:bg-neutral-800"
				onclick={() => setFilter('unread')}
			>
				Unread
			</button>
		</div>

		{#if notice}
			<p data-testid="notice" class="text-sm text-neutral-600 dark:text-neutral-400">{notice}</p>
		{/if}

		{#if loading}
			<p class="text-neutral-600 dark:text-neutral-400">Loading your Entries…</p>
		{:else if entries.length === 0}
			<p class="text-neutral-600 dark:text-neutral-400">
				{feeds.length === 0
					? 'No Feeds yet. Add one to start reading.'
					: filter === 'unread'
						? 'Nothing unread here.'
						: 'Nothing to read here yet.'}
			</p>
		{:else}
			<ul class="flex flex-col divide-y divide-neutral-200 dark:divide-neutral-800">
				{#each entries as entry, index (entry.id)}
					<li>
						<button
							type="button"
							data-testid="entry"
							aria-current={index === currentIndex}
							class="flex w-full flex-col gap-1 py-3 text-left hover:bg-neutral-50 aria-[current=true]:bg-neutral-100 dark:hover:bg-neutral-900 dark:aria-[current=true]:bg-neutral-800"
							onclick={() => openEntryAt(index)}
						>
							<span
								class="font-medium underline-offset-2 hover:underline"
								class:text-neutral-500={entry.read}
								class:dark:text-neutral-500={entry.read}
							>
								{entry.title || entry.url}
							</span>
							<span class="text-xs text-neutral-500 dark:text-neutral-400">
								{entry.feed_title} · {formatPublished(entry.published_at)}{entry.read ? '' : ' · unread'}
							</span>
						</button>
					</li>
				{/each}
			</ul>

			{#if cursor}
				<button
					type="button"
					class="self-start rounded border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
					onclick={loadMore}
					disabled={busy}
				>
					Load more
				</button>
			{/if}
		{/if}
	</main>
</div>

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
