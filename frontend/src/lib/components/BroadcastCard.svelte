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

	// The note is shared across the broadcast; editing the primary cascades to
	// every sibling on the backend.
	let canEditNote = $derived(!killed && canKill);
	let editingNote = $state(false);
	let editNoteText = $state('');
	let savingNote = $state(false);
	function startEditNote() {
		editNoteText = primary.note;
		editingNote = true;
	}
	function cancelEditNote() {
		editingNote = false;
	}
	async function saveNote() {
		savingNote = true;
		try {
			await api.editAnnouncementNote(primary.id, editNoteText.trim());
			editingNote = false;
		} catch (err) {
			fail(err);
		} finally {
			savingNote = false;
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
				<span class="badge diff" data-diff={primary.difficulty} title="Difficulty · charm points">{primary.difficulty} ★ {primary.charmPoints}</span>
				<span class="badge group-badge">{announcements.length} groups</span>
				{#if alreadyKilled}
					<span class="badge mine" title="You've already killed this Echo Warden">✓ In your list</span>
				{/if}
			</div>
			{#if editingNote}
				<form class="note-edit" onsubmit={(e) => { e.preventDefault(); saveNote(); }}>
					<!-- svelte-ignore a11y_autofocus -->
					<textarea
						bind:value={editNoteText}
						maxlength="1000"
						rows="2"
						placeholder="Add a note…"
						disabled={savingNote}
						autofocus
					></textarea>
					<div class="note-edit-actions">
						<button type="submit" class="btn btn-sm btn-primary" disabled={savingNote}>
							{savingNote ? 'Saving…' : 'Save'}
						</button>
						<button type="button" class="btn btn-sm" onclick={cancelEditNote} disabled={savingNote}>
							Cancel
						</button>
					</div>
				</form>
			{:else}
				{#if primary.note}<div class="muted note">{primary.note}</div>{/if}
				{#if canEditNote}
					<button type="button" class="note-edit-trigger" onclick={startEditNote}>
						{primary.note ? '✏️ Edit note' : '＋ Add note'}
					</button>
				{/if}
			{/if}
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
	.note-edit {
		margin-top: 0.35rem;
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	.note-edit textarea {
		width: 100%;
		resize: vertical;
		font: inherit;
	}
	.note-edit-actions {
		display: flex;
		gap: 0.4rem;
	}
	.note-edit-trigger {
		margin-top: 0.2rem;
		padding: 0;
		background: none;
		border: none;
		color: var(--text-dim);
		font-size: 0.82rem;
		cursor: pointer;
	}
	.note-edit-trigger:hover {
		color: var(--text);
		text-decoration: underline;
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
	.diff {
		white-space: nowrap;
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
	.status-open {
		color: var(--success);
		border-color: var(--success);
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
