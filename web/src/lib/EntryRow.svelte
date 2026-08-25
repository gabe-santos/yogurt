<script lang="ts">
  import type { Entry } from '$lib/api';
  import FeedIcon from '$lib/FeedIcon.svelte';
  import { entryExcerpt, formatPublished } from '$lib/format';
  import ArchiveIcon from '@lucide/svelte/icons/archive';
  import StarIcon from '@lucide/svelte/icons/star';

  interface Props {
    entry: Entry;
    isCurrent: boolean;
    /** iconUrl is the Feed's stored Feed Icon, undefined when it has none. */
    iconUrl?: string;
    onClick: () => void;
  }

  const { entry, isCurrent, iconUrl, onClick }: Props = $props();

  // excerpt is blank whenever the Entry's Feed carried no body worth
  // showing, so the row can skip the line entirely instead of leaving an
  // empty box beneath the title.
  const excerpt = $derived(entryExcerpt(entry.content));
</script>

<li>
  <button
    type="button"
    data-testid="entry"
    aria-current={isCurrent}
    class="flex w-full flex-col gap-1 py-3 text-left hover:bg-accent/50 aria-[current=true]:bg-accent"
    onclick={onClick}
  >
    <span class="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
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
      <span class="shrink-0">·</span>
      <span class="shrink-0">{formatPublished(entry.published_at)}</span>
      <span class="ml-auto flex shrink-0 items-center gap-1.5">
        {#if entry.starred}
          <StarIcon class="size-3" aria-hidden="true" />
          <span class="sr-only">Starred</span>
        {/if}
        {#if entry.archived}
          <ArchiveIcon class="size-3" aria-hidden="true" />
          <span class="sr-only">Archived</span>
        {/if}
      </span>
    </span>
    <span class="line-clamp-2 font-medium" class:text-muted-foreground={entry.read}>
      {entry.title || entry.url}
    </span>
    {#if excerpt}
      <span class="line-clamp-2 text-xs text-muted-foreground">
        {excerpt}
      </span>
    {/if}
  </button>
</li>
