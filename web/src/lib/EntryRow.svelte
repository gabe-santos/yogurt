<script lang="ts">
  import type { Entry } from '$lib/api';
  import FeedIcon from '$lib/FeedIcon.svelte';
  import { entryExcerpt, formatEntryAge, formatPublished } from '$lib/format';
  import ArchiveIcon from '@lucide/svelte/icons/archive';
  import StarIcon from '@lucide/svelte/icons/star';

  interface Props {
    entry: Entry;
    isCurrent: boolean;
    /** iconUrl is the Feed's stored Feed Icon, undefined when it has none. */
    iconUrl?: string;
    /** tabbable makes this the Entry List's single tab stop. The list is one
     * control the reader moves through with j/k, so Tab reaches it once and
     * then leaves for the Reading Pane rather than walking every loaded row. */
    tabbable: boolean;
    onClick: () => void;
  }

  const { entry, isCurrent, iconUrl, tabbable, onClick }: Props = $props();

  // excerpt is blank whenever the Entry's Feed carried no body worth
  // showing, so the row can skip the line entirely instead of leaving an
  // empty box beneath the title.
  const excerpt = $derived(entryExcerpt(entry.content));
  // The row shows the Entry's age, which is what a newest-first list is read
  // by; the exact date stays one hover away rather than eating the line the
  // Feed name also has to fit on.
  const age = $derived(formatEntryAge(entry.published_at));
  const published = $derived(formatPublished(entry.published_at));
</script>

<li>
  <button
    type="button"
    data-testid="entry"
    aria-current={isCurrent}
    tabindex={tabbable ? 0 : -1}
    class="flex w-full flex-col gap-1 px-3 py-3 text-left transition-colors hover:bg-accent/50 focus-visible:bg-accent/50 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring aria-[current=true]:bg-accent"
    onclick={onClick}
  >
    <span class="flex w-full min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
      {#if entry.read}
        <span class="size-1.5 shrink-0" aria-hidden="true"></span>
      {:else}
        <!-- The dot is colour-only, so an sr-only label carries the state
        for assistive tech instead of relying on the fill alone. -->
        <span class="size-1.5 shrink-0 rounded-full bg-primary" aria-hidden="true"></span>
        <span class="sr-only">unread</span>
      {/if}
      <FeedIcon feedTitle={entry.feed_title} {iconUrl} />
      <span class="min-w-0 truncate">{entry.feed_title}</span>
      <span class="ml-auto flex shrink-0 items-center gap-1.5">
        {#if entry.starred}
          <StarIcon class="size-3" aria-hidden="true" />
          <span class="sr-only">Starred</span>
        {/if}
        {#if entry.archived}
          <ArchiveIcon class="size-3" aria-hidden="true" />
          <span class="sr-only">Archived</span>
        {/if}
        <span class="tabular-nums" title={published}>{age}</span>
      </span>
    </span>
    <span
      class="line-clamp-2 leading-snug font-medium"
      class:text-muted-foreground={entry.read}
    >
      {entry.title || entry.url}
    </span>
    {#if excerpt}
      <span class="line-clamp-2 text-xs leading-snug text-muted-foreground">
        {excerpt}
      </span>
    {/if}
  </button>
</li>
