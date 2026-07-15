<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api';
	import { currentUser, authLoading, loadCurrentUser } from '$lib/stores';

	let name = $state('');
	let saving = $state(false);
	let error = $state('');

	// Guard: require auth; skip onboarding if already named.
	$effect(() => {
		if ($authLoading) return;
		if (!$currentUser) {
			goto('/', { replaceState: true });
		} else if ($currentUser.characterName) {
			goto('/', { replaceState: true });
		} else if (!name) {
			name = $currentUser.discordUsername;
		}
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		const trimmed = name.trim();
		if (!trimmed) {
			error = 'Please enter your character name.';
			return;
		}
		saving = true;
		try {
			await api.updateCharacterName(trimmed);
			await loadCurrentUser();
			goto('/');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong.';
		} finally {
			saving = false;
		}
	}
</script>

<div class="wrap">
	<form class="card" onsubmit={submit}>
		<h1>Welcome!</h1>
		<p class="muted">What is your Tibia character name? This is how your group will recognize you.</p>
		<input
			type="text"
			placeholder="e.g. Bubble the Brave"
			bind:value={name}
			maxlength="60"
			autocomplete="off"
		/>
		{#if error}<p class="error">{error}</p>{/if}
		<button class="btn btn-primary" type="submit" disabled={saving}>
			{saving ? 'Saving…' : 'Continue'}
		</button>
	</form>
</div>

<style>
	.wrap {
		display: flex;
		justify-content: center;
		padding: 4rem 1.5rem;
	}
	.card {
		max-width: 440px;
		width: 100%;
	}
	.card p {
		margin: 0.5rem 0 1.25rem;
	}
	.card input {
		margin-bottom: 1rem;
	}
	.card button {
		width: 100%;
		justify-content: center;
	}
</style>
