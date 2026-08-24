<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { logOut } from '$lib/api';

	let signingOut = $state(false);

	async function signOut() {
		signingOut = true;
		try {
			await logOut();
			await invalidateAll();
			await goto('/login');
		} finally {
			signingOut = false;
		}
	}
</script>

<main class="mx-auto flex max-w-2xl flex-col gap-6 p-8">
	<header class="flex items-center justify-between gap-4">
		<h1 class="text-2xl font-semibold">Reader</h1>
		<button
			type="button"
			class="rounded border border-neutral-300 px-3 py-1.5 text-sm hover:bg-neutral-100 disabled:opacity-50 dark:border-neutral-700 dark:hover:bg-neutral-800"
			onclick={signOut}
			disabled={signingOut}
		>
			Sign out
		</button>
	</header>

	<p data-testid="signed-in" class="text-neutral-600 dark:text-neutral-400">
		Signed in. No Feeds yet.
	</p>
</main>
