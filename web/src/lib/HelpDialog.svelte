<script lang="ts">
	import { bindings } from '$lib/keys';

	interface Props {
		onClose: () => void;
	}

	const { onClose }: Props = $props();
</script>

<div
	data-testid="help-dialog"
	role="dialog"
	aria-label="Keyboard shortcuts"
	class="fixed inset-0 z-30 flex items-center justify-center bg-black/30"
>
	<div class="flex w-full max-w-sm flex-col gap-4 rounded bg-white p-6 dark:bg-neutral-950">
		<div class="flex items-center justify-between gap-4">
			<h2 class="text-lg font-semibold">Keyboard shortcuts</h2>
			<button
				type="button"
				aria-label="Close"
				onclick={onClose}
				class="rounded border border-neutral-300 px-2 py-1 text-sm dark:border-neutral-700"
			>
				Close
			</button>
		</div>

		<dl class="flex flex-col gap-2 text-sm">
			{#each bindings as binding (binding.key)}
				<div class="flex items-center justify-between gap-4">
					<dt>
						<kbd
							class="rounded border border-neutral-300 bg-neutral-100 px-1.5 py-0.5 font-mono text-xs dark:border-neutral-700 dark:bg-neutral-900"
							>{binding.key}</kbd
						>
					</dt>
					<dd class="text-neutral-600 dark:text-neutral-400">{binding.description}</dd>
				</div>
			{/each}
		</dl>
	</div>
</div>
