<script lang="ts">
	import { Dialog as DialogPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		...restProps
	}: DialogPrimitive.OverlayProps = $props();
</script>

<DialogPrimitive.Overlay
	bind:ref
	data-slot="dialog-overlay"
	class={cn(
		// fade-in/out is gated motion-safe: rerunning the shadcn-svelte
		// generator for this vendored primitive will drop that gate.
		"bg-black/30 duration-100 supports-backdrop-filter:backdrop-blur-sm motion-safe:data-open:animate-in motion-safe:data-open:fade-in-0 motion-safe:data-closed:animate-out motion-safe:data-closed:fade-out-0 fixed inset-0 isolate z-50",
		className
	)}
	{...restProps}
/>
