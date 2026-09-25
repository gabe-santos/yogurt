<script lang="ts">
  import type { Entry } from '$lib/api';
  import * as ContextMenu from '$lib/components/ui/context-menu';
  import FeedIcon from '$lib/FeedIcon.svelte';
  import { entryExcerpt, formatEntryAge, formatPublished } from '$lib/format';
  import ArchiveIcon from '@lucide/svelte/icons/archive';
  import ArchiveRestoreIcon from '@lucide/svelte/icons/archive-restore';
  import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
  import CircleIcon from '@lucide/svelte/icons/circle';
  import StarIcon from '@lucide/svelte/icons/star';

  interface Props {
    entry: Entry;
    isCurrent: boolean;
    /** iconUrl is the Feed's stored Feed Icon, undefined when it has none. */
    iconUrl?: string;
    /** tabbable makes this the Entry List's single tab stop, which is to say
     * it marks the row the reader is at. The list is one control the reader
     * moves through with j/k, so Tab reaches it once and then leaves for the
     * Reading Pane rather than walking every loaded row. */
    tabbable: boolean;
    /** focusRequest counts the keyboard acts that should leave focus on the
     * row the reader is at; 0 asks for none. It is read rather than compared
     * so that the same row can be asked twice — closing the Reading Pane
     * overlay asks for the focus the pane took. */
    focusRequest: number;
    disabled: boolean;
    onClick: () => void;
    onToggleRead: () => void;
    onToggleStar: () => void;
    onToggleArchive: () => void;
  }

  const {
    entry,
    isCurrent,
    iconUrl,
    tabbable,
    focusRequest,
    disabled,
    onClick,
    onToggleRead,
    onToggleStar,
    onToggleArchive,
  }: Props = $props();

  // excerpt is blank whenever the Entry's Feed carried no body worth
  // showing, so the row can skip the line entirely instead of leaving an
  // empty box beneath the title.
  const excerpt = $derived(entryExcerpt(entry.content));
  // The row shows the Entry's age, which is what a newest-first list is read
  // by; the exact date stays one hover away rather than eating the line the
  // Feed name also has to fit on.
  const age = $derived(formatEntryAge(entry.published_at));
  const published = $derived(formatPublished(entry.published_at));

  let button = $state<HTMLButtonElement | null>(null);
  // j and k move the reader's position without touching the list's scroll
  // box, so the row they land on has to bring itself into view or the reader
  // ends up reading an Entry whose row is a screenful away. `nearest` scrolls
  // by the least that makes the row whole, and does nothing when it already
  // is. Focus follows the keyboard only: it is what announces the move to a
  // screen reader and leaves Tab continuing from the right row, and it is
  // also how the narrow-screen overlay hands focus back to the list it
  // covered.
  $effect(() => {
    const requested = focusRequest > 0;
    const row = button;
    if (!row || !tabbable) return;
    row.scrollIntoView({ block: "nearest" });
    if (requested) row.focus({ preventScroll: true });
  });
</script>

<!-- The separator below a row and the one above it are the border of this
     `li` and of the one before it; both clear while the row is highlighted,
     or they would poke out past its rounded corners. -->
<li
  class="flex flex-col transition-colors hover:border-transparent has-focus-visible:border-transparent has-aria-[current=true]:border-transparent [&:has(+li:hover)]:border-transparent [&:has(+li_:focus-visible)]:border-transparent [&:has(+li_[aria-current=true])]:border-transparent"
>
  <ContextMenu.Root>
    <ContextMenu.Trigger>
      {#snippet child({ props })}
        <!-- The row answers the press, not the release: selection lands on
             click, and without a pressed state the ~100ms in between reads as
             a dead row. The highlight is the one the selected row already
             wears, so a press previews its own outcome; it is applied with no
             transition on the way in and the shared one on the way out, which
             is instant to the finger and still graceful when it lets go. -->
        <button
          {...props}
          bind:this={button}
          type="button"
          data-testid="entry"
          aria-current={isCurrent}
          tabindex={tabbable ? 0 : -1}
          class="flex -ms-6.5 -me-5 ps-6.5 pe-5 flex-col gap-1 rounded-xl pt-4 pb-5  text-left transition-colors hover:bg-accent/50 focus-visible:bg-accent/50 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring active:bg-accent active:duration-0 aria-[current=true]:bg-accent"
          onclick={onClick}
        >
          <span class="relative flex w-full min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
            {#if !entry.read}
              <!-- The dot hangs in the row's leading padding, as Mail's does,
              so the Feed Icon lines up with the title on every row. It sits
              centred there: ps-6.5 is the 6px dot with 10px either side, so
              -start-4 is 10px plus the dot. It is colour-only, so an sr-only
              label carries the state for assistive tech instead of relying
              on the fill alone. -->
              <span
                class="absolute top-1/2 -start-4 size-1.5 -translate-y-1/2 rounded-full bg-primary"
                aria-hidden="true"
              ></span>
              <span class="sr-only">unread</span>
            {/if}
            <FeedIcon feedTitle={entry.feed_title} {iconUrl} />
            <span class="min-w-0 truncate">{entry.feed_title}</span>
            <span class="ms-auto flex shrink-0 items-center gap-1.5">
              {#if entry.starred}
                <StarIcon class="size-3 fill-current" strokeWidth={1.5} aria-hidden="true" />
                <span class="sr-only">Starred</span>
              {/if}
              {#if entry.archived}
                <ArchiveIcon class="size-3" strokeWidth={1.5} aria-hidden="true" />
                <span class="sr-only">Archived</span>
              {/if}
              <span class="tabular-nums" title={published}>{age}</span>
            </span>
          </span>
          <span
            class="line-clamp-2 text-sm font-medium @max-3xl:text-base"
            class:text-muted-foreground={entry.read}
          >
            {entry.title || entry.url}
          </span>
          {#if excerpt}
            <span class="line-clamp-2 text-xs text-muted-foreground">
              {excerpt}
            </span>
          {/if}
        </button>
      {/snippet}
    </ContextMenu.Trigger>
    <ContextMenu.Content data-testid="entry-context-menu">
      <ContextMenu.Group>
        <ContextMenu.Item
          disabled={disabled || entry.archived}
          onclick={onToggleRead}
        >
          {#if entry.read}
            <CircleCheckIcon strokeWidth={1.5} />
            Mark unread
          {:else}
            <CircleIcon strokeWidth={1.5} />
            Mark read
          {/if}
        </ContextMenu.Item>
        <ContextMenu.Item disabled={disabled} onclick={onToggleStar}>
          <StarIcon strokeWidth={1.5} class={entry.starred ? 'fill-current' : undefined} />
          {entry.starred ? 'Unstar' : 'Star'}
        </ContextMenu.Item>
      </ContextMenu.Group>
      <ContextMenu.Separator />
      <ContextMenu.Group>
        <ContextMenu.Item disabled={disabled} onclick={onToggleArchive}>
          {#if entry.archived}
            <ArchiveRestoreIcon strokeWidth={1.5} />
            Unarchive
          {:else}
            <ArchiveIcon strokeWidth={1.5} />
            Archive
          {/if}
        </ContextMenu.Item>
      </ContextMenu.Group>
    </ContextMenu.Content>
  </ContextMenu.Root>
</li>
