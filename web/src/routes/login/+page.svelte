<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { ApiError, logIn } from '$lib/api';

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
		<label class="flex flex-col gap-1 text-sm" for="password">
			Password
			<input
				id="password"
				name="password"
				type="password"
				autocomplete="current-password"
				required
				bind:value={password}
				class="rounded border border-neutral-300 px-3 py-2 text-base dark:border-neutral-700 dark:bg-neutral-900"
			/>
		</label>

		<button
			type="submit"
			disabled={signingIn}
			class="rounded bg-neutral-900 px-3 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-neutral-100 dark:text-neutral-900"
		>
			Sign in
		</button>

		{#if error}
			<p role="alert" class="text-sm text-red-600 dark:text-red-400">{error}</p>
		{/if}
	</form>
</main>
