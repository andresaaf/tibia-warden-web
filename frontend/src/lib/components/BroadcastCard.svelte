<script lang="ts">
	import { api, ApiError } from '$lib/api';
	import type { Announcement } from '$lib/types';

	let {
		announcements,
		meId,
		alreadyKilled = false,
		onactionerror,
		onclaimed
	}: {
		announcements: Announcement[];
		meId: number | undefined;
		alreadyKilled?: boolean;
		onactionerror?: (msg: string) => void;
		onclaimed?: (creatureId: number) => void;
	} = $props();

	let primary = $derived(announcements[0]);
	let killed = $derived(announcements.some((a) => a.status === 'killed'));
	let sorted = $derived(
		[...announcements].sort((a, b) => (a.groupName ?? '').localeCompare(b.groupName ?? ''))
	);

	function namesByStatus(a: Announcement, status: string): string[] {
		return a.responses.filter((r) => r.status === status).map((r) => r.characterName);
	}
	function claimNames(a: Announcement): string[] {
		return a.claims.map((c) => c.characterName);
	}

	function myStatus(): string | null {
		for (const a of announcements) {
			const r = a.responses.find((x) => x.userId === meId);
			if (r) return r.status;
		}
		return null;
	}
	let hasClaimed = $derived(announcements.some((a) => a.claims.some((c) => c.userId === meId)));
	let canKill = $derived(
		announcements.some(
			(a) => a.authorId === meId || a.viewerRole === 'owner' || a.viewerRole === 'admin'
		)
	);

	function fail(err: unknown) {
		onactionerror?.(err instanceof ApiError ? err.message : 'Something went wrong.');
	}

	async function respond(status: 'coming' | 'ready') {
		const clear = myStatus() === status;
		try {
			await Promise.all(
				announcements.map((a) => (clear ? api.clearResponse(a.id) : api.setResponse(a.id, status)))
			);
		} catch (err) {
			fail(err);
		}
	}
	async function markKilled() {
		try {
			await api.markAnnouncementKilled(primary.id);
		} catch (err) {
			fail(err);
		}
	}
	async function claim() {
		try {
			await Promise.all(announcements.map((a) => api.claimAnnouncement(a.id)));
			onclaimed?.(primary.creatureId);
		} catch (err) {
			fail(err);
		}
	}
</script>

<div class="card announcement" class:killed>
	<div class="head">
		{#if primary.creatureImageUrl}
			<img
				class="creature-img"
				src={primary.creatureImageUrl}
				alt=""
				onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
			/>
		{/if}
		<div class="head-text">
			<div class="row" style="gap: 0.5rem; flex-wrap: wrap">
				<strong class="creature-name">{primary.creatureName}</strong>
				{#if killed}
					<span class="badge status-killed">Killed</span>
				{:else}
					<span class="badge status-open">Open</span>
				{/if}
				<span class="badge group-badge">{announcements.length} groups</span>
				{#if alreadyKilled}
					<span class="badge mine" title="You've already killed this Echo Warden">✓ In your list</span>
				{/if}
			</div>
			{#if primary.note}<div class="muted note">{primary.note}</div>{/if}
			<div class="muted small">
				by {primary.authorName} · {new Date(primary.createdAt).toLocaleTimeString()}
			</div>
		</div>
	</div>

	{#if !killed}
		<div class="actions">
			<button class="btn btn-sm" class:on={myStatus() === 'coming'} onclick={() => respond('coming')}>
				🏃 Coming
			</button>
			<button class="btn btn-sm" class:on-ready={myStatus() === 'ready'} onclick={() => respond('ready')}>
				✅ Ready
			</button>
			{#if canKill}
				<button class="btn btn-sm btn-danger" onclick={markKilled}>💀 Killed</button>
			{/if}
		</div>
		<div class="groups">
			{#each sorted as a (a.id)}
				<div class="group-section">
					<div class="group-name">{a.groupName || 'Group'}</div>
					{#if namesByStatus(a, 'coming').length}
						<div class="muted small">Coming: {namesByStatus(a, 'coming').join(', ')}</div>
					{/if}
					{#if namesByStatus(a, 'ready').length}
						<div class="muted small">Ready: {namesByStatus(a, 'ready').join(', ')}</div>
					{/if}
					{#if namesByStatus(a, 'coming').length === 0 && namesByStatus(a, 'ready').length === 0}
						<div class="muted small">No responses yet.</div>
					{/if}
				</div>
			{/each}
		</div>
	{:else}
		<div class="actions">
			<button class="btn btn-sm btn-primary" disabled={hasClaimed} onclick={claim}>
				{hasClaimed ? '✓ On your list' : '➕ I got it — tick my list'}
			</button>
		</div>
		<div class="groups">
			{#each sorted as a (a.id)}
				<div class="group-section">
					<div class="group-name">{a.groupName || 'Group'}</div>
					{#if claimNames(a).length}
						<div class="muted small">Got the kill: {claimNames(a).join(', ')}</div>
					{:else}
						<div class="muted small">No claims yet.</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.announcement.killed {
		opacity: 0.92;
	}
	.head {
		display: flex;
		gap: 0.7rem;
		align-items: flex-start;
	}
	.creature-img {
		width: 40px;
		height: 40px;
		object-fit: contain;
		flex: none;
		image-rendering: pixelated;
	}
	.head-text {
		min-width: 0;
	}
	.creature-name {
		font-size: 1.1rem;
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
	.groups {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		margin-top: 0.75rem;
	}
	.group-section {
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
		border-left: 2px solid var(--border);
		padding-left: 0.6rem;
	}
	.group-name {
		font-weight: 650;
		color: var(--text);
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
	.group-badge {
		color: var(--info);
		border-color: var(--info);
	}
	.mine {
		color: var(--success);
		border-color: var(--success);
	}
</style>
