<script lang="ts">
	import { untrack } from 'svelte';
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
	import * as Tooltip from '$lib/components/ui/tooltip';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import BookOpenIcon from '@lucide/svelte/icons/book-open';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import MailIcon from '@lucide/svelte/icons/mail';
	import MailOpenIcon from '@lucide/svelte/icons/mail-open';
	import RssIcon from '@lucide/svelte/icons/rss';
	import StarIcon from '@lucide/svelte/icons/star';

	interface Props {
		entry: Entry;
		busy: boolean;
		/** view is the reader's remembered choice, owned by the page. */
		view: EntryView;
		/** onClose backs out of the overlay the pane becomes on a narrow
		 * screen. On a wide one the pane is furniture and nothing calls it. */
		onClose: () => void;
		onView: (view: EntryView) => void;
		onToggleRead: () => void;
		onToggleStar: () => void;
		onArchive: () => void;
	}

	const { entry, busy, view, onClose, onView, onToggleRead, onToggleStar, onArchive }: Props =
		$props();

	// The Article views are fetched per Entry and per view, on demand: an Entry
	// the reader passes through in the Feed's own text never touches the
	// publisher. The remembered view, by contrast, is the reader's and outlives
	// the Entry, so switching Entries keeps it and only drops what was loaded
	// for the Entry left behind.
	let article = $state<Article | null>(null);
	let original = $state<Original | null>(null);
	let loadError = $state('');
	let loading = $state(false);
	let loadedEntryId: number | undefined;
	// Selecting an Entry is now a keystroke rather than a deliberate open, so a
	// reader holding j would otherwise fire one publisher request per Entry they
	// skim past. Nothing is fetched until the selection has settled.
	const fetchDelay = 250;
	let fetchTimer: ReturnType<typeof setTimeout> | undefined;

	let scroller = $state<HTMLElement | null>(null);
	let titleAnchor = $state<HTMLElement | null>(null);
	// titleScrolledAway drives the sticky bar's copy of the title: the title
	// itself lives in the scrolling body, so once it leaves the pane the bar has
	// to say what is being read.
	let titleScrolledAway = $state(false);

	// Cookies ignore ports. A publisher on Reader's own host could therefore
	// receive the session even when its URL has a different origin; never frame
	// one. External publishers keep their own origin so their cookies, storage,
	// and JavaScript applications continue to work.
	const originalSharesAppHost = $derived(
		original !== null && new URL(original.url).hostname === window.location.hostname
	);

	const views: { id: EntryView; label: string; icon: typeof RssIcon }[] = [
		{ id: 'feed', label: 'From the Feed', icon: RssIcon },
		{ id: 'reader', label: 'Reader View', icon: BookOpenIcon },
		{ id: 'original', label: 'Original View', icon: GlobeIcon }
	];

	// One effect owns everything that must happen when the Entry or the view
	// changes, so a fetch can never outlive the selection that asked for it: the
	// cleanup below cancels a scheduled fetch before the next run schedules its
	// own, which is what makes the delay a debounce rather than a stagger.
	$effect(() => {
		const wantedEntry = entry.id;
		const wantedView = view;

		untrack(() => {
			if (wantedEntry !== loadedEntryId) {
				loadedEntryId = wantedEntry;
				article = null;
				original = null;
				loadError = '';
				loading = false;
				titleScrolledAway = false;
				scroller?.scrollTo({ top: 0 });
			}

			if (wantedView === 'feed') return;
			if (wantedView === 'reader' && article) return;
			if (wantedView === 'original' && original) return;
			fetchTimer = setTimeout(() => void load(wantedEntry, wantedView), fetchDelay);
		});

		return () => clearTimeout(fetchTimer);
	});

	// The title is watched against the pane's own scroll box rather than the
	// viewport, because on a wide screen the pane scrolls and the page does not.
	$effect(() => {
		const anchor = titleAnchor;
		const root = scroller;
		if (!anchor || !root) return;
		const observer = new IntersectionObserver(
			([seen]) => {
				titleScrolledAway = !seen.isIntersecting;
			},
			{ root, threshold: 0 }
		);
		observer.observe(anchor);
		return () => observer.disconnect();
	});

	// load fetches what the wanted view needs. A response for an Entry the
	// reader has since left is discarded rather than shown under the wrong
	// title.
	async function load(requestedFor: number, wanted: EntryView) {
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

{#snippet control(
	label: string,
	icon: typeof RssIcon,
	onclick: () => void,
	pressed: boolean,
	disabled: boolean,
	testid: string
)}
	<Tooltip.Root>
		<Tooltip.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					variant={pressed ? 'secondary' : 'ghost'}
					size="icon-sm"
					aria-label={label}
					aria-pressed={pressed}
					data-testid={testid}
					{disabled}
					{onclick}
				>
					{@const Icon = icon}
					<Icon />
				</Button>
			{/snippet}
		</Tooltip.Trigger>
		<Tooltip.Content>{label}</Tooltip.Content>
	</Tooltip.Root>
{/snippet}

