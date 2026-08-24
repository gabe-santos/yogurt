<script lang="ts">
	import { onMount } from 'svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import { ApiError, addFeed, listEntries, listFeeds, logOut, refreshFeeds } from '$lib/api';
	import type { Entry, Feed } from '$lib/api';

	// The reading list is the server's, and this page mutates it as the reader
	// works, so it owns the copy rather than deriving one from a load function.
	let feeds = $state<Feed[]>([]);
	let entries = $state<Entry[]>([]);
	let cursor = $state('');
	let scope = $state<number | undefined>(undefined);
	let loading = $state(true);

	let address = $state('');
	let subscribing = $state(false);
	let subscribeError = $state('');
	let notice = $state('');
	let busy = $state(false);

	const scopedFeed = $derived(feeds.find((feed) => feed.id === scope));

	onMount(async () => {
		try {
			const [subscribed, page] = await Promise.all([listFeeds(), listEntries()]);
			feeds = subscribed;
			entries = page.entries;
			cursor = page.next_cursor;
		} finally {
			loading = false;
		}
	});

	/** reload replaces the list with the first page of the current scope. */
	async function reload() {
		const page = await listEntries({ feed: scope });
		entries = page.entries;
		cursor = page.next_cursor;
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

	async function loadMore() {
		busy = true;
		try {
			const page = await listEntries({ feed: scope, cursor });
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

	const published = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	function publishedAt(entry: Entry): string {
		return published.format(new Date(entry.published_at));
	}
</script>

<div class="mx-auto flex max-w-5xl flex-col gap-6 p-6 sm:flex-row sm:gap-10">
	<aside class="flex w-full shrink-0 flex-col gap-4 sm:w-64">
		<header class="flex items-center justify-between gap-4">
			<h1 class="text-2xl font-semibold">Reader</h1>
			<button
				type="button"
				class="rounded border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
				onclick={signOut}
			>
				Sign out
			</button>
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

		{#if notice}
			<p data-testid="notice" class="text-sm text-neutral-600 dark:text-neutral-400">{notice}</p>
		{/if}

		{#if loading}
			<p class="text-neutral-600 dark:text-neutral-400">Loading your Entries…</p>
		{:else if entries.length === 0}
			<p class="text-neutral-600 dark:text-neutral-400">
				{feeds.length === 0 ? 'No Feeds yet. Add one to start reading.' : 'Nothing to read here yet.'}
			</p>
		{:else}
			<ul class="flex flex-col divide-y divide-neutral-200 dark:divide-neutral-800">
				{#each entries as entry (entry.id)}
					<li data-testid="entry" class="flex flex-col gap-1 py-3">
						<a
							href={entry.url}
							target="_blank"
							rel="noreferrer"
							class="font-medium underline-offset-2 hover:underline"
						>
							{entry.title || entry.url}
						</a>
						<p class="text-xs text-neutral-500 dark:text-neutral-400">
							{entry.feed_title} · {publishedAt(entry)}
						</p>
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
