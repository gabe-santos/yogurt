<script lang="ts">
	import {
		getArticle,
		getOriginal,
		type Article,
		type Entry,
		type EntryView,
		type Original
	} from '$lib/api';
	import { formatPublished } from '$lib/format';
	import { Button } from '$lib/components/ui/button';
	import * as Sheet from '$lib/components/ui/sheet';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import RssIcon from '@lucide/svelte/icons/rss';
	import StarIcon from '@lucide/svelte/icons/star';

	interface Props {
		entry: Entry;
		hasPrev: boolean;
		hasNext: boolean;
		busy: boolean;
		/** view is the reader's remembered choice, owned by the page. */
		view: EntryView;
		onClose: () => void;
		onPrev: () => void;
		onNext: () => void;
		onView: (view: EntryView) => void;
		onToggleRead: () => void;
		onToggleStar: () => void;
		onArchive: () => void;
	}

	const {
		entry,
		hasPrev,
		hasNext,
		busy,
		view,
		onClose,
		onPrev,
		onNext,
		onView,
		onToggleRead,
		onToggleStar,
		onArchive
	}: Props = $props();

	// The Article views are fetched per Entry and per view, on demand: an Entry
	// the reader passes through in the Feed's own text never touches the
	// publisher. The remembered view, by contrast, is the reader's and outlives
	// the Entry, so switching Entries keeps it and only drops what was loaded
	// for the Entry left behind.
	let article = $state<Article | null>(null);
	let original = $state<Original | null>(null);
	let loadError = $state('');
	let loading = $state(false);
	let loadedEntryId = $state<number | undefined>(undefined);

	const views: { id: EntryView; label: string; icon: typeof RssIcon }[] = [
		{ id: 'feed', label: 'From the Feed', icon: RssIcon },
		{ id: 'reader', label: 'Reader View', icon: BookOpenIcon },
		{ id: 'original', label: 'Original View', icon: GlobeIcon }
	];

	$effect(() => {
		if (entry.id === loadedEntryId) return;
		loadedEntryId = entry.id;
		article = null;
		original = null;
		loadError = '';
		loading = false;
		void load();
	});

	function chooseView(next: EntryView) {
		if (next === view) return;
		onView(next);
		void load(next);
	}

	// load fetches whatever the wanted view needs and has not got yet. A
	// response for an Entry the reader has since left is discarded rather than
	// shown under the wrong title.
	async function load(wanted: EntryView = view) {
		if (wanted === 'feed') return;
		if (wanted === 'reader' && article) return;
		if (wanted === 'original' && original) return;

		const requestedFor = entry.id;
		loading = true;
		loadError = '';
		try {
			if (wanted === 'reader') {
				const loaded = await getArticle(requestedFor);
				if (requestedFor !== loadedEntryId) return;
				article = loaded;
			} else {
				const loaded = await getOriginal(requestedFor);
				if (requestedFor !== loadedEntryId) return;
				original = loaded;
			}
		} catch (cause) {
			if (requestedFor !== loadedEntryId) return;
			const fallback =
				wanted === 'reader' ? 'Could not extract that Article' : 'Could not reach that publisher';
			loadError = cause instanceof Error ? cause.message : fallback;
		} finally {
			if (requestedFor === loadedEntryId) loading = false;
		}
	}
</script>

