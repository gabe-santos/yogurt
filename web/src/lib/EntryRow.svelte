<script lang="ts">
  import type { Entry } from '$lib/api';
  import FeedIcon from '$lib/FeedIcon.svelte';
  import { formatPublished } from '$lib/format';

  interface Props {
    entry: Entry;
    isCurrent: boolean;
    /** iconUrl is the Feed's stored Feed Icon, undefined when it has none. */
    iconUrl?: string;
    onClick: () => void;
  }

  const { entry, isCurrent, iconUrl, onClick }: Props = $props();
</script>

<li>
  <button
    type="button"
    data-testid="entry"
    aria-current={isCurrent}
    class="flex w-full flex-col gap-1 py-3 text-left hover:bg-accent/50 aria-[current=true]:bg-accent"
    onclick={onClick}
  >
    <span
      class="font-medium underline-offset-2 hover:underline"
      class:text-muted-foreground={entry.read}
    >
      {entry.title || entry.url}
    </span>
    <span class="flex items-center gap-1.5 text-xs text-muted-foreground">
      <FeedIcon feedTitle={entry.feed_title} {iconUrl} />
      <span>
        {entry.feed_title} · {formatPublished(entry.published_at)}{entry.read
          ? ''
          : ' · unread'}{entry.starred ? ' · Starred' : ''}{entry.archived
          ? ' · Archived'
          : ''}
      </span>
    </span>
  </button>
</li>
