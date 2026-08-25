<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();

	// A colour-scheme flip repaints background, text, border and shadow on nearly
	// every element at once, and every 150ms colour transition in the app fires
	// together: containers land first while rows and labels catch up, so the
	// switch smears instead of snapping. Kill transitions for the swap, force the
	// new colours to be computed, then hand them back a frame later.
	$effect(() => {
		const scheme = window.matchMedia('(prefers-color-scheme: dark)');
		const onChange = () => {
			const stop = document.createElement('style');
			stop.textContent = '*,*::before,*::after{transition:none !important}';
			document.head.append(stop);
			void document.body.offsetHeight;
			requestAnimationFrame(() => stop.remove());
		};
		scheme.addEventListener('change', onChange);
		return () => scheme.removeEventListener('change', onChange);
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Reader</title>
</svelte:head>

<div class="min-h-dvh bg-background text-foreground">
	{@render children()}
</div>
