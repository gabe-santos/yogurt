<script lang="ts">
  import { feedIconUrl } from '$lib/api';
  import type { Feed } from '$lib/api';
  import * as ContextMenu from '$lib/components/ui/context-menu';
  import * as Dialog from '$lib/components/ui/dialog';
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
  import * as Sidebar from '$lib/components/ui/sidebar';
  import { cn } from '$lib/utils';
  import { formatPublished } from '$lib/format';
  import FeedIcon from '$lib/FeedIcon.svelte';
  import EllipsisIcon from '@lucide/svelte/icons/ellipsis';
  import PencilIcon from '@lucide/svelte/icons/pencil';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

  interface Props {
    feed: Feed;
    isActive: boolean;
    onSelect: () => void;
    onEdit: () => void;
    onDelete: () => void;
  }

  const { feed, isActive, onSelect, onEdit, onDelete }: Props = $props();

  // The clamped preview stays two lines so a stack-trace-shaped reason can't
  // blow up the menu; this dialog is the keyboard- and touch-reachable path
  // to the rest of it, opened from a menu item rather than a hover tooltip.
  let showError = $state(false);

  // Right-click and the "…" button open the identical menu, so its body is
  // written once and rendered inside both ContextMenu.Content and
  // DropdownMenu.Content: the two component families share the same prop
  // shape for every member used here.
  type MenuNamespace = typeof ContextMenu | typeof DropdownMenu;
</script>

{#snippet feedMenuBody(M: MenuNamespace)}
  <!-- Silence and breakage read alike in the Collection List, so the last check
       is stated rather than inferred. A stack-trace-shaped failure reason is
       clamped so it cannot blow up the menu. -->
  <div class="px-2 pt-1.5 pb-2 text-xs text-muted-foreground">
    <p>
      {#if feed.last_success_at}
        Checked {formatPublished(feed.last_success_at)}
      {:else}
        Not checked yet
      {/if}
    </p>
    {#if feed.last_error}
      <p class="mt-1 line-clamp-2 text-destructive">{feed.last_error}</p>
    {/if}
  </div>
  {#if feed.last_error}
    <M.Separator />
    <M.Group>
      <M.Item onclick={() => (showError = true)}>
        <TriangleAlertIcon strokeWidth={1.5} />
        View full error
      </M.Item>
    </M.Group>
  {/if}
  <M.Separator />
  <M.Group>
    <M.Item onclick={onEdit}>
      <PencilIcon strokeWidth={1.5} />
      Edit
    </M.Item>
  </M.Group>
  <M.Separator />
  <M.Group>
    <!-- The one act here that cannot be undone is separated from the
         reversible ones, and says what it deletes when it is confirmed. -->
    <M.Item onclick={onDelete}>
      <Trash2Icon strokeWidth={1.5} />
      Delete Feed
    </M.Item>
  </M.Group>
{/snippet}

<ContextMenu.Root>
  <ContextMenu.Trigger>
    {#snippet child({ props })}
      <!-- The unread count is absolutely positioned chrome, so the name has to
           be told to stop before it. -->
      <Sidebar.MenuButton
        {...props}
        data-testid="feed"
        class={cn(
          (props as { class?: string }).class,
          feed.unread_count > 0 ? 'pr-10' : undefined,
        )}
        {isActive}
        aria-current={isActive}
        onclick={onSelect}
      >
        <FeedIcon feedTitle={feed.title} iconUrl={feedIconUrl(feed)} />
        {#if feed.last_error}
          <TriangleAlertIcon
            strokeWidth={1.5}
            class="size-3 shrink-0 text-destructive"
            role="img"
            aria-label="This Feed is failing"
          />
        {/if}
        <span class="truncate">
          {feed.title}
        </span>
      </Sidebar.MenuButton>
    {/snippet}
  </ContextMenu.Trigger>
  <ContextMenu.Content data-testid="feed-context-menu">
    {@render feedMenuBody(ContextMenu)}
  </ContextMenu.Content>
</ContextMenu.Root>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <Sidebar.MenuAction
        {...props}
        data-testid="feed-menu-button"
        showOnHover
        aria-label={`${feed.title} menu`}
      >
        <EllipsisIcon strokeWidth={1.5} />
      </Sidebar.MenuAction>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content data-testid="feed-menu" align="start">
    {@render feedMenuBody(DropdownMenu)}
  </DropdownMenu.Content>
</DropdownMenu.Root>

{#if feed.last_error}
  <Dialog.Root bind:open={showError}>
    <Dialog.Content data-testid="feed-error-dialog" class="sm:max-w-md">
      <Dialog.Header>
        <Dialog.Title>{feed.title} error</Dialog.Title>
      </Dialog.Header>
      <p class="max-h-[60vh] overflow-y-auto text-sm whitespace-pre-wrap text-destructive">
        {feed.last_error}
      </p>
    </Dialog.Content>
  </Dialog.Root>
{/if}
{#if feed.unread_count > 0}
  <!-- The badge and the "…" action share one slot (top-1.5 right-1): on
       touch the action is always shown there, and on desktop it takes the
       slot back on hover/focus, so the badge steps aside rather than
       crowding beside it. -->
  <Sidebar.MenuBadge
    class="top-1 tabular-nums hidden md:flex md:group-hover/menu-item:hidden md:group-focus-within/menu-item:hidden"
  >
    {feed.unread_count}
  </Sidebar.MenuBadge>
{/if}
