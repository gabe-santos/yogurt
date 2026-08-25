<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog';
  import { ApiError, search as runSearch } from '$lib/api';
  import type { Feed, SearchEntry } from '$lib/api';
  import { formatPublished } from '$lib/format';

  interface Props {
    onClose: () => void;
    onSelectEntry: (entry: SearchEntry) => void;
    onSelectFeed: (feed: Feed) => void;
  }

  const { onClose, onSelectEntry, onSelectFeed }: Props = $props();

  /** Result is one row of the combined, keyboard-navigable list: a Feed to
   * jump to, or an Entry to open. */
  type Result =
    | { kind: 'feed'; feed: Feed }
    | { kind: 'entry'; entry: SearchEntry };

  let query = $state('');
  let feeds = $state<Feed[]>([]);
  let entries = $state<SearchEntry[]>([]);
  let searching = $state(false);
  let error = $state('');
  let activeIndex = $state(0);
  let input = $state<HTMLInputElement | undefined>(undefined);
  let debounceHandle: ReturnType<typeof setTimeout> | undefined;
  let requestID = 0;

  const results = $derived<Result[]>([
    ...feeds.map((feed): Result => ({ kind: 'feed', feed })),
    ...entries.map((entry): Result => ({ kind: 'entry', entry })),
  ]);

  $effect(() => {
    input?.focus();
  });

  function scheduleSearch(next: string) {
    query = next;
    clearTimeout(debounceHandle);
    if (next.trim() === '') {
      feeds = [];
      entries = [];
      searching = false;
      error = '';
      return;
    }
    searching = true;
    debounceHandle = setTimeout(() => void runQuery(next), 200);
  }

  async function runQuery(text: string) {
    const id = ++requestID;
    try {
      const found = await runSearch(text);
      if (id !== requestID) return;
      feeds = found.feeds;
      entries = found.entries;
      activeIndex = 0;
      error = '';
    } catch (cause) {
      if (id !== requestID) return;
      error = cause instanceof ApiError ? cause.message : 'Could not reach the server';
    } finally {
      if (id === requestID) searching = false;
    }
  }

  function select(result: Result) {
    if (result.kind === 'feed') {
      onSelectFeed(result.feed);
    } else {
      onSelectEntry(result.entry);
    }
  }

  function onInputKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      if (results.length > 0) activeIndex = (activeIndex + 1) % results.length;
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      if (results.length > 0) activeIndex = (activeIndex - 1 + results.length) % results.length;
    } else if (event.key === 'Enter') {
      event.preventDefault();
      const active = results[activeIndex];
      if (active) select(active);
    }
  }
</script>

<Dialog.Root open onOpenChange={(open) => !open && onClose()}>
  <Dialog.Content data-testid="search-dialog" class="gap-3 sm:max-w-lg">
    <Dialog.Title class="sr-only">Search</Dialog.Title>
    <input
      bind:this={input}
      data-testid="search-input"
      type="text"
      placeholder="Search Entries and Feeds…"
      class="w-full border-b border-border bg-transparent pb-2 text-base outline-none placeholder:text-muted-foreground"
      value={query}
      oninput={(event) => scheduleSearch((event.target as HTMLInputElement).value)}
      onkeydown={onInputKeydown}
    />

    {#if error}
      <p class="text-sm text-destructive" data-testid="search-error">{error}</p>
    {:else if searching}
      <p class="text-sm text-muted-foreground">Searching…</p>
    {:else if query.trim() !== '' && results.length === 0}
      <p class="text-sm text-muted-foreground">No matches.</p>
    {:else if results.length > 0}
      <ul class="flex max-h-96 flex-col overflow-y-auto" data-testid="search-results">
        {#if feeds.length > 0}
          <li class="px-1 pb-1 text-xs font-medium text-muted-foreground">Feeds</li>
        {/if}
        {#each results as result, index (result.kind + ':' + (result.kind === 'feed' ? result.feed.id : result.entry.id))}
          {#if result.kind === 'entry' && index === feeds.length && feeds.length > 0}
            <li class="px-1 pt-2 pb-1 text-xs font-medium text-muted-foreground">Entries</li>
          {/if}
          <li>
            <button
              type="button"
              data-testid="search-result"
              aria-current={index === activeIndex}
              class="flex w-full flex-col gap-0.5 rounded-md px-2 py-2 text-left hover:bg-accent aria-[current=true]:bg-accent"
              onmouseenter={() => (activeIndex = index)}
              onclick={() => select(result)}
            >
              {#if result.kind === 'feed'}
                <span class="font-medium">{result.feed.title}</span>
                <span class="text-xs text-muted-foreground">Feed · {result.feed.unread_count} unread</span>
              {:else}
                <span class="font-medium">{result.entry.title || result.entry.url}</span>
                <span class="text-xs text-muted-foreground">
                  {result.entry.feed_title} · {formatPublished(result.entry.published_at)}
                </span>
                {#if result.entry.snippet}
                  <span class="text-xs text-muted-foreground">{result.entry.snippet}</span>
                {/if}
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </Dialog.Content>
</Dialog.Root>
