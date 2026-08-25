<script lang="ts">
  import { feedIconUrl } from '$lib/api';
  import type { Feed, Group } from '$lib/api';
  import * as ContextMenu from '$lib/components/ui/context-menu';
  import * as Sidebar from '$lib/components/ui/sidebar';
  import { cn } from '$lib/utils';
  import FeedIcon from '$lib/FeedIcon.svelte';
  import CirclePauseIcon from '@lucide/svelte/icons/circle-pause';
  import CirclePlayIcon from '@lucide/svelte/icons/circle-play';
  import FolderIcon from '@lucide/svelte/icons/folder';
  import PencilIcon from '@lucide/svelte/icons/pencil';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

  interface Props {
    feed: Feed;
    /** groups is every Group this Feed could be moved to, the one it is
     * already in included: the menu states where the Feed is as much as it
     * offers to move it. */
    groups: Group[];
    isActive: boolean;
    onSelect: () => void;
    onRename: () => void;
    onToggleSuspend: () => void;
    onMove: (groupID: number) => void;
    onDelete: () => void;
  }

  const {
    feed,
    groups,
    isActive,
    onSelect,
    onRename,
    onToggleSuspend,
    onMove,
    onDelete,
  }: Props = $props();
</script>

<ContextMenu.Root>
  <ContextMenu.Trigger>
    {#snippet child({ props })}
      <!-- The unread count is absolutely positioned chrome, so the name has to
           be told to stop before it. -->
      <Sidebar.MenuSubButton
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
            aria-label="This Feed is failing"
          />
        {/if}
        <span
          class="truncate {feed.suspended
            ? 'text-muted-foreground italic'
            : ''}"
        >
          {feed.title}
        </span>
      </Sidebar.MenuSubButton>
    {/snippet}
  </ContextMenu.Trigger>
  <ContextMenu.Content data-testid="feed-context-menu">
    <ContextMenu.Group>
      <ContextMenu.Item onclick={onRename}>
        <PencilIcon strokeWidth={1.5} />
        Rename
      </ContextMenu.Item>
      <ContextMenu.Item onclick={onToggleSuspend}>
        {#if feed.suspended}
          <CirclePlayIcon strokeWidth={1.5} />
          Resume
        {:else}
          <CirclePauseIcon strokeWidth={1.5} />
          Suspend
        {/if}
      </ContextMenu.Item>
      <ContextMenu.Sub>
        <ContextMenu.SubTrigger class="gap-2">
          <FolderIcon strokeWidth={1.5} />
          Move to Group
        </ContextMenu.SubTrigger>
        <ContextMenu.SubContent>
          <!-- A Feed belongs to exactly one Group, so the Group it is in is
               the checked choice rather than a separate line of text. -->
          <ContextMenu.RadioGroup
            value={String(feed.group_id)}
            onValueChange={(value) => {
              const groupID = Number(value);
              if (groupID !== feed.group_id) onMove(groupID);
            }}
          >
            {#each groups as group (group.id)}
              <ContextMenu.RadioItem value={String(group.id)}>
                {group.name}
              </ContextMenu.RadioItem>
            {/each}
          </ContextMenu.RadioGroup>
        </ContextMenu.SubContent>
      </ContextMenu.Sub>
    </ContextMenu.Group>
    <ContextMenu.Separator />
    <ContextMenu.Group>
      <!-- The one act here that cannot be undone is separated from the
           reversible ones, and says what it deletes when it is confirmed. -->
      <ContextMenu.Item onclick={onDelete}>
        <Trash2Icon strokeWidth={1.5} />
        Delete Feed
      </ContextMenu.Item>
    </ContextMenu.Group>
  </ContextMenu.Content>
</ContextMenu.Root>
{#if feed.unread_count > 0}
  <Sidebar.MenuBadge class="top-1 tabular-nums">
    {feed.unread_count}
  </Sidebar.MenuBadge>
{/if}
