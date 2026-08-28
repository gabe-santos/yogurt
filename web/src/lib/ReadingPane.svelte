<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fly } from 'svelte/transition';
	import { expoOut } from 'svelte/easing';
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
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import * as Tabs from '$lib/components/ui/tabs';
	import FeedIcon from '$lib/FeedIcon.svelte';
	import IconSwap from '$lib/IconSwap.svelte';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import ArchiveRestoreIcon from '@lucide/svelte/icons/archive-restore';
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
		/** iconUrl is the Entry's Feed Icon, so the pane's own metadata line
		 * names its Feed the same way the Entry List row did. */
		iconUrl?: string;
		/** overlay is true while the pane covers the Entry List instead of
		 * standing beside it. The page owns the answer because it owns the box
		 * both panes are measured against; the CSS here asks the same question
		 * with `@3xl`. */
		overlay: boolean;
		/** onClose backs out of the overlay the pane becomes on a narrow
		 * screen. On a wide one the pane is furniture and nothing calls it. */
		onClose: () => void;
		onView: (view: EntryView) => void;
		onToggleRead: () => void;
		onToggleStar: () => void;
		onToggleArchive: () => void;
	}

	const {
		entry,
		busy,
		view,
		iconUrl,
		overlay,
		onClose,
		onView,
		onToggleRead,
		onToggleStar,
		onToggleArchive
	}: Props = $props();

	// Only the overlay has somewhere to arrive from and leave to. The enter is
	// CSS gated by `@max-3xl:motion-safe:`; the exit needs the same two
	// answers in JavaScript, which is what `overlay` carries in.
	// Exits run 250ms against the entrance's 300ms — the reader should not wait
	// on the way out — and travel the same 32px the entrance already uses,
	// along expoOut: the JS equivalent of the entrance's own
	// cubic-bezier(0.16, 1, 0.3, 1). Sampled across the curve expoOut deviates
	// by 0.0056 (0.18px over 32px); cubicOut, the obvious default, deviates by
	// 0.264 (8.5px) and would visibly change the entrance this exit mirrors.
	// The third column never transitions; only the overlay leaves. Reduced
	// motion still cross-fades — gentler, not absent — just without the
	// horizontal travel, and faster than the full exit.
	const exit = $derived(
		!overlay
			? { duration: 0 }
			: prefersReducedMotion.current
				? { x: 0, duration: 100 }
				: { x: 32, duration: 250, easing: expoOut }
	);

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
	// backButton is where focus lands when this component mounts as the
	// narrow-screen overlay: opening it makes the Entry List behind it inert,
	// which blurs whatever was focused there, so something inside the overlay
	// has to claim focus or it is lost to the document body entirely.
	let backButton = $state<HTMLButtonElement | null>(null);

	// This component mounts once per "opening" of the overlay — clearSelection
	// unmounts it, so a later selection is a fresh mount — which is exactly
	// when focus needs to move in. Selecting a different Entry while the pane
	// stays open must not keep re-stealing focus, so this never re-runs for
	// that; onMount alone gives the once-per-open timing for free.
	onMount(() => {
		if (overlay) backButton?.focus({ preventScroll: true });
	});
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

