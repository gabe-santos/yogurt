<script lang="ts">
	import { bindings } from '$lib/keys';
	import * as Dialog from '$lib/components/ui/dialog';

	interface Props {
		onClose: () => void;
		/** gistsEnabled hides the `g` row entirely: GABE-201/203 says nothing
		 * about Gists appears, anywhere, until the Reader configures a model. */
		gistsEnabled: boolean;
	}

	const { onClose, gistsEnabled }: Props = $props();
	const shown = $derived(
		bindings.filter((binding) => binding.action !== 'requestGist' || gistsEnabled),
	);
</script>

<Dialog.Root open onOpenChange={(open) => !open && onClose()}>
	<Dialog.Content data-testid="help-dialog" class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>Keyboard shortcuts</Dialog.Title>
		</Dialog.Header>

		<dl class="flex flex-col gap-2 text-sm">
			{#each shown as binding (binding.key)}
				<div class="flex items-center justify-between gap-4">
					<dt>
						<kbd class="rounded-md border border-border bg-muted px-1.5 py-0.5 font-mono text-xs"
							>{binding.key}</kbd
						>
					</dt>
					<dd class="text-muted-foreground">{binding.description}</dd>
				</div>
			{/each}
		</dl>
	</Dialog.Content>
</Dialog.Root>
