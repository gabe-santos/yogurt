<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { ApiError, logIn } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import * as Alert from '$lib/components/ui/alert';

	let password = $state('');
	let error = $state('');
	let signingIn = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		signingIn = true;
		try {
			await logIn(password);
			password = '';
			await invalidateAll();
			await goto('/');
		} catch (cause) {
			error = cause instanceof ApiError ? cause.message : 'Could not reach the server';
		} finally {
			signingIn = false;
		}
	}
</script>

<main class="mx-auto flex min-h-dvh max-w-sm flex-col justify-center gap-6 p-8">
	<h1 class="text-2xl font-semibold">Reader</h1>

	<form class="flex flex-col gap-3" onsubmit={submit}>
		<Field.FieldGroup>
			<Field.Field>
				<Field.FieldLabel for="password">Password</Field.FieldLabel>
				<Input
					id="password"
					name="password"
					type="password"
					autocomplete="current-password"
					required
					bind:value={password}
				/>
			</Field.Field>
		</Field.FieldGroup>

		<Button type="submit" disabled={signingIn}>Sign in</Button>

		{#if error}
			<Alert.Root variant="destructive">
				<Alert.Description>{error}</Alert.Description>
			</Alert.Root>
		{/if}
	</form>
</main>