<!-- The way out of a page that refuses to be embedded: the reader's own
     browser, in a tab, keeping their place in the reading list behind it. -->
{#snippet openInNewTab()}
	<Button
		variant="outline"
		size="sm"
		href={entry.url}
		target="_blank"
		rel="noreferrer"
		data-testid="open-in-new-tab"
	>
		<ExternalLinkIcon data-icon="inline-start" />
		Open in a new tab
	</Button>
{/snippet}

<Sheet.Root open onOpenChange={(open) => !open && onClose()}>
	<Sheet.Content data-testid="entry-drawer" side="right" class="w-full gap-4 p-6 sm:max-w-2xl">
		<div class="flex flex-wrap items-center justify-between gap-3 pr-10">
			<div class="flex items-center gap-2">
				<Button
					variant="outline"
					size="icon-sm"
					aria-label="Previous Entry"
					disabled={busy || !hasPrev}
					onclick={onPrev}
				>
					<ArrowLeftIcon />
				</Button>
				<Button
					variant="outline"
					size="icon-sm"
					aria-label="Next Entry"
					disabled={busy || !hasNext}
					onclick={onNext}
				>
					<ArrowRightIcon />
				</Button>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<Button variant={entry.starred ? 'secondary' : 'outline'} size="sm" onclick={onToggleStar} disabled={busy}>
					<StarIcon data-icon="inline-start" />
					{entry.starred ? 'Unstar' : 'Star'}
				</Button>
				{#if !entry.archived}
					<Button variant="outline" size="sm" onclick={onArchive} disabled={busy}>
						<ArchiveIcon data-icon="inline-start" />
						Archive
					</Button>
				{/if}
				<Button variant="outline" size="sm" onclick={onToggleRead} disabled={busy || entry.archived}>
					{entry.read ? 'Mark unread' : 'Mark read'}
				</Button>
			</div>
		</div>

		<Sheet.Header class="gap-1 p-0 text-left">
			<Sheet.Title class="text-xl font-semibold">{entry.title || entry.url}</Sheet.Title>
			<Sheet.Description class="text-sm text-muted-foreground">
				{entry.feed_title} · {formatPublished(entry.published_at)}
			</Sheet.Description>
			<a href={entry.url} target="_blank" rel="noreferrer" class="text-sm underline-offset-2 hover:underline">
				Open the original
			</a>
		</Sheet.Header>

		<div class="flex flex-wrap items-center gap-2" role="group" aria-label="View">
			{#each views as choice (choice.id)}
				<Button
					variant={view === choice.id ? 'secondary' : 'outline'}
					size="sm"
					onclick={() => chooseView(choice.id)}
					disabled={busy}
					aria-pressed={view === choice.id}
					data-testid="view-{choice.id}"
				>
					<choice.icon data-icon="inline-start" />
					{choice.label}
				</Button>
			{/each}
		</div>

		<div
			data-testid="entry-content"
			class="max-w-none flex-1 overflow-y-auto text-base leading-relaxed break-words text-foreground [&_a]:underline [&_a]:underline-offset-2 [&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground [&_h1]:mt-6 [&_h1]:mb-3 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:mt-5 [&_h2]:mb-2 [&_h2]:text-lg [&_h2]:font-semibold [&_img]:max-w-full [&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-6"
		>
			{#if loading}
				<p class="text-muted-foreground" data-testid="view-loading">
					{view === 'reader' ? 'Extracting the Article…' : 'Asking the publisher…'}
				</p>
			{:else if loadError}
				<div class="flex flex-col items-start gap-3">
					<p class="text-destructive" data-testid="view-error">{loadError}</p>
					{#if view === 'original'}
						{@render openInNewTab()}
					{/if}
				</div>
			{:else if view === 'reader'}
				{#if article}
					{@html article.html}
				{/if}
			{:else if view === 'original'}
				{#if original?.embeddable}
					<!-- The publisher's live page, in an opaque origin: no
					     allow-same-origin, so the frame reaches neither this app's
					     cookies and session nor its DOM, and no referrer names the
					     Entry it came from. A link the reader follows out of the
					     frame escapes the sandbox on purpose — a tab inherited into
					     an opaque origin would be a broken browser, not a safer
					     one — while the embedded document itself stays isolated. -->
					<iframe
						data-testid="original-view"
						title={entry.title || entry.url}
						src={original.url}
						sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox allow-forms"
						referrerpolicy="no-referrer"
						class="h-full min-h-96 w-full rounded-md border border-border bg-background"
					></iframe>
				{:else if original}
					<div class="flex flex-col items-start gap-3" data-testid="original-view-forbidden">
						<p class="text-muted-foreground">
							This publisher does not allow their page to be shown inside another site. Reader View
							still works, or the page can be opened in a new tab.
						</p>
						{@render openInNewTab()}
					</div>
				{/if}
			{:else}
				{@html entry.content}
			{/if}
		</div>
	</Sheet.Content>
</Sheet.Root>
