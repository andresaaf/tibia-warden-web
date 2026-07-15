<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api';
	import { currentUser, authLoading, loadCurrentUser } from '$lib/stores';

	let name = $state('');
	let saving = $state(false);
	let error = $state('');
	let saved = $state(false);
	let initialised = $state(false);

	$effect(() => {
		if ($authLoading) return;
		if (!$currentUser) {
			goto('/', { replaceState: true });
		} else if (!initialised) {
			name = $currentUser.characterName;
			initialised = true;
		}
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		saved = false;
		const trimmed = name.trim();
		if (!trimmed) {
			error = 'Please enter your character name.';
			return;
		}
		saving = true;
		try {
			await api.updateCharacterName(trimmed);
			await loadCurrentUser();
			saved = true;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Something went wrong.';
		} finally {
			saving = false;
		}
	}
</script>

<div class="container">
	<a class="muted back" href="/groups">← Back</a>
	<h1>Account settings</h1>

	{#if $currentUser}
		<div class="card stack">
			<div class="row profile">
				{#if $currentUser.discordAvatar}
					<img class="avatar" src={$currentUser.discordAvatar} alt="" />
				{/if}
				<div>
					<strong>{$currentUser.discordUsername}</strong>
					<div class="muted small">Signed in with Discord</div>
				</div>
			</div>

			<form class="stack" onsubmit={submit}>
				<label class="field">
					<span class="muted small">Tibia character name</span>
					<input
						type="text"
						placeholder="e.g. Bubble the Brave"
						bind:value={name}
						maxlength="60"
						autocomplete="off"
						oninput={() => {
							saved = false;
							error = '';
						}}
					/>
				</label>
				{#if error}<p class="error">{error}</p>{/if}
				{#if saved}<p class="success">Saved.</p>{/if}
				<div class="row">
					<button class="btn btn-primary" type="submit" disabled={saving}>
						{saving ? 'Saving…' : 'Save'}
					</button>
				</div>
			</form>
		</div>
	{/if}
</div>

<style>
	.back {
		display: inline-block;
		margin-bottom: 0.5rem;
		font-size: 0.85rem;
	}
	.profile {
		gap: 0.75rem;
		padding-bottom: 0.75rem;
		border-bottom: 1px solid var(--border);
	}
	.avatar {
		width: 44px;
		height: 44px;
		border-radius: 50%;
		flex: none;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}
	.small {
		font-size: 0.85rem;
	}
	.success {
		color: var(--success);
		margin: 0;
	}
</style>
