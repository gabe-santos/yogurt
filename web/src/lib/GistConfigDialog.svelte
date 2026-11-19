<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
	import { clearAIConfig, describeError, getAIConfig, setAIConfig } from '$lib/api';
	import { onMount } from 'svelte';

	interface Props {
		onClose: () => void;
	}

	const { onClose }: Props = $props();

	let url = $state('');
	let model = $state('');
	// The key field is always blank: a stored key is never shown back, only
	// reported through hasKey below. Typing here replaces it; leaving it
	// blank on save keeps whatever is stored.
	let key = $state('');
	let hasKey = $state(false);
	let loaded = $state(false);
	let submitting = $state(false);
	let clearing = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			const config = await getAIConfig();
			url = config.url;
			model = config.model;
			hasKey = config.has_key;
		} catch (cause) {
			error = describeError(cause, 'Could not load your Gist configuration');
		} finally {
			loaded = true;
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (submitting || clearing) return;
		submitting = true;
		error = '';
		try {
			const config = await setAIConfig(
				key ? { url, model, key } : { url, model },
			);
			url = config.url;
			model = config.model;
			hasKey = config.has_key;
			key = '';
		} catch (cause) {
			error = describeError(cause, 'Could not update your Gist configuration');
		} finally {
			submitting = false;
		}
	}

	async function turnOff() {
		if (submitting || clearing) return;
		clearing = true;
		error = '';
		try {
			await clearAIConfig();
			url = '';
			model = '';
			key = '';
			hasKey = false;
		} catch (cause) {
			error = describeError(cause, 'Could not clear your Gist configuration');
		} finally {
			clearing = false;
		}
	}

</script>

<Dialog.Root open onOpenChange={(open) => !open && onClose()}>
	<Dialog.Content data-testid="gist-config-dialog" class="sm:max-w-sm">
		<Dialog.Header>
			<Dialog.Title>Gists</Dialog.Title>
		</Dialog.Header>

		<p class="text-sm text-muted-foreground">
			Configure an OpenAI-compatible language model to turn on Gists: a short condensation of an
			Entry's Article, made only when you ask for it.
		</p>

		<p class="text-sm text-muted-foreground">
			With a hosted provider, the Article text leaves this Instance when you request a Gist; a
			local model such as Ollama or LM Studio avoids that.
		</p>

		{#if error}
			<p class="text-sm text-destructive">{error}</p>
		{/if}

		<form class="flex flex-col gap-4" onsubmit={submit}>
			<Field.FieldSet>
				<Field.FieldLabel for="ai-url">Endpoint URL</Field.FieldLabel>
				<Input
					id="ai-url"
					type="url"
					placeholder="https://api.openai.com/v1/chat/completions"
					bind:value={url}
					disabled={!loaded}
				/>
			</Field.FieldSet>

			<Field.FieldSet>
				<Field.FieldLabel for="ai-model">Model</Field.FieldLabel>
				<Input id="ai-model" placeholder="gpt-4o-mini" bind:value={model} disabled={!loaded} />
			</Field.FieldSet>

			<Field.FieldSet>
				<Field.FieldLabel for="ai-key">API key</Field.FieldLabel>
				<Input
					id="ai-key"
					type="password"
					placeholder={hasKey ? 'Stored — leave blank to keep it' : 'Optional, for local models'}
					bind:value={key}
					disabled={!loaded}
				/>
				<Field.FieldDescription>
					{hasKey ? 'An API key is stored.' : 'No API key is stored.'}
				</Field.FieldDescription>
			</Field.FieldSet>

			<Dialog.Footer class="justify-between sm:justify-between">
				<Button
					type="button"
					variant="outline"
					disabled={!loaded || submitting || clearing}
					onclick={turnOff}
				>
					{#if clearing}
						<LoaderCircleIcon class="animate-spin" />
					{/if}
					Turn off Gists
				</Button>
				<Button type="submit" disabled={!loaded || submitting || clearing}>
					{#if submitting}
						<LoaderCircleIcon class="animate-spin" />
					{/if}
					Save
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
