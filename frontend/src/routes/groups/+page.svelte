<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import type { Group } from '$lib/types';

	let tab = $state<'mine' | 'public'>('mine');
	let groups = $state<Group[]>([]);
	let search = $state('');
	let loading = $state(true);
	let error = $state('');
	let debounce: ReturnType<typeof setTimeout>;

	// Create-group form.
	let showCreate = $state(false);
	let newName = $state('');
	let newDesc = $state('');
	let newVisibility = $state<'public' | 'private'>('public');
	let creating = $state(false);
	let createError = $state('');

	// Join-by-code.
	let inviteCode = $state('');
	let joining = $state(false);
	let joinError = $state('');

	$effect(() => {
		if (!$authLoading && !$currentUser) goto('/', { replaceState: true });
	});

	onMount(() => {
		load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			groups = await api.listGroups(tab === 'mine' ? 'mine' : 'public', search.trim());
		} catch {
			error = 'Failed to load groups.';
		} finally {
			loading = false;
		}
	}

	function switchTab(next: 'mine' | 'public') {
		tab = next;
		load();
	}

	function onSearchInput() {
		clearTimeout(debounce);
		debounce = setTimeout(load, 250);
	}

	async function createGroup(e: SubmitEvent) {
		e.preventDefault();
		createError = '';
		if (!newName.trim()) {
			createError = 'A group name is required.';
			return;
		}
		creating = true;
		try {
			const group = await api.createGroup(newName.trim(), newDesc.trim(), newVisibility);
			goto(`/groups/${group.id}`);
		} catch (err) {
			createError = err instanceof ApiError ? err.message : 'Failed to create group.';
		} finally {
			creating = false;
		}
	}

	async function joinByCode(e: SubmitEvent) {
		e.preventDefault();
		joinError = '';
		if (!inviteCode.trim()) return;
		joining = true;
		try {
			const { groupId } = await api.redeemInvite(inviteCode.trim());
			goto(`/groups/${groupId}`);
		} catch (err) {
			joinError = err instanceof ApiError ? err.message : 'Failed to join.';
		} finally {
			joining = false;
		}
	}

	async function joinPublic(group: Group) {
		try {
			await api.joinPublic(group.id);
			goto(`/groups/${group.id}`);
		} catch {
			error = 'Failed to join group.';
		}
	}
</script>

<div class="container stack">
	<div class="spread">
		<h1>Groups</h1>
		<button class="btn btn-primary" onclick={() => (showCreate = !showCreate)}>
			{showCreate ? 'Cancel' : '+ New Group'}
		</button>
	</div>

	{#if showCreate}
		<form class="card stack" onsubmit={createGroup}>
			<h3>Create a group</h3>
			<input type="text" placeholder="Group name" bind:value={newName} maxlength="80" />
			<textarea placeholder="Description (optional)" bind:value={newDesc} rows="2"></textarea>
			<div class="row">
				<label class="radio">
					<input type="radio" bind:group={newVisibility} value="public" /> Public
				</label>
				<label class="radio">
					<input type="radio" bind:group={newVisibility} value="private" /> Private (invite only)
				</label>
			</div>
			{#if createError}<p class="error">{createError}</p>{/if}
			<button class="btn btn-primary" type="submit" disabled={creating}>
				{creating ? 'Creating…' : 'Create group'}
			</button>
		</form>
	{/if}

	<form class="card join-row" onsubmit={joinByCode}>
		<input type="text" placeholder="Have an invite code? Paste it here" bind:value={inviteCode} />
		<button class="btn" type="submit" disabled={joining}>{joining ? 'Joining…' : 'Join'}</button>
		{#if joinError}<span class="error">{joinError}</span>{/if}
	</form>

	<div class="tabs">
		<button class="tab" class:active={tab === 'mine'} onclick={() => switchTab('mine')}>
			My Groups
		</button>
		<button class="tab" class:active={tab === 'public'} onclick={() => switchTab('public')}>
			Discover Public
		</button>
	</div>

	{#if tab === 'public'}
		<input type="text" placeholder="Search public groups…" bind:value={search} oninput={onSearchInput} />
	{/if}

	{#if error}
		<p class="error">{error}</p>
	{:else if loading}
		<p class="muted">Loading…</p>
	{:else if groups.length === 0}
		<p class="muted">
			{tab === 'mine'
				? "You haven't joined any groups yet. Create one or discover public groups."
				: 'No public groups found.'}
		</p>
	{:else}
		<div class="stack">
			{#each groups as group (group.id)}
				<div class="card spread group-row">
					<div>
						<div class="row" style="gap: 0.5rem">
							<strong>{group.name}</strong>
							<span class="badge">{group.visibility}</span>
							{#if group.role}<span class="badge role">{group.role}</span>{/if}
						</div>
						{#if group.description}<p class="muted desc">{group.description}</p>{/if}
						<p class="muted small">{group.memberCount} member{group.memberCount === 1 ? '' : 's'}</p>
					</div>
					<div>
						{#if group.role}
							<a class="btn" href={`/groups/${group.id}`}>Open</a>
						{:else}
							<button class="btn btn-primary" onclick={() => joinPublic(group)}>Join</button>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.tabs {
		display: flex;
		gap: 0.3rem;
		border-bottom: 1px solid var(--border);
	}
	.tab {
		background: none;
		border: none;
		color: var(--text-dim);
		padding: 0.6rem 0.9rem;
		font-weight: 600;
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
	}
	.tab.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
	}
	.join-row {
		display: flex;
		gap: 0.6rem;
		align-items: center;
		flex-wrap: wrap;
	}
	.join-row input {
		flex: 1;
		min-width: 200px;
	}
	.radio {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		width: auto;
	}
	.radio input {
		width: auto;
	}
	.desc {
		margin: 0.3rem 0 0.2rem;
	}
	.small {
		font-size: 0.85rem;
		margin: 0;
	}
	.role {
		color: var(--accent);
		border-color: var(--accent);
	}
	.group-row {
		align-items: flex-start;
	}
</style>
