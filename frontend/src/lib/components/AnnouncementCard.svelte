<script lang="ts">
	import { api, ApiError } from '$lib/api';
	import { copyText } from '$lib/clipboard';
	import type { Announcement } from '$lib/types';

	let {
		a,
		meId,
		canManage = false,
		showGroup = false,
		alreadyKilled = false,
		onactionerror,
		onclaimed
	}: {
		a: Announcement;
		meId: number | undefined;
		canManage?: boolean;
		showGroup?: boolean;
		alreadyKilled?: boolean;
		onactionerror?: (msg: string) => void;
		onclaimed?: (creatureId: number) => void;
	} = $props();

	function fail(err: unknown) {
		onactionerror?.(err instanceof ApiError ? err.message : 'Something went wrong.');
	}

	function myResponse(): string | null {
		return a.responses.find((r) => r.userId === meId)?.status ?? null;
	}
	function hasClaimed(): boolean {
		return a.claims.some((c) => c.userId === meId);
	}
	let canKill = $derived(a.authorId === meId || canManage);
	let canEditNote = $derived(a.status === 'open' && (a.authorId === meId || canManage));
	let coming = $derived(a.responses.filter((r) => r.status === 'coming'));
	let ready = $derived(a.responses.filter((r) => r.status === 'ready'));

	let editingNote = $state(false);
	let editNoteText = $state('');
	let savingNote = $state(false);
	function startEditNote() {
		editNoteText = a.note;
		editingNote = true;
	}
	function cancelEditNote() {
		editingNote = false;
	}
	async function saveNote() {
		savingNote = true;
		try {
			await api.editAnnouncementNote(a.id, editNoteText.trim());
			editingNote = false;
		} catch (err) {
			fail(err);
		} finally {
			savingNote = false;
		}
	}

	async function respond(status: 'coming' | 'ready') {
		try {
			if (myResponse() === status) await api.clearResponse(a.id);
			else await api.setResponse(a.id, status);
		} catch (err) {
			fail(err);
		}
	}
	async function markKilled() {
		try {
			await api.markAnnouncementKilled(a.id);
		} catch (err) {
			fail(err);
		}
	}
	async function claim() {
		try {
			await api.claimAnnouncement(a.id);
			onclaimed?.(a.creatureId);
		} catch (err) {
			fail(err);
		}
	}
</script>

<div class="card announcement" class:killed={a.status === 'killed'}>
	<div class="head">
		{#if a.creatureImageUrl}
			<img
				class="creature-img"
				src={a.creatureImageUrl}
				alt=""
				onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
			/>
		{/if}
		<div class="head-text">
			<div class="row" style="gap: 0.5rem; flex-wrap: wrap">
				<strong class="creature-name">{a.creatureName}</strong>
				{#if a.status === 'killed'}
					<span class="badge status-killed">Killed</span>
				{:else}
					<span class="badge status-open">Open</span>
				{/if}
				<span class="badge diff" data-diff={a.difficulty} title="Difficulty · charm points">{a.difficulty} ★ {a.charmPoints}</span>
				{#if showGroup && a.groupName}
					<a class="badge group-badge" href={`/groups/${a.groupId}`}>{a.groupName}</a>
				{/if}
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
				{#if a.note}<div class="muted note">{a.note}</div>{/if}
				{#if canEditNote}
					<button type="button" class="note-edit-trigger" onclick={startEditNote}>
						{a.note ? '✏️ Edit note' : '＋ Add note'}
					</button>
				{/if}
			{/if}
			<div class="muted small">
				by {a.authorName}<button
					type="button"
					class="exiva-btn"
					title={`Copy: exiva "${a.authorName}`}
					aria-label={`Copy exiva command for ${a.authorName}`}
					onclick={() => copyText(`exiva "${a.authorName}`)}>🔍</button> · {new Date(a.createdAt).toLocaleTimeString()}
			</div>
		</div>
	</div>

	{#if a.status === 'open'}
		<div class="actions">
			<button class="btn btn-sm" class:on={myResponse() === 'coming'} onclick={() => respond('coming')}>
				🏃 Coming
			</button>
			<button class="btn btn-sm" class:on-ready={myResponse() === 'ready'} onclick={() => respond('ready')}>
				✅ Ready
			</button>
			{#if canKill}
				<button class="btn btn-sm btn-danger" onclick={markKilled}>💀 Killed</button>
			{/if}
		</div>
		<div class="responders">
			{#if coming.length}
				<span class="muted small">Coming: {coming.map((r) => r.characterName).join(', ')}</span>
			{/if}
			{#if ready.length}
				<span class="muted small">Ready: {ready.map((r) => r.characterName).join(', ')}</span>
			{/if}
		</div>
	{:else}
		<div class="actions">
			<button class="btn btn-sm btn-primary" disabled={hasClaimed()} onclick={claim}>
				{hasClaimed() ? '✓ On your list' : '➕ I got it — tick my list'}
			</button>
		</div>
		<div class="responders">
			{#if a.claims.length}
				<span class="muted small">Got the kill: {a.claims.map((c) => c.characterName).join(', ')}</span>
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
	.exiva-btn {
		padding: 0;
		margin-left: 0.15rem;
		background: none;
		border: none;
		font-size: 0.9rem;
		line-height: 1;
		cursor: pointer;
		opacity: 0.65;
	}
	.exiva-btn:hover {
		opacity: 1;
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
