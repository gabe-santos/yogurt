<script lang="ts">
  import type { Snippet } from 'svelte';
  import { cn } from '$lib/utils';

  interface Props {
    /** active chooses which icon is on screen. */
    active: boolean;
    /** on renders while active, off while not. Both stay mounted. */
    on: Snippet;
    off: Snippet;
  }

  const { active, on, off }: Props = $props();

  // Both icons share one grid cell, so each has an enter and an exit: the icon
  // leaving shrinks and blurs away while its replacement grows in. Toggling
  // visibility gives you neither, and in a button whose label never changes the
  // swap is the sign that the action landed.
  const cell =
    'col-start-1 row-start-1 flex transition-[opacity,scale,filter] duration-300 ease-[cubic-bezier(0.2,0,0,1)]';
  const shown = 'scale-100 opacity-100 blur-[0px]';
  const hidden = 'scale-25 opacity-0 blur-[4px]';
</script>

<span class="grid shrink-0 place-items-center">
  <span class={cn(cell, active ? shown : hidden)} aria-hidden={!active}>
    {@render on()}
  </span>
  <span class={cn(cell, active ? hidden : shown)} aria-hidden={active}>
    {@render off()}
  </span>
</span>
