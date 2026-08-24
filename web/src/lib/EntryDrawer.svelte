<script lang="ts">
	import type { Entry } from '$lib/api';
	import { formatPublished } from '$lib/format';

	interface Props {
		entry: Entry;
		hasPrev: boolean;
		hasNext: boolean;
		onClose: () => void;
		onPrev: () => void;
		onNext: () => void;
		onToggleRead: () => void;
	}

	const { entry, hasPrev, hasNext, onClose, onPrev, onNext, onToggleRead }: Props = $props();
</script>

<div
	data-testid="entry-drawer"
	role="dialog"
	aria-label={entry.title || entry.url}
	class="fixed inset-0 z-20 flex justify-end bg-black/30"
>
	<div class="flex h-full w-full max-w-2xl flex-col gap-4 overflow-y-auto bg-white p-6 dark:bg-neutral-950">
		<div class="flex items-center justify-between gap-4">
			<div class="flex items-center gap-2">
				<button
					type="button"
					aria-label="Previous Entry"
					disabled={!hasPrev}
					onclick={onPrev}
					class="rounded border border-neutral-300 px-2 py-1 text-sm disabled:opacity-40 dark:border-neutral-700"
				>
					&larr;
				</button>
				<button
					type="button"
					aria-label="Next Entry"
					disabled={!hasNext}
					onclick={onNext}
					class="rounded border border-neutral-300 px-2 py-1 text-sm disabled:opacity-40 dark:border-neutral-700"
				>
					&rarr;
				</button>
			</div>
			<div class="flex items-center gap-2">
				<button
					type="button"
					onclick={onToggleRead}
					class="rounded border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700"
				>
					{entry.read ? 'Mark unread' : 'Mark read'}
				</button>
				<button
					type="button"
					aria-label="Close"
					onclick={onClose}
					class="rounded border border-neutral-300 px-3 py-1.5 text-sm dark:border-neutral-700"
				>
					Close
				</button>
			</div>
		</div>

		<header class="flex flex-col gap-1">
			<h2 class="text-xl font-semibold">{entry.title || entry.url}</h2>
			<p class="text-sm text-neutral-500 dark:text-neutral-400">
				{entry.feed_title} · {formatPublished(entry.published_at)}
			</p>
			<a href={entry.url} target="_blank" rel="noreferrer" class="text-sm underline-offset-2 hover:underline">
				Open the original
			</a>
		</header>

		<div
			data-testid="entry-content"
			class="max-w-none text-base leading-relaxed break-words text-neutral-800 dark:text-neutral-200 [&_a]:underline [&_a]:underline-offset-2 [&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-neutral-300 [&_blockquote]:pl-3 [&_blockquote]:text-neutral-600 [&_h1]:mt-6 [&_h1]:mb-3 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:mt-5 [&_h2]:mb-2 [&_h2]:text-lg [&_h2]:font-semibold [&_img]:max-w-full [&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-6 dark:[&_blockquote]:border-neutral-700 dark:[&_blockquote]:text-neutral-400"
		>
			{@html entry.content}
		</div>
	</div>
</div>
