<script lang="ts">
  import type { Group } from '$lib/api';
  import * as ContextMenu from '$lib/components/ui/context-menu';
  import * as Sidebar from '$lib/components/ui/sidebar';
  import { cn } from '$lib/utils';
  import PencilIcon from '@lucide/svelte/icons/pencil';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';

  interface Props {
    group: Group;
    isActive: boolean;
    onSelect: () => void;
    onRename: () => void;
    onDelete: () => void;
  }

  const { group, isActive, onSelect, onRename, onDelete }: Props = $props();
</script>

<ContextMenu.Root>
  <ContextMenu.Trigger>
    {#snippet child({ props })}
      <!-- The unread count is absolutely positioned chrome, so the name has to
           be told to stop before it. -->
      <Sidebar.MenuButton
        {...props}
        data-testid="group"
        class={cn(
          (props as { class?: string }).class,
          group.unread_count > 0 ? 'pr-11' : undefined,
        )}
        {isActive}
        aria-current={isActive}
        onclick={onSelect}
      >
        <span class="truncate font-medium">{group.name}</span>
      </Sidebar.MenuButton>
    {/snippet}
  </ContextMenu.Trigger>
  <ContextMenu.Content data-testid="group-context-menu">
    <ContextMenu.Group>
      <ContextMenu.Item onclick={onRename}>
        <PencilIcon />
        Rename
      </ContextMenu.Item>
    </ContextMenu.Group>
    <!-- The default Group is where a Feed with no Group of its own lives, so
         there is nothing to offer: it cannot be deleted. -->
    {#if !group.is_default}
      <ContextMenu.Separator />
      <ContextMenu.Group>
        <ContextMenu.Item onclick={onDelete}>
          <Trash2Icon />
          Delete Group
        </ContextMenu.Item>
      </ContextMenu.Group>
    {/if}
  </ContextMenu.Content>
</ContextMenu.Root>
{#if group.unread_count > 0}
  <Sidebar.MenuBadge class="top-1.5 tabular-nums">
    {group.unread_count}
  </Sidebar.MenuBadge>
{/if}
