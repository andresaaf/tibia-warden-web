<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { api, ApiError } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import { GroupRoom, type RoomEvent } from '$lib/ws';
	import type { Announcement, Creature, Group, GroupMember, InviteCode } from '$lib/types';

	let groupId = $derived(Number($page.params.id));

	let group = $state<Group | null>(null);
	let announcements = $state<Announcement[]>([]);
	let creatures = $state<Creature[]>([]);
	let members = $state<GroupMember[]>([]);
	let invites = $state<InviteCode[]>([]);
	let loading = $state(true);
	let error = $state('');
	let room: GroupRoom | null = null;

	// New announcement form.
	let creatureId = $state<number | ''>('');
	let location = $state('');
	let note = $state('');
	let posting = $state(false);
	let postError = $state('');
	let creatureQuery = $state('');
	let showCreatureList = $state(false);
	let highlightIndex = $state(0);

	let showAdmin = $state(false);

	let me = $derived($currentUser);
	let isManager = $derived(group?.role === 'owner' || group?.role === 'admin');

	$effect(() => {
		if (!$authLoading && !$currentUser) goto('/', { replaceState: true });
	});

	onMount(() => {
		init();
		return () => room?.close();
	});

	onDestroy(() => room?.close());

	async function init() {
		loading = true;
		error = '';
		try {
			group = await api.getGroup(groupId);
			const [ann, crt] = await Promise.all([
				api.announcements(groupId),
				api.creatures('', [])
			]);
			announcements = ann;
			creatures = crt;
			if (isManager) await loadAdminData();
			connectRoom();
		} catch (err) {
			if (err instanceof ApiError && err.status === 403) {
				error = 'You are not a member of this group.';
			} else if (err instanceof ApiError && err.status === 404) {
				error = 'Group not found.';
			} else {
				error = 'Failed to load the group.';
			}
		} finally {
			loading = false;
		}
	}

	async function loadAdminData() {
		try {
			[members, invites] = await Promise.all([api.members(groupId), api.invites(groupId)]);
		} catch {
			// non-fatal
		}
	}

	function connectRoom() {
		room = new GroupRoom(groupId, handleEvent);
		room.connect();
	}

	function handleEvent(event: RoomEvent) {
		if (event.type === 'announcement.created') {
			upsertAnnouncement(event.payload, true);
		} else if (event.type === 'announcement.updated') {
			upsertAnnouncement(event.payload, false);
		}
	}

	function upsertAnnouncement(a: Announcement, prepend: boolean) {
		const idx = announcements.findIndex((x) => x.id === a.id);
		if (idx >= 0) {
			announcements[idx] = a;
			announcements = [...announcements];
		} else if (prepend) {
			announcements = [a, ...announcements];
		}
	}

	let filteredCreatures = $derived.by(() => {
		const q = creatureQuery.trim().toLowerCase();
		const list = q ? creatures.filter((c) => c.name.toLowerCase().includes(q)) : creatures;
		return list.slice(0, 8);
	});

	function selectCreature(c: Creature) {
		creatureId = c.id;
		creatureQuery = c.name;
		showCreatureList = false;
	}

	function onCreatureInput() {
		// Typing invalidates any previous selection until a new one is chosen.
		creatureId = '';
		showCreatureList = true;
		highlightIndex = 0;
	}

	function onCreatureKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			showCreatureList = true;
			highlightIndex = Math.min(highlightIndex + 1, filteredCreatures.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			highlightIndex = Math.max(highlightIndex - 1, 0);
		} else if (e.key === 'Enter') {
			if (showCreatureList && filteredCreatures[highlightIndex]) {
				e.preventDefault();
				selectCreature(filteredCreatures[highlightIndex]);
			}
		} else if (e.key === 'Escape') {
			showCreatureList = false;
		}
	}

	async function postAnnouncement(e: SubmitEvent) {
		e.preventDefault();
		postError = '';
		if (!creatureId) {
			postError = 'Choose a creature.';
			return;
		}
		posting = true;
		try {
			await api.createAnnouncement(groupId, {
				creatureId: Number(creatureId),
				location: location.trim(),
				note: note.trim(),
				goldCost: 0
			});
			creatureId = '';
			location = '';
			note = '';
			creatureQuery = '';
			showCreatureList = false;
		} catch (err) {
			postError = err instanceof ApiError ? err.message : 'Failed to post.';
		} finally {
			posting = false;
		}
	}

	function myResponse(a: Announcement): string | null {
		return a.responses.find((r) => r.userId === me?.id)?.status ?? null;
	}
	function hasClaimed(a: Announcement): boolean {
		return a.claims.some((c) => c.userId === me?.id);
	}

	async function respond(a: Announcement, status: 'coming' | 'ready') {
		try {
			if (myResponse(a) === status) await api.clearResponse(a.id);
			else await api.setResponse(a.id, status);
		} catch {
			/* live update will reconcile */
		}
	}
	async function markKilled(a: Announcement) {
		try {
			await api.markAnnouncementKilled(a.id);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to mark killed.';
		}
	}
	async function claim(a: Announcement) {
		try {
			await api.claimAnnouncement(a.id);
		} catch {
			/* live update will reconcile */
		}
	}

	async function leave() {
		if (!confirm('Leave this group?')) return;
		try {
			await api.leaveGroup(groupId);
			goto('/groups');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to leave group.';
		}
	}

	async function createInvite() {
		try {
			await api.createInvite(groupId);
			invites = await api.invites(groupId);
		} catch {
			error = 'Failed to create invite.';
		}
	}
	async function setRole(userId: number, role: string) {
		try {
			await api.setRole(groupId, userId, role);
			members = await api.members(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to update role.';
		}
	}
	async function kick(userId: number) {
		if (!confirm('Remove this member?')) return;
		try {
			await api.removeMember(groupId, userId);
			members = await api.members(groupId);
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to remove member.';
		}
	}

	function copyCode(code: string) {
		navigator.clipboard?.writeText(code);
	}

	function comingList(a: Announcement) {
		return a.responses.filter((r) => r.status === 'coming');
	}
	function readyList(a: Announcement) {
		return a.responses.filter((r) => r.status === 'ready');
	}
</script>

<div class="container">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else if error && !group}
		<div class="card">
			<p class="error">{error}</p>
			<a class="btn" href="/groups">Back to groups</a>
		</div>
	{:else if group}
		<div class="spread header">
			<div>
				<a class="muted back" href="/groups">← Groups</a>
				<h1>{group.name}</h1>
				{#if group.description}<p class="muted">{group.description}</p>{/if}
			</div>
			<div class="row">
				{#if isManager}
					<button class="btn btn-sm" onclick={() => (showAdmin = !showAdmin)}>
						{showAdmin ? 'Hide' : 'Manage'}
					</button>
				{/if}
				{#if group.role !== 'owner'}
					<button class="btn btn-sm btn-danger" onclick={leave}>Leave</button>
				{/if}
			</div>
		</div>

		{#if error}<p class="error">{error}</p>{/if}

		{#if showAdmin && isManager}
			<div class="card stack admin">
				<div class="spread">
					<h3>Members</h3>
					<button class="btn btn-sm" onclick={createInvite}>+ Create invite code</button>
				</div>

				{#if invites.length}
					<div class="invites">
						{#each invites as inv (inv.id)}
							<div class="invite" class:used={!!inv.usedBy}>
								<code>{inv.code}</code>
								{#if inv.usedBy}
									<span class="badge">used</span>
								{:else}
									<button class="btn btn-sm" onclick={() => copyCode(inv.code)}>Copy</button>
								{/if}
							</div>
						{/each}
					</div>
				{/if}

				<div class="stack member-list">
					{#each members as m (m.userId)}
						<div class="spread member">
							<div>
								<strong>{m.characterName || m.discordName}</strong>
								<span class="badge">{m.role}</span>
							</div>
							{#if m.role !== 'owner'}
								<div class="row">
									{#if group.role === 'owner'}
										{#if m.role === 'admin'}
											<button class="btn btn-sm" onclick={() => setRole(m.userId, 'member')}>
												Demote
											</button>
										{:else}
											<button class="btn btn-sm" onclick={() => setRole(m.userId, 'admin')}>
												Make admin
											</button>
										{/if}
									{/if}
									<button class="btn btn-sm btn-danger" onclick={() => kick(m.userId)}>Kick</button>
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<form class="card stack post-form" onsubmit={postAnnouncement}>
			<h3>Announce an Echo Warden</h3>
			<div class="post-grid">
				<div class="combobox">
					<input
						type="text"
						placeholder="Search creature…"
						bind:value={creatureQuery}
						autocomplete="off"
						oninput={onCreatureInput}
						onfocus={() => (showCreatureList = true)}
						onblur={() => setTimeout(() => (showCreatureList = false), 120)}
						onkeydown={onCreatureKeydown}
					/>
					{#if showCreatureList && (filteredCreatures.length > 0 || creatureQuery.trim())}
						<div class="combobox-list">
							{#each filteredCreatures as c, i (c.id)}
								<button
									type="button"
									class="opt"
									class:highlight={i === highlightIndex}
									onclick={() => selectCreature(c)}
									onmousemove={() => (highlightIndex = i)}
								>
									<span class="opt-name">{c.name}</span>
									<span class="badge diff" data-diff={c.difficulty}>{c.difficulty}</span>
								</button>
							{/each}
							{#if filteredCreatures.length === 0}
								<div class="opt empty muted">No creatures match</div>
							{/if}
						</div>
					{/if}
				</div>
				<input type="text" placeholder="Location / spawn" bind:value={location} />
			</div>
			<input type="text" placeholder="Note (optional)" bind:value={note} />
			{#if postError}<p class="error">{postError}</p>{/if}
			<button class="btn btn-primary" type="submit" disabled={posting}>
				{posting ? 'Posting…' : 'Post announcement'}
			</button>
		</form>

		<div class="stack feed">
			{#if announcements.length === 0}
				<p class="muted">No announcements yet. Be the first to spot an Echo Warden!</p>
			{/if}
			{#each announcements as a (a.id)}
				<div class="card announcement" class:killed={a.status === 'killed'}>
					<div class="spread">
						<div>
							<div class="row" style="gap: 0.5rem">
								<strong class="creature-name">{a.creatureName}</strong>
								{#if a.status === 'killed'}
									<span class="badge status-killed">Killed</span>
								{:else}
									<span class="badge status-open">Open</span>
								{/if}
							</div>
							{#if a.location}<div class="loc">📍 {a.location}</div>{/if}
							{#if a.note}<div class="muted note">{a.note}</div>{/if}
							<div class="muted small">
								by {a.authorName} · {new Date(a.createdAt).toLocaleTimeString()}
							</div>
						</div>
					</div>

					{#if a.status === 'open'}
						<div class="actions">
							<button
								class="btn btn-sm"
								class:on={myResponse(a) === 'coming'}
								onclick={() => respond(a, 'coming')}
							>
								🏃 Coming
							</button>
							<button
								class="btn btn-sm"
								class:on-ready={myResponse(a) === 'ready'}
								onclick={() => respond(a, 'ready')}
							>
								✅ Ready
							</button>
							{#if a.authorId === me?.id}
								<button class="btn btn-sm btn-danger" onclick={() => markKilled(a)}>
									💀 Killed
								</button>
							{/if}
						</div>

						<div class="responders">
							{#if comingList(a).length}
								<span class="muted small">Coming: {comingList(a).map((r) => r.characterName).join(', ')}</span>
							{/if}
							{#if readyList(a).length}
								<span class="muted small">Ready: {readyList(a).map((r) => r.characterName).join(', ')}</span>
							{/if}
						</div>
					{:else}
						<div class="actions">
							<button class="btn btn-sm btn-primary" disabled={hasClaimed(a)} onclick={() => claim(a)}>
								{hasClaimed(a) ? '✓ On your list' : '➕ I got it — tick my list'}
							</button>
						</div>
						<div class="responders">
							{#if a.claims.length}
								<span class="muted small">
									Got the kill: {a.claims.map((c) => c.characterName).join(', ')}
								</span>
							{:else}
								<span class="muted small">No one has claimed the benefit yet.</span>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.header {
		align-items: flex-start;
		margin-bottom: 1rem;
	}
	.back {
		display: inline-block;
		margin-bottom: 0.3rem;
		font-size: 0.85rem;
	}
	.admin {
		margin-bottom: 1rem;
	}
	.invites {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.invite {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.3rem 0.5rem;
	}
	.invite.used code {
		text-decoration: line-through;
		color: var(--text-dim);
	}
	.invite code {
		font-family: ui-monospace, monospace;
		letter-spacing: 0.05em;
	}
	.member-list {
		gap: 0.4rem;
	}
	.member {
		padding: 0.4rem 0;
		border-bottom: 1px solid var(--border);
	}
	.post-form {
		margin-bottom: 1.25rem;
	}
	.post-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.6rem;
	}
	.combobox {
		position: relative;
	}
	.combobox-list {
		position: absolute;
		z-index: 20;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow);
		max-height: 260px;
		overflow-y: auto;
		padding: 4px;
	}
	.opt {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		width: 100%;
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 0.45rem 0.55rem;
		border-radius: 6px;
	}
	.opt.highlight {
		background: var(--bg-elev);
	}
	.opt.empty {
		cursor: default;
	}
	.opt-name {
		font-weight: 550;
	}
	.diff[data-diff='Harmless'] {
		color: var(--diff-harmless);
		border-color: var(--diff-harmless);
	}
	.diff[data-diff='Trivial'] {
		color: var(--diff-trivial);
		border-color: var(--diff-trivial);
	}
	.diff[data-diff='Easy'] {
		color: var(--diff-easy);
		border-color: var(--diff-easy);
	}
	.diff[data-diff='Medium'] {
		color: var(--diff-medium);
		border-color: var(--diff-medium);
	}
	.diff[data-diff='Hard'] {
		color: var(--diff-hard);
		border-color: var(--diff-hard);
	}
	.diff[data-diff='Challenging'] {
		color: var(--diff-challenging);
		border-color: var(--diff-challenging);
	}
	.feed {
		gap: 0.7rem;
	}
	.announcement.killed {
		border-color: var(--border);
		opacity: 0.92;
	}
	.creature-name {
		font-size: 1.1rem;
	}
	.loc {
		margin-top: 0.3rem;
	}
	.note {
		margin-top: 0.15rem;
	}
	.small {
		font-size: 0.82rem;
	}
	.actions {
		display: flex;
		gap: 0.4rem;
		margin-top: 0.75rem;
		flex-wrap: wrap;
	}
	.responders {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		margin-top: 0.5rem;
	}
	.btn.on {
		border-color: var(--info);
		color: var(--info);
		background: color-mix(in srgb, var(--info) 15%, var(--bg-elev-2));
	}
	.btn.on-ready {
		border-color: var(--success);
		color: var(--success);
		background: color-mix(in srgb, var(--success) 15%, var(--bg-elev-2));
	}
	.status-open {
		color: var(--accent);
		border-color: var(--accent);
	}
	.status-killed {
		color: var(--danger);
		border-color: var(--danger);
	}
</style>
