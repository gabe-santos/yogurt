<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';

	interface Props {
		title: string;
		/** description says what the action removes, in the reader's own terms,
		 * so the consequence is on screen rather than implied by the verb. */
		description: string;
		confirmLabel: string;
		onConfirm: () => void;
		onCancel: () => void;
	}

	const { title, description, confirmLabel, onConfirm, onCancel }: Props = $props();
</script>

<!-- The app's one irreversible act, asked in the app's own voice rather than
     in the browser's. Cancel is the wide, quiet default; the destructive
     choice is the one that carries the system's single chromatic token. -->
<Dialog.Root open onOpenChange={(open) => !open && onCancel()}>
	<Dialog.Content data-testid="confirm-dialog" class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title class="text-lg font-semibold tracking-lg">{title}</Dialog.Title>
			<Dialog.Description>{description}</Dialog.Description>
		</Dialog.Header>

		<Dialog.Footer>
			<Button variant="outline" onclick={onCancel}>Cancel</Button>
			<Button variant="destructive" data-testid="confirm-dialog-confirm" onclick={onConfirm}>
				{confirmLabel}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
