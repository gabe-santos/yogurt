<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Reader</title>
</svelte:head>

<div class="min-h-dvh bg-white text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
	{@render children()}
</div>