<!-- One component, two containers: a column of the layout once there is room
     for three, and a full-bleed overlay over the Entry List before that. The
     page marks what is behind it inert, which is what keeps the tab order
     inside the overlay without a Sheet to do it for us. -->
<section
	data-testid="reading-pane"
	aria-label="Reading Pane"
	class="absolute inset-0 z-30 flex flex-col bg-background lg:static lg:z-auto lg:flex-1 lg:border-l lg:border-border"
>
	<Tooltip.Provider delayDuration={400}>
		<header
			class="flex h-12 shrink-0 items-center gap-2 border-b border-border bg-background/95 px-2 backdrop-blur"
		>
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							variant="ghost"
							size="icon-sm"
							class="lg:hidden"
							aria-label="Back to the Entry List"
							data-testid="reading-pane-back"
							onclick={onClose}
						>
							<ArrowLeftIcon />
						</Button>
					{/snippet}
				</Tooltip.Trigger>
				<Tooltip.Content>Back to the Entry List</Tooltip.Content>
			</Tooltip.Root>

			<p
				data-testid="reading-pane-title"
				class="min-w-0 flex-1 truncate text-sm font-medium transition-opacity duration-150"
				class:opacity-0={!titleScrolledAway}
				aria-hidden={!titleScrolledAway}
			>
				{entry.title || entry.url}
			</p>

			<div class="flex shrink-0 items-center gap-0.5" role="group" aria-label="View">
				{#each views as choice (choice.id)}
					{@render control(
						choice.label,
						choice.icon,
						() => choice.id !== view && onView(choice.id),
						view === choice.id,
						busy,
						`view-${choice.id}`
					)}
				{/each}
			</div>

			<div class="mx-1 h-5 w-px shrink-0 bg-border" aria-hidden="true"></div>

			<div class="flex shrink-0 items-center gap-0.5">
				{@render control(
					entry.starred ? 'Unstar' : 'Star',
					StarIcon,
					onToggleStar,
					entry.starred,
					busy,
					'entry-star'
				)}
				{#if !entry.archived}
					{@render control('Archive', ArchiveIcon, onArchive, false, busy, 'entry-archive')}
				{/if}
				{@render control(
					entry.read ? 'Mark unread' : 'Mark read',
					entry.read ? MailIcon : MailOpenIcon,
					onToggleRead,
					false,
					busy || entry.archived,
					'entry-read'
				)}
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon-sm"
								href={entry.url}
								target="_blank"
								rel="noreferrer"
								aria-label="Open the original in a new tab"
								data-testid="open-original"
							>
								<ExternalLinkIcon />
							</Button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Content>Open the original in a new tab</Tooltip.Content>
				</Tooltip.Root>
			</div>
		</header>
	</Tooltip.Provider>

	<div bind:this={scroller} data-testid="entry-content" class="flex-1 overflow-y-auto">
		<!-- Reader View and Feed View are our own markup and are held to a
		     readable measure. Original View is the publisher's layout and gets the
		     whole pane, or a responsive site renders its phone design in a
		     desktop-sized column. -->
		<div
			class={view === 'original'
				? 'flex h-full flex-col gap-4 p-4'
				: 'mx-auto flex max-w-2xl flex-col gap-4 px-6 py-6'}
		>
			<div bind:this={titleAnchor} class="flex flex-col gap-1">
				<h1 class="text-2xl font-semibold break-words">{entry.title || entry.url}</h1>
				<p class="text-sm text-muted-foreground">
					{entry.feed_title} · {formatPublished(entry.published_at)}
				</p>
			</div>

			<div
				class="max-w-none flex-1 text-base leading-relaxed break-words text-foreground [&_a]:underline [&_a]:underline-offset-2 [&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground [&_h1]:mt-6 [&_h1]:mb-3 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:mt-5 [&_h2]:mb-2 [&_h2]:text-lg [&_h2]:font-semibold [&_img]:max-w-full [&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-6"
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
					{#if original?.embeddable && !originalSharesAppHost}
						<!-- allow-same-origin preserves the publisher's own cookies and
						     storage. Because same-host URLs are refused below, the
						     browser's same-origin policy still keeps Reader's session and
						     DOM inaccessible; no referrer names the Entry it came from. -->
						<iframe
							data-testid="original-view"
							title={entry.title || entry.url}
							src={original.url}
							sandbox="allow-same-origin allow-scripts allow-popups allow-popups-to-escape-sandbox allow-forms"
							referrerpolicy="no-referrer"
							class="h-full min-h-96 w-full rounded-md border border-border bg-background"
						></iframe>
					{:else if original && originalSharesAppHost}
						<div class="flex flex-col items-start gap-3" data-testid="original-view-unsafe">
							<p class="text-muted-foreground">
								This address shares Reader's host, so embedding it could expose your session. Open it
								in a new tab instead.
							</p>
							{@render openInNewTab()}
						</div>
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
		</div>
	</div>
</section>
