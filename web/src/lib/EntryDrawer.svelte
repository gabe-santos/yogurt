<script lang="ts">
	import { getArticle, type Article, type Entry } from '$lib/api';
	import { formatPublished } from '$lib/format';
	import { Button } from '$lib/components/ui/button';
	import * as Sheet from '$lib/components/ui/sheet';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import StarIcon from '@lucide/svelte/icons/star';

	interface Props {
		entry: Entry;
		hasPrev: boolean;
		hasNext: boolean;
		busy: boolean;
		onClose: () => void;
		onPrev: () => void;
		onNext: () => void;
		onToggleRead: () => void;
		onToggleStar: () => void;
		onArchive: () => void;
	}

	const {
		entry,
		hasPrev,
		hasNext,
		busy,
		onClose,
		onPrev,
		onNext,
		onToggleRead,
		onToggleStar,
		onArchive
	}: Props = $props();

	// Reader View is requested afresh for each Entry rather than remembered
	// across them: an Entry opens showing what the Feed supplied.
	let readerView = $state(false);
	let article = $state<Article | null>(null);
	let articleError = $state('');
	let articleLoading = $state(false);
	let readerViewEntryId = $state<number | undefined>(undefined);

	$effect(() => {
		if (entry.id === readerViewEntryId) return;
		readerViewEntryId = entry.id;
		readerView = false;
		article = null;
		articleError = '';
		articleLoading = false;
	});

	async function toggleReaderView() {
		readerView = !readerView;
		if (!readerView || article || articleLoading) return;
		const requestedFor = entry.id;
		articleLoading = true;
		articleError = '';
		try {
			const extracted = await getArticle(requestedFor);
			if (requestedFor !== readerViewEntryId) return;
			article = extracted;
		} catch (cause) {
			if (requestedFor !== readerViewEntryId) return;
			articleError = cause instanceof Error ? cause.message : 'Could not extract that Article';
		} finally {
			if (requestedFor === readerViewEntryId) articleLoading = false;
		}
	}
</script>

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
				<Button
					variant={readerView ? 'secondary' : 'outline'}
					size="sm"
					onclick={toggleReaderView}
					disabled={busy}
					data-testid="reader-view-toggle"
				>
					<BookOpenIcon data-icon="inline-start" />
					{readerView ? 'Original' : 'Reader View'}
				</Button>
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

		<div
			data-testid="entry-content"
			class="max-w-none flex-1 overflow-y-auto text-base leading-relaxed break-words text-foreground [&_a]:underline [&_a]:underline-offset-2 [&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground [&_h1]:mt-6 [&_h1]:mb-3 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:mt-5 [&_h2]:mb-2 [&_h2]:text-lg [&_h2]:font-semibold [&_img]:max-w-full [&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-6"
		>
			{#if readerView}
				{#if articleLoading}
					<p class="text-muted-foreground">Extracting the Article…</p>
				{:else if articleError}
					<p class="text-destructive" data-testid="reader-view-error">{articleError}</p>
				{:else if article}
					{@html article.html}
				{/if}
			{:else}
				{@html entry.content}
			{/if}
		</div>
	</Sheet.Content>
</Sheet.Root>
