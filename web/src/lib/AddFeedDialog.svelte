<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import { addFeed, describeError } from '$lib/api';
	import type { Feed } from '$lib/api';

	interface Props {
		onClose: () => void;
		onFeedAdded: (feed: Feed) => void;
	}

	const { onClose, onFeedAdded }: Props = $props();

	let address = $state('');
	let name = $state('');
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
		error = '';
		submitting = true;
		try {
			const feed = await addFeed(address, name);
			onFeedAdded(feed);
		} catch (cause) {
			error = describeError(cause, 'Could not reach the server');
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root open onOpenChange={(open) => !open && onClose()}>
	<Dialog.Content data-testid="add-feed-dialog" class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title class="text-lg font-semibold">Add Feed</Dialog.Title>
		</Dialog.Header>

		<form class="flex flex-col gap-4" onsubmit={submit}>
			<Field.FieldSet data-invalid={error ? true : undefined}>
				<Field.FieldLabel for="add-feed-address">Feed or site address</Field.FieldLabel>
				<Input
					id="add-feed-address"
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
				<Field.FieldLabel for="add-feed-name">Name</Field.FieldLabel>
				<Input id="add-feed-name" bind:value={name} />
				<Field.FieldDescription>Defaults to the publisher's own title.</Field.FieldDescription>
			</Field.FieldSet>

			<Dialog.Footer>
				<Button type="submit" disabled={submitting}>
					{#if submitting}
						<LoaderCircleIcon class="animate-spin" />
					{/if}
					Add Feed
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
