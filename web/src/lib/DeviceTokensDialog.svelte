<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import { formatPublished } from '$lib/format';
	import {
		ApiError,
		createDeviceToken,
		listDeviceTokens,
		revokeDeviceToken
	} from '$lib/api';
	import type { DeviceToken } from '$lib/api';
	import { onMount } from 'svelte';

	interface Props {
		onClose: () => void;
	}

	const { onClose }: Props = $props();

	let tokens = $state<DeviceToken[]>([]);
	let newName = $state('');
	let creating = $state(false);
	let revealed = $state<string | null>(null);
	let error = $state('');

	onMount(async () => {
		try {
			tokens = await listDeviceTokens();
		} catch {
			error = 'Could not load your device tokens.';
		}
	});

	async function submitCreate(event: SubmitEvent) {
		event.preventDefault();
		const name = newName.trim();
		if (!name || creating) return;
		creating = true;
		error = '';
		try {
			const { device_token, token } = await createDeviceToken(name);
			tokens = [...tokens, device_token];
			revealed = token;
			newName = '';
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Could not create that device token.';
		} finally {
			creating = false;
		}
	}

	async function revoke(id: number) {
		const previous = tokens;
		tokens = tokens.filter((t) => t.id !== id);
		try {
			await revokeDeviceToken(id);
		} catch {
			tokens = previous;
			error = 'Could not revoke that device token.';
		}
	}
</script>

<Dialog.Root open onOpenChange={(open) => !open && onClose()}>
	<Dialog.Content data-testid="device-tokens-dialog" class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="text-lg font-semibold">Device tokens</Dialog.Title>
		</Dialog.Header>

		<p class="text-sm text-muted-foreground">
			A device token authenticates a non-browser client anywhere a session cookie would. Its raw
			value is shown once, when it is created.
		</p>

		{#if error}
			<p class="text-sm text-destructive">{error}</p>
		{/if}

		{#if revealed}
			<div class="flex flex-col gap-1 rounded-md bg-muted p-3 shadow-border">
				<span class="text-xs text-muted-foreground">Copy this now — it will not be shown again.</span>
				<code class="break-all text-sm" data-testid="revealed-token">{revealed}</code>
				<Button
					variant="outline"
					size="sm"
					class="mt-1 self-start"
					onclick={() => (revealed = null)}
				>
					Done
				</Button>
			</div>
		{/if}

		<!-- The list is what grows without limit here, so it is what scrolls: the
		     form below it is the reason the dialog is open and stays in view. -->
		<ul class="scrollbar-hover flex max-h-64 flex-col gap-2 overflow-y-auto">
			{#each tokens as token (token.id)}
				<li
					class="flex items-center justify-between gap-3 rounded-md px-3 py-2 text-sm shadow-border"
				>
					<div class="flex flex-col">
						<span class="font-medium">{token.name}</span>
						<span class="text-xs text-muted-foreground">
							{token.last_used_at
								? `Last used ${formatPublished(token.last_used_at)}`
								: 'Never used'}
						</span>
					</div>
					<Button variant="outline" size="sm" onclick={() => revoke(token.id)}>Revoke</Button>
				</li>
			{:else}
				<li class="text-sm text-muted-foreground">No device tokens yet.</li>
			{/each}
		</ul>

		<form class="flex items-end gap-2" onsubmit={submitCreate}>
			<Field.FieldSet class="flex-1">
				<Field.FieldLabel for="new-device-token">Name</Field.FieldLabel>
				<Input
					id="new-device-token"
					placeholder="Desktop shell"
					bind:value={newName}
				/>
			</Field.FieldSet>
			<Button type="submit" disabled={creating || !newName.trim()}>Create</Button>
		</form>
	</Dialog.Content>
</Dialog.Root>
