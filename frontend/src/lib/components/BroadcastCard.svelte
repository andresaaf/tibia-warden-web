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

	type Person = { userId: number; characterName: string; group?: string };

	// Merge responders across all sibling groups; ready beats coming per person.
	let responders = $derived.by(() => {
		const map = new Map<number, { p: Person; status: string }>();
		for (const a of announcements) {
			for (const r of a.responses) {
				const cur = map.get(r.userId);
				if (!cur || (r.status === 'ready' && cur.status === 'coming')) {
					map.set(r.userId, {
						p: { userId: r.userId, characterName: r.characterName, group: a.groupName },
						status: r.status
					});
				}
			}
		}
		const coming: Person[] = [];
		const ready: Person[] = [];
		for (const { p, status } of map.values()) (status === 'ready' ? ready : coming).push(p);
		return { coming, ready };
	});

	let claimers = $derived.by(() => {
		const map = new Map<number, Person>();
		for (const a of announcements) {
			for (const c of a.claims) {
				if (!map.has(c.userId))
					map.set(c.userId, { userId: c.userId, characterName: c.characterName, group: a.groupName });
			}
		}
		return [...map.values()];
	});

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

	function label(p: Person): string {
		return p.group ? `${p.characterName} (${p.group})` : p.characterName;
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
				by {primary.authorName} · {new Date(primary.createdAt).toLocaleTimeString()} · posted to {announcements
					.map((a) => a.groupName)
					.join(', ')}
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
		<div class="responders">
			{#if responders.coming.length}
				<span class="muted small">Coming: {responders.coming.map(label).join(', ')}</span>
			{/if}
			{#if responders.ready.length}
				<span class="muted small">Ready: {responders.ready.map(label).join(', ')}</span>
			{/if}
		</div>
	{:else}
		<div class="actions">
			<button class="btn btn-sm btn-primary" disabled={hasClaimed} onclick={claim}>
				{hasClaimed ? '✓ On your list' : '➕ I got it — tick my list'}
			</button>
		</div>
		<div class="responders">
			{#if claimers.length}
				<span class="muted small">Got the kill: {claimers.map(label).join(', ')}</span>
			{:else}
				<span class="muted small">No one has claimed the benefit yet.</span>
			{/if}
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
	.group-badge {
		color: var(--info);
		border-color: var(--info);
	}
	.mine {
		color: var(--success);
		border-color: var(--success);
	}
</style>