<!-- Every dead end offers the same two ways on: the publisher's own page in a
     tab, and — when extraction is what failed — the view that does not depend
     on it. Neither an error nor a refusal is ever the last word in the pane. -->
{#snippet waysOn(offerReaderView: boolean)}
	<div class="flex flex-wrap items-center gap-2">
		{#if offerReaderView && view !== 'reader'}
			<Button variant="outline" size="sm" onclick={() => onView('reader')} data-testid="try-reader">
				<BookOpenIcon data-icon="inline-start" />
				Reader View
			</Button>
		{/if}
		{#if offerReaderView && view === 'reader'}
			<Button
				variant="outline"
				size="sm"
				onclick={() => onView('original')}
				data-testid="try-original"
			>
				<GlobeIcon data-icon="inline-start" />
				Original View
			</Button>
		{/if}
		{@render openInNewTab()}
	</div>
{/snippet}

<!-- Header controls are 28px where a cursor points at them and 36px where a
     thumb does, which is the width the phone triage session needs. -->
{#snippet control(
	label: string,
	icon: typeof RssIcon,
	onclick: () => void,
	pressed: boolean,
	disabled: boolean,
	testid: string,
	swapIcon: typeof RssIcon | undefined,
	swapActive: boolean
)}
	<Tooltip.Root>
		<Tooltip.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					variant={pressed ? 'secondary' : 'ghost'}
					size="icon-sm"
					class="@max-3xl:size-9"
					aria-label={label}
					aria-pressed={pressed}
					data-testid={testid}
					{disabled}
					{onclick}
				>
					{#if swapIcon}
						{@const On = icon}
						{@const Off = swapIcon}
						<IconSwap active={swapActive}>
							{#snippet on()}
								<On />
							{/snippet}
							{#snippet off()}
								<Off />
							{/snippet}
						</IconSwap>
					{:else}
						{@const Icon = icon}
						<Icon class={pressed ? 'fill-current' : undefined} />
					{/if}
				</Button>
			{/snippet}
		</Tooltip.Trigger>
		<Tooltip.Content>{label}</Tooltip.Content>
	</Tooltip.Root>
{/snippet}

<!-- The three views are one choice, so they read as one control on a track
     rather than as three of the seven identical glyphs the header used to be.
     The chosen view is the only lifted surface in the group. -->
{#snippet viewControl(id: EntryView, label: string, icon: typeof RssIcon)}
	{@const Icon = icon}
	<Tooltip.Root>
		<Tooltip.Trigger>
			{#snippet child({ props })}
				<Tabs.Trigger
					{...props}
					data-slot="tabs-trigger"
					value={id}
					disabled={busy}
					aria-label={label}
					data-testid={`view-${id}`}
					class="relative flex size-7 items-center justify-center rounded-[calc(var(--radius)*1.8_-_2px)] text-muted-foreground transition-[color,scale] hover:text-foreground active:scale-[0.96] focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50 data-active:text-foreground @max-3xl:size-9"
				>
					<Icon class="size-4" />
				</Tabs.Trigger>
			{/snippet}
		</Tooltip.Trigger>
		<Tooltip.Content>{label}</Tooltip.Content>
	</Tooltip.Root>
{/snippet}

<!-- One component, two containers: a column of the layout once there is room
     for three, and a full-bleed overlay over the Entry List before that. The
     page marks what is behind it inert, which is what keeps the tab order
     inside the overlay without a Sheet to do it for us. Only the overlay
     animates: arriving over the list is a change of place, whereas the third
     column was already there. -->
<section
	data-testid="reading-pane"
	aria-label="Reading Pane"
	class="absolute inset-0 z-30 flex flex-col bg-background @max-3xl:motion-safe:animate-in @max-3xl:motion-safe:slide-in-from-right-8 @max-3xl:motion-safe:duration-300 @max-3xl:motion-safe:ease-[cubic-bezier(0.16,1,0.3,1)] @3xl:static @3xl:z-auto @3xl:flex-1 @3xl:border-s @3xl:border-border"
	out:fly={exit}
>
	<Tooltip.Provider delayDuration={400}>
		<!-- Eight 36px controls and their divider need 311px, so below 21rem the
		     bar tightens its own margins rather than letting the trailing
		     control run into the edge of the screen. -->
		<header
			class="flex h-12 shrink-0 items-center gap-1 border-b border-border bg-background/95 px-2 backdrop-blur @max-3xl:h-14 @max-3xl:gap-0 @max-[21rem]:px-1"
		>
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							bind:ref={backButton}
							variant="ghost"
							size="icon-sm"
							class="@3xl:hidden @max-3xl:size-9"
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
				class="min-w-0 flex-1 truncate px-1 text-sm font-medium transition-opacity duration-150 @max-3xl:hidden"
				class:opacity-0={!titleScrolledAway}
				aria-hidden={!titleScrolledAway}
			>
				{entry.title || entry.url}
			</p>

			<!-- On a phone the eight controls are the whole bar, so the spare
			     width sits between the two groups rather than beside them. -->
			<div class="flex-1 @3xl:hidden"></div>

			<Tabs.Root value={view} onValueChange={(value) => onView(value as EntryView)} class="shrink-0">
				<Tabs.List aria-label="View" class="gap-0.5 p-0.5">
					{#each views as choice (choice.id)}
						{@render viewControl(choice.id, choice.label, choice.icon)}
					{/each}
				</Tabs.List>
			</Tabs.Root>

			<div
				class="mx-1.5 h-5 w-px shrink-0 bg-border @max-3xl:mx-1 @max-[21rem]:mx-0.5"
				aria-hidden="true"
			></div>

			<div class="flex shrink-0 items-center gap-0.5">
				{@render control(
					entry.starred ? 'Unstar' : 'Star',
					StarIcon,
					onToggleStar,
					entry.starred,
					busy,
					'entry-star',
					undefined,
					false
				)}
				{@render control(
					entry.archived ? 'Unarchive' : 'Archive',
					ArchiveRestoreIcon,
					onToggleArchive,
					false,
					busy,
					'entry-archive',
					ArchiveIcon,
					entry.archived
				)}
				{@render control(
					entry.read ? 'Mark unread' : 'Mark read',
					MailIcon,
					onToggleRead,
					false,
					busy || entry.archived,
					'entry-read',
					MailOpenIcon,
					entry.read
				)}
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								variant="ghost"
								size="icon-sm"
								class="@max-3xl:size-9"
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

	<div bind:this={scroller} data-testid="entry-content" class="scrollbar-hover flex-1 overflow-y-auto">
		<!-- Reader View and Feed View are our own markup and are held to a
		     readable measure. Original View is the publisher's layout and gets the
		     whole pane, or a responsive site renders its phone design in a
		     desktop-sized column. -->
		<div
			class={view === 'original'
				? 'flex h-full flex-col gap-5 p-4'
				: 'mx-auto flex max-w-2xl flex-col gap-5 px-6 py-8'}
		>
			<div bind:this={titleAnchor} class="flex flex-col gap-2">
				<h2 class="text-2xl leading-tight font-semibold break-words text-balance">
					{entry.title || entry.url}
				</h2>
				<p class="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
					<FeedIcon feedTitle={entry.feed_title} {iconUrl} />
					<span class="min-w-0 truncate">{entry.feed_title}</span>
					<span aria-hidden="true">·</span>
					<span class="shrink-0">{formatPublished(entry.published_at)}</span>
				</p>
			</div>

			<div
				dir="auto"
				class="max-w-none flex-1 text-base leading-relaxed break-words text-foreground [&_a]:underline [&_a]:decoration-from-font [&_a]:[text-underline-position:from-font] [&_a]:[text-decoration-skip-ink:auto] [&_blockquote]:my-3 [&_blockquote]:border-s-2 [&_blockquote]:border-border [&_blockquote]:ps-3 [&_blockquote]:text-muted-foreground [&_h2]:mt-6 [&_h2]:mb-3 [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:mt-5 [&_h3]:mb-2 [&_h3]:text-lg [&_h3]:font-semibold [&_img]:max-w-full [&_img]:rounded-md [&_img]:outline [&_img]:outline-1 [&_img]:-outline-offset-1 [&_img]:outline-prose-image-outline [&_ol]:my-3 [&_ol]:list-decimal [&_ol]:ps-6 [&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:ps-6"
			>
				{#if loading}
					<!-- The shape of what is coming, rather than a sentence about it:
					     the wait is short and the pane should not jump when it ends. -->
					<div class="flex flex-col gap-3" data-testid="view-loading">
						<span class="sr-only">
							{view === 'reader' ? 'Extracting the Article…' : 'Asking the publisher…'}
						</span>
						<Skeleton class="h-4 w-full rounded-md" />
						<Skeleton class="h-4 w-11/12 rounded-md" />
						<Skeleton class="h-4 w-4/5 rounded-md" />
						<Skeleton class="mt-3 h-4 w-full rounded-md" />
						<Skeleton class="h-4 w-10/12 rounded-md" />
						<Skeleton class="h-4 w-2/3 rounded-md" />
					</div>
				{:else if loadError}
					<div class="flex max-w-2xl flex-col items-start gap-3">
						<p class="text-destructive" data-testid="view-error">{loadError}</p>
						{@render waysOn(true)}
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
						<div
							class="flex max-w-2xl flex-col items-start gap-3"
							data-testid="original-view-unsafe"
						>
							<p class="text-muted-foreground">
								This address shares Reader's host, so embedding it could expose your session. Open it
								in a new tab instead.
							</p>
							{@render waysOn(true)}
						</div>
					{:else if original}
						<div
							class="flex max-w-2xl flex-col items-start gap-3"
							data-testid="original-view-forbidden"
						>
							<p class="text-muted-foreground">
								This publisher does not allow their page to be shown inside another site. Reader View
								still works, or the page can be opened in a new tab.
							</p>
							{@render waysOn(true)}
						</div>
					{/if}
				{:else}
					{@html entry.content}
				{/if}
			</div>
		</div>
	</div>
</section>
