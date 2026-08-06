<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import type { User } from '$lib/types';

	let users = $state<User[]>([]);
	let loading = $state(false);
	let error = $state('');
	let search = $state('');
	let busyId = $state<number | null>(null);
	let loaded = $state(false);

	// Client-side guard (server enforces admin on every /api/admin call too).
	$effect(() => {
		if ($authLoading) return;
		if (!$currentUser || !$currentUser.isAdmin) {
			goto('/', { replaceState: true });
			return;
		}
		if (!loaded) {
			loaded = true;
			load();
		}
	});

	async function load() {
		loading = true;
		error = '';
		try {
			users = await api.adminUsers(search.trim());
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to load users.';
		} finally {
			loading = false;
		}
	}

	function onSearch(e: SubmitEvent) {
		e.preventDefault();
		load();
	}

	async function ban(u: User) {
		if (!confirm(`Ban ${u.characterName || u.discordUsername}? They will be logged out and blocked from the site.`))
			return;
		await setBanned(u, true);
	}

	async function unban(u: User) {
		await setBanned(u, false);
	}

	async function setBanned(u: User, banned: boolean) {
		busyId = u.id;
		error = '';
		try {
			if (banned) await api.adminBan(u.id);
			else await api.adminUnban(u.id);
			u.banned = banned;
			users = [...users];
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Action failed.';
		} finally {
			busyId = null;
		}
	}

	function fmtDate(iso: string): string {
		return new Date(iso).toLocaleDateString();
	}
</script>

<div class="container">
	<a class="muted back" href="/settings">← Back</a>
	<h1>Admin</h1>
	<p class="muted">Manage site access. Banned accounts are logged out and blocked from the website.</p>

	<form class="search" onsubmit={onSearch}>
		<input
			type="text"
			placeholder="Search Discord username or character name…"
			bind:value={search}
			autocomplete="off"
		/>
		<button class="btn" type="submit" disabled={loading}>Search</button>
	</form>

	{#if error}<p class="error">{error}</p>{/if}

	<div class="card table-wrap">
		{#if loading}
			<p class="muted small pad">Loading…</p>
		{:else if users.length === 0}
			<p class="muted small pad">No users found.</p>
		{:else}
			<table>
				<thead>
					<tr>
						<th>User</th>
						<th>Discord ID</th>
						<th>Joined</th>
						<th>Status</th>
						<th class="actions-col"></th>
					</tr>
				</thead>
				<tbody>
					{#each users as u (u.id)}
						<tr class:banned={u.banned}>
							<td>
								<div class="user-cell">
									{#if u.discordAvatar}<img class="avatar" src={u.discordAvatar} alt="" />{/if}
									<div>
										<strong>{u.characterName || u.discordUsername}</strong>
										{#if u.characterName}<div class="muted small">{u.discordUsername}</div>{/if}
										{#if u.isAdmin}<span class="badge admin">admin</span>{/if}
									</div>
								</div>
							</td>
							<td class="mono muted small">{u.discordId}</td>
							<td class="muted small">{fmtDate(u.createdAt)}</td>
							<td>
								{#if u.banned}<span class="badge danger">banned</span>{:else}<span class="muted small">active</span>{/if}
							</td>
							<td class="actions-col">
								{#if u.isAdmin || u.id === $currentUser?.id}
									<span class="muted small">—</span>
								{:else if u.banned}
									<button class="btn btn-sm" disabled={busyId === u.id} onclick={() => unban(u)}>
										Unban
									</button>
								{:else}
									<button class="btn btn-sm btn-danger" disabled={busyId === u.id} onclick={() => ban(u)}>
										Ban
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</div>

<style>
	.back {
		display: inline-block;
		margin-bottom: 0.5rem;
		font-size: 0.85rem;
	}
	.search {
		display: flex;
		gap: 0.5rem;
		margin: 1rem 0;
	}
	.search input {
		flex: 1;
	}
	.table-wrap {
		padding: 0;
		overflow-x: auto;
	}
	.pad {
		padding: 1rem;
	}
	table {
		width: 100%;
		border-collapse: collapse;
	}
	th,
	td {
		text-align: left;
		padding: 0.6rem 0.9rem;
		border-bottom: 1px solid var(--border);
		vertical-align: middle;
	}
	th {
		font-size: 0.8rem;
		color: var(--text-dim);
		font-weight: 600;
	}
	tbody tr:last-child td {
		border-bottom: none;
	}
	tr.banned {
		opacity: 0.6;
	}
	.user-cell {
		display: flex;
		align-items: center;
		gap: 0.6rem;
	}
	.avatar {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		flex: none;
	}
	.mono {
		font-family: ui-monospace, monospace;
	}
	.small {
		font-size: 0.85rem;
	}
	.actions-col {
		text-align: right;
		white-space: nowrap;
	}
	.badge {
		display: inline-block;
		font-size: 0.7rem;
		font-weight: 600;
		padding: 0.05rem 0.4rem;
		border-radius: 6px;
		background: var(--bg-elev-2);
		color: var(--text-dim);
		margin-top: 0.15rem;
	}
	.badge.admin {
		background: var(--accent);
		color: var(--bg);
	}
	.badge.danger {
		background: var(--danger, #c0392b);
		color: #fff;
	}
</style>
