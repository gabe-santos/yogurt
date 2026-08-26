<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";

	export const tabsListVariants = tv({
		base: "rounded-2xl p-[3px] group-data-horizontal/tabs:h-8 group-data-vertical/tabs:p-1 data-[variant=line]:rounded-none group/tabs-list inline-flex w-fit items-center justify-center text-muted-foreground group-data-[orientation=vertical]/tabs:h-fit group-data-[orientation=vertical]/tabs:flex-col",
		variants: {
			variant: {
				default: "cn-tabs-list-variant-default bg-muted",
				line: "cn-tabs-list-variant-line gap-1 bg-transparent",
			},
		},
		defaultVariants: {
			variant: "default",
		},
	});

	export type TabsListVariant = VariantProps<typeof tabsListVariants>["variant"];
</script>

<script lang="ts">
	import { Tabs as TabsPrimitive } from "bits-ui";
	import { prefersReducedMotion, Spring } from "svelte/motion";
	import { cn } from "$lib/utils.js";

	let {
		ref = $bindable(null),
		variant = "default",
		class: className,
		...restProps
	}: TabsPrimitive.ListProps & {
		variant?: TabsListVariant;
	} = $props();

	// A filter switch is high-frequency, so the indicator travels briefly. With
	// damping 1 the Spring's per-frame recurrence (v = (1 - damping) * v +
	// stiffness * delta, dt = 1 frame at 60fps) collapses to a pure exponential:
	// no overshoot at all, 95% of the distance in ~180ms, the tail sub-pixel.
	// Bounce is 0 by construction — the indicator never rings.
	const indicator = new Spring(
		{ x: 0, y: 0, width: 0, height: 0 },
		{ stiffness: 0.25, damping: 1 },
	);
	let indicatorReady = $state(false);

	function measure(instant: boolean) {
		if (!ref) return;
		const active = ref.querySelector<HTMLElement>(
			'[data-slot="tabs-trigger"][data-state="active"]',
		);
		if (!active) return;
		const listRect = ref.getBoundingClientRect();
		const activeRect = active.getBoundingClientRect();
		indicator.set(
			{
				x: activeRect.left - listRect.left,
				y: activeRect.top - listRect.top,
				width: activeRect.width,
				height: activeRect.height,
			},
			{ instant: instant || prefersReducedMotion.current },
		);
		indicatorReady = true;
	}

	$effect(() => {
		if (!ref) return;

		measure(true);

		const mutationObserver = new MutationObserver(() => measure(false));
		mutationObserver.observe(ref, {
			attributes: true,
			attributeFilter: ["data-state"],
			subtree: true,
		});

		const resizeObserver = new ResizeObserver(() => measure(true));
		resizeObserver.observe(ref);

		return () => {
			mutationObserver.disconnect();
			resizeObserver.disconnect();
		};
	});
</script>

<div class={cn("relative inline-flex")}>
	{#if variant === "default" && indicatorReady}
		<div
			class="pointer-events-none absolute rounded-[calc(var(--radius)*1.8_-_3px)] border border-transparent bg-background group-data-vertical/tabs:rounded-[calc(var(--radius)*1.8_-_4px)] dark:border-input dark:bg-input/30"
			style:transform="translate({indicator.current.x}px, {indicator.current.y}px)"
			style:width="{indicator.current.width}px"
			style:height="{indicator.current.height}px"
		></div>
	{/if}
	<TabsPrimitive.List
		bind:ref
		data-slot="tabs-list"
		data-variant={variant}
		class={cn(tabsListVariants({ variant }), className)}
		{...restProps}
	/>
</div>
