<script lang="ts">
  import { feedMonogram } from '$lib/format';

  interface Props {
    feedTitle: string;
    /** iconUrl is the Feed's stored Feed Icon, undefined when it has none. */
    iconUrl?: string;
  }

  const { feedTitle, iconUrl }: Props = $props();

  const monogram = $derived(feedMonogram(feedTitle));
  // imageFailed covers a stored icon that fails to decode in the browser —
  // an SVG or ICO byte-for-byte from a publisher is not guaranteed to
  // render — falling back to the monogram rather than a broken image.
  let imageFailed = $state(false);
</script>

<span
  data-testid="feed-icon"
  aria-hidden="true"
  class="flex h-4 w-4 shrink-0 items-center justify-center overflow-hidden rounded-sm bg-muted text-[9px] font-semibold leading-none text-muted-foreground"
>
  {#if iconUrl && !imageFailed}
    <img
      src={iconUrl}
      alt=""
      class="h-full w-full object-contain outline outline-1 -outline-offset-1 outline-black/10 dark:outline-white/10"
      onerror={() => (imageFailed = true)}
    />
  {:else}
    {monogram}
  {/if}
</span>
