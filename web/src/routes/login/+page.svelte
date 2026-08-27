<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { describeError, logIn } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Field from '$lib/components/ui/field';
	import * as Alert from '$lib/components/ui/alert';

	let password = $state('');
	let error = $state('');
	let signingIn = $state(false);
	let passwordInput = $state<HTMLInputElement | null>(null);

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
			error = describeError(cause, 'Could not reach the server');
			passwordInput?.focus();
		} finally {
			signingIn = false;
		}
	}
</script>

<main class="mx-auto flex min-h-dvh max-w-sm flex-col justify-center gap-6 p-8">
	<h1 class="enter enter-1 text-2xl font-semibold">Reader</h1>

	<form class="flex flex-col gap-3" onsubmit={submit}>
		<div class="enter enter-2 flex flex-col">
			<Field.FieldGroup>
				<Field.Field>
					<Field.FieldLabel for="password">Password</Field.FieldLabel>
					<Input
						id="password"
						name="password"
						type="password"
						autocomplete="current-password"
						required
						aria-invalid={error !== ''}
						aria-describedby={error !== '' ? 'login-error' : undefined}
						bind:ref={passwordInput}
						bind:value={password}
					/>
				</Field.Field>
			</Field.FieldGroup>
		</div>

		<div class="enter enter-3 flex flex-col">
			<Button type="submit" disabled={signingIn}>Sign in</Button>
		</div>

		{#if error}
			<Alert.Root variant="destructive">
				<Alert.Description id="login-error">{error}</Alert.Description>
			</Alert.Root>
		{/if}
	</form>
</main>

<!-- The one page in the app that is a staged arrival rather than a working
     surface: title, then the field, then the button, so the sequence says what
     to do in the order it wants doing. A single container animating as one
     would cost the same and say less. -->
<style>
	@keyframes enter {
		from {
			opacity: 0;
			translate: 0 12px;
			filter: blur(4px);
		}
	}

	.enter {
		animation: enter 400ms cubic-bezier(0.2, 0, 0, 1) both;
	}

	.enter-1 {
		animation-delay: 0ms;
	}

	.enter-2 {
		animation-delay: 100ms;
	}

	.enter-3 {
		animation-delay: 200ms;
	}

	@media (prefers-reduced-motion: reduce) {
		.enter {
			animation: none;
		}
	}
</style>
