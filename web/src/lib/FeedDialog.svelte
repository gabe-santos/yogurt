<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import { addFeed, describeError, updateFeed } from '$lib/api';
	import type { Feed } from '$lib/api';

	interface Props {
		/** The Feed to edit; absent, the dialog adds a new one. */
		feed?: Feed;
		onClose: () => void;
		onSaved: (feed: Feed) => void;
	}

	const { feed, onClose, onSaved }: Props = $props();

	// Mounted fresh for each opening, so the Feed's values are only the
	// starting point of the draft.
	// svelte-ignore state_referenced_locally
	let address = $state(feed?.url ?? '');
	// svelte-ignore state_referenced_locally
	let name = $state(feed?.title ?? '');
	let submitting = $state(false);
	let error = $state('');
	// The dialog opens with focus already in the one required field, the way
	// every other dialog and inline edit in this app takes focus for itself.
	let addressInput = $state<HTMLInputElement | null>(null);
	$effect(() => {
		addressInput?.focus();
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (submitting) return;
		if (feed && address.trim() === feed.url && name.trim() === feed.title) {
			onClose();
			return;
		}
		error = '';
		submitting = true;
		try {
			onSaved(
				feed
					? await updateFeed(feed.id, { url: address, title: name })
					: await addFeed(address, name),
			);
		} catch (cause) {
			error = describeError(cause, 'Could not reach the server');
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root open onOpenChange={(open) => !open && onClose()}>
	<Dialog.Content data-testid={feed ? 'edit-feed-dialog' : 'add-feed-dialog'} class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title class="text-lg font-semibold tracking-lg">{feed ? 'Edit Feed' : 'Add Feed'}</Dialog.Title>
		</Dialog.Header>

		<form class="flex flex-col gap-4" onsubmit={submit}>
			<Field.FieldSet data-invalid={error ? true : undefined}>
				<Field.FieldLabel for="feed-address">Feed or site address</Field.FieldLabel>
				<Input
					id="feed-address"
					bind:ref={addressInput}
					type="url"
					required
					placeholder="https://example.com"
					bind:value={address}
					aria-invalid={error ? true : undefined}
				/>
				<Field.FieldError errors={error ? [{ message: error }] : undefined} />
			</Field.FieldSet>

			<Field.FieldSet>
				<Field.FieldLabel for="feed-name">Name</Field.FieldLabel>
				<Input id="feed-name" bind:value={name} />
				<Field.FieldDescription>Defaults to the publisher's own title.</Field.FieldDescription>
			</Field.FieldSet>

			<Dialog.Footer>
				<Button type="submit" disabled={submitting}>
					{#if submitting}
						<LoaderCircleIcon class="animate-spin" />
					{/if}
					{feed ? 'Save' : 'Add Feed'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
