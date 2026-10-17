<script lang="ts">
	import { onMount } from 'svelte';
	import { flip } from 'svelte/animate';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import { FeedRoom, type RoomEvent } from '$lib/ws';
	import CheckerRow from '$lib/components/CheckerRow.svelte';
	import {
		COOLDOWN_MIN,
		WINDOW_MIN,
		compareByStatus,
		isActive,
		statusOf,
		type StatusInfo
	} from '$lib/checker';
	import type { Area, CheckerShare, CheckerState, Subarea, SubareaCheck } from '$lib/types';

	type Toast =
		| { kind: 'undo'; subareaId: number; text: string }
		| { kind: 'error'; text: string };

	let areas = $state<Area[]>([]);
	/** Every check visible to us (ours + groupmates sharing), any age; expired
	 * ones are ignored by `bySubarea` and dropped on the next refetch. */
	let checks = $state<SubareaCheck[]>([]);
	let favorites = $state<Set<number>>(new Set());
	let sharing = $state<CheckerShare[]>([]);
	let loading = $state(true);
	let error = $state('');

	/** serverNow − Date.now(), so every client counts down from the same clock. */
	let clockOffset = 0;
	let now = $state(Date.now());

	let search = $state('');
	/** Hide red subareas from the full list; yellow stays, it's worth a look. */
	let hideCooldown = $state(false);

	/** Per-area open state keyed by area *name* (stable across re-seeds).
	 * Absent = collapsed, so Favorites stays the focus. */
	const OPEN_KEY = 'checker:areaOpen';
	let openOverride = $state<Map<string, boolean>>(loadOpen());

	let toast = $state<Toast | null>(null);
	let toastTimer: ReturnType<typeof setTimeout> | undefined;
	/** In-flight check requests per subarea, so an Undo waits for its check. */
	const pending = new Map<number, Promise<unknown>>();

	let room: FeedRoom | null = null;

	function loadOpen(): Map<string, boolean> {
		try {
			const raw = localStorage.getItem(OPEN_KEY);
			if (raw) return new Map(JSON.parse(raw) as [string, boolean][]);
		} catch {
			// ignore malformed or unavailable storage
		}
		return new Map();
	}

	function saveOpen(map: Map<string, boolean>) {
		try {
			localStorage.setItem(OPEN_KEY, JSON.stringify([...map]));
		} catch {
			// ignore quota or unavailable storage
		}
	}

	$effect(() => {
		if (!$authLoading && !$currentUser) goto('/', { replaceState: true });
	});

	onMount(() => {
		const tick = setInterval(() => (now = Date.now() + clockOffset), 1000);
		load();
		return () => {
			clearInterval(tick);
			clearTimeout(toastTimer);
			room?.close();
		};
	});

	async function load() {
		try {
			const [a, c] = await Promise.all([api.areas(), api.checker()]);
			areas = a;
			applyState(c);
			room = new FeedRoom(handleEvent, refresh);
			room.connect();
		} catch {
			error = 'Failed to load the checker.';
		} finally {
			loading = false;
		}
	}

	// Re-fetch on every (re)connect and on resync events, reconciling anything
	// missed while the socket was down.
	async function refresh() {
		try {
			applyState(await api.checker());
		} catch {
			// keep the current view; the next reconnect retries
		}
	}

	function applyState(c: CheckerState) {
		clockOffset = Date.parse(c.now) - Date.now();
		now = Date.now() + clockOffset;
		checks = c.checks;
		favorites = new Set(c.favorites);
		sharing = c.sharing;
	}

	function handleEvent(event: RoomEvent) {
		if (event.type === 'checker.updated') {
			const { subareaId, userId, check } = event.payload;
			setCheck(subareaId, userId, check);
		} else if (event.type === 'checker.resync') {
			refresh();
		}
	}

	/** Upsert (or remove, when null) one user's check of one subarea. */
	function setCheck(subareaId: number, userId: number, check: SubareaCheck | null) {
		const rest = checks.filter((c) => !(c.subareaId === subareaId && c.userId === userId));
		checks = check ? [check, ...rest] : rest;
	}

	/** Active checks per subarea, newest first. */
	let bySubarea = $derived.by(() => {
		const m = new Map<number, SubareaCheck[]>();
		for (const c of checks) {
			if (!isActive(Date.parse(c.checkedAt), now)) continue;
			const list = m.get(c.subareaId);
			if (list) list.push(c);
			else m.set(c.subareaId, [c]);
		}
		for (const list of m.values()) {
			list.sort((a, b) => Date.parse(b.checkedAt) - Date.parse(a.checkedAt));
		}
		return m;
	});

	function checksFor(subareaId: number): SubareaCheck[] {
		return bySubarea.get(subareaId) ?? [];
	}

	function infoFor(subareaId: number): StatusInfo {
		const latest = bySubarea.get(subareaId)?.[0];
		return statusOf(latest ? Date.parse(latest.checkedAt) : null, now);
	}

	let subareaIndex = $derived(
		new Map(areas.flatMap((area) => area.subareas.map((s) => [s.id, { area, subarea: s }] as const)))
	);

	function labelOf(area: Area, subarea: Subarea): string {
		return subarea.name === area.name ? area.name : `${area.name} · ${subarea.name}`;
	}

	let favoriteRows = $derived.by(() => {
		const rows: { area: Area; subarea: Subarea; info: StatusInfo }[] = [];
		for (const id of favorites) {
			const hit = subareaIndex.get(id);
			if (hit) rows.push({ ...hit, info: infoFor(id) });
		}
		return rows.sort((a, b) => compareByStatus(a.info, b.info));
	});

	let term = $derived(search.trim().toLowerCase());
	let searching = $derived(term !== '');

	// A match on the area name shows all its subareas; otherwise only subareas
	// whose name or creatures match. Header counts always cover the whole area.
	let visibleAreas = $derived.by(() => {
		const out: {
			area: Area;
			rows: { subarea: Subarea; info: StatusInfo }[];
			red: number;
			yellow: number;
		}[] = [];
		for (const area of areas) {
			const areaMatch = !searching || area.name.toLowerCase().includes(term);
			let rows = area.subareas
				.filter(
					(s) =>
						areaMatch ||
						s.name.toLowerCase().includes(term) ||
						s.creatures.some((c) => c.name.toLowerCase().includes(term))
				)
				.map((subarea) => ({ subarea, info: infoFor(subarea.id) }));
			if (hideCooldown) rows = rows.filter((r) => r.info.status !== 'red');
			if (rows.length === 0) continue;
			rows.sort((a, b) => compareByStatus(a.info, b.info));
			let red = 0;
			let yellow = 0;
			for (const s of area.subareas) {
				const st = infoFor(s.id).status;
				if (st === 'red') red++;
				else if (st === 'yellow') yellow++;
			}
			out.push({ area, rows, red, yellow });
		}
		return out;
	});

	function isOpen(area: Area): boolean {
		return searching || (openOverride.get(area.name) ?? false);
	}

	function toggleArea(area: Area) {
		if (searching) return; // force-open while searching
		const next = new Map(openOverride);
		next.set(area.name, !isOpen(area));
		openOverride = next;
		saveOpen(next);
	}

	function setAllOpen(open: boolean) {
		const next = new Map(openOverride);
		for (const a of areas) next.set(a.name, open);
		openOverride = next;
		saveOpen(next);
	}

	function showToast(t: Toast) {
		clearTimeout(toastTimer);
		toast = t;
		toastTimer = setTimeout(() => (toast = null), t.kind === 'undo' ? 5000 : 4000);
	}

	function dismissToast() {
		clearTimeout(toastTimer);
		toast = null;
	}

	/** Mark a subarea as checked now (restarting the timer if already running).
	 * Optimistic; rolled back if the server rejects it. */
	async function check(area: Area, subarea: Subarea) {
		const user = $currentUser;
		if (!user) return;
		const prev = checks.find((c) => c.subareaId === subarea.id && c.userId === user.id) ?? null;
		setCheck(subarea.id, user.id, {
			subareaId: subarea.id,
			userId: user.id,
			name: user.characterName || user.discordUsername,
			avatar: user.discordAvatar,
			checkedAt: new Date(now).toISOString()
		});
		showToast({ kind: 'undo', subareaId: subarea.id, text: `Checked ${labelOf(area, subarea)}` });
		const req = api.checkSubarea(subarea.id);
		pending.set(subarea.id, req);
		try {
			const res = await req;
			setCheck(subarea.id, user.id, res.check);
		} catch {
			setCheck(subarea.id, user.id, prev);
			showToast({ kind: 'error', text: `Couldn't save the check of ${labelOf(area, subarea)}.` });
		} finally {
			if (pending.get(subarea.id) === req) pending.delete(subarea.id);
		}
	}

	async function undo(subareaId: number) {
		const user = $currentUser;
		if (!user) return;
		dismissToast();
		try {
			await pending.get(subareaId)?.catch(() => {});
			const res = await api.undoCheck(subareaId);
			setCheck(subareaId, user.id, res.check);
		} catch {
			showToast({ kind: 'error', text: "Couldn't undo the check." });
		}
	}

	async function toggleFavorite(subareaId: number) {
		const was = favorites.has(subareaId);
		const flipTo = (on: boolean) => {
			const next = new Set(favorites);
			if (on) next.add(subareaId);
			else next.delete(subareaId);
			favorites = next;
		};
		flipTo(!was);
		try {
			if (was) await api.removeCheckerFavorite(subareaId);
			else await api.addCheckerFavorite(subareaId);
		} catch {
			flipTo(was);
			showToast({ kind: 'error', text: "Couldn't update favorites." });
		}
	}

	let sharedCount = $derived(sharing.filter((s) => s.share).length);

	async function toggleShare(groupId: number) {
		const g = sharing.find((x) => x.groupId === groupId);
		if (!g) return;
		const share = !g.share;
		const setShare = (v: boolean) =>
			(sharing = sharing.map((x) => (x.groupId === groupId ? { ...x, share: v } : x)));
		setShare(share);
		try {
			await api.setCheckerShare(groupId, share);
		} catch {
			setShare(!share);
			showToast({ kind: 'error', text: "Couldn't update sharing." });
		}
	}
</script>

{#snippet row(area: Area, subarea: Subarea, info: StatusInfo, showArea: boolean)}
	<CheckerRow
		name={subarea.name}
		areaName={showArea ? area.name : undefined}
		checks={checksFor(subarea.id)}
		{info}
		{now}
		meId={$currentUser?.id}
		favorite={favorites.has(subarea.id)}
		oncheck={() => check(area, subarea)}
		ontogglefavorite={() => toggleFavorite(subarea.id)}
	/>
{/snippet}

<div class="container stack">
	<div class="spread page-head">
		<div>
			<h1>Checker</h1>
			<p class="muted">
				Tap a subarea when you step on it. A wild echo can respawn there {WINDOW_MIN}–{COOLDOWN_MIN}
				min later.
			</p>
		</div>
		<details class="share">
			<summary class="btn btn-sm">
				{#if sharing.length === 0}
					Not sharing
				{:else}
					Sharing with {sharedCount} of {sharing.length}
					{sharing.length === 1 ? 'group' : 'groups'}
				{/if}
			</summary>
			<div class="share-panel">
				{#if sharing.length === 0}
					<p class="muted">Join a group to see each other's checks live.</p>
				{:else}
					<p class="muted small">Members of these groups see your checks live.</p>
					{#each sharing as g (g.groupId)}
						<label class="share-row">
							<input type="checkbox" checked={g.share} onchange={() => toggleShare(g.groupId)} />
							<span>{g.name}</span>
						</label>
					{/each}
				{/if}
			</div>
		</details>
	</div>

	<div class="legend" aria-label="Legend">
		<span><i class="dot red"></i>Cooldown · 0–{WINDOW_MIN} min</span>
		<span><i class="dot yellow"></i>May respawn · {WINDOW_MIN}–{COOLDOWN_MIN} min</span>
		<span><i class="dot ready"></i>Ready</span>
	</div>

	{#if error}
		<p class="error">{error}</p>
	{:else if loading}
		<p class="muted">Loading…</p>
	{:else}
		<section class="section">
			<h2>★ Favorites</h2>
			{#if favoriteRows.length === 0}
				<p class="muted empty">Tap ☆ on any subarea below to pin it here.</p>
			{:else}
				<div class="rows">
					{#each favoriteRows as r (r.subarea.id)}
						<div animate:flip={{ duration: 300 }}>
							{@render row(r.area, r.subarea, r.info, true)}
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="section">
			<h2>All areas</h2>
			<input
				class="search"
				type="text"
				placeholder="Search areas, subareas or creatures…"
				bind:value={search}
			/>
			<div class="controls">
				<button
					class="chip"
					class:active={hideCooldown}
					aria-pressed={hideCooldown}
					onclick={() => (hideCooldown = !hideCooldown)}
				>
					Hide on cooldown
				</button>
				<span class="grow"></span>
				<button class="btn btn-sm" onclick={() => setAllOpen(false)}>Collapse all</button>
				<button class="btn btn-sm" onclick={() => setAllOpen(true)}>Expand all</button>
			</div>

			{#if areas.length === 0}
				<p class="muted">No areas have been set up yet.</p>
			{:else if visibleAreas.length === 0}
				<p class="muted">
					{searching ? 'No areas, subareas or creatures match your search.' : 'Everything is on cooldown.'}
				</p>
			{:else}
				<div class="areas">
					{#each visibleAreas as { area, rows, red, yellow } (area.id)}
						{#if area.subareas.length === 1}
							{@render row(area, rows[0].subarea, rows[0].info, true)}
						{:else}
							{@const open = isOpen(area)}
							<div class="area">
								<button
									class="area-head"
									class:clickable={!searching}
									aria-expanded={open}
									onclick={() => toggleArea(area)}
								>
									<span class="arrow" aria-hidden="true">{open ? '▾' : '▸'}</span>
									<span class="area-name">{area.name}</span>
									{#if red > 0}
										<span class="count red" title="{red} on cooldown"><i class="dot red"></i>{red}</span>
									{/if}
									{#if yellow > 0}
										<span class="count yellow" title="{yellow} may have respawned"
											><i class="dot yellow"></i>{yellow}</span
										>
									{/if}
									<span class="area-total">{area.subareas.length} spots</span>
								</button>
								{#if open}
									<div class="rows inner">
										{#each rows as r (r.subarea.id)}
											<div animate:flip={{ duration: 300 }}>
												{@render row(area, r.subarea, r.info, false)}
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</div>

{#if toast}
	<div class="toast" class:toast-error={toast.kind === 'error'} role="status">
		<span>{toast.text}</span>
		{#if toast.kind === 'undo'}
			{@const id = toast.subareaId}
			<button class="btn btn-sm" onclick={() => undo(id)}>Undo</button>
		{/if}
		<button class="close" aria-label="Dismiss" onclick={dismissToast}>✕</button>
	</div>
{/if}

<style>
	.page-head {
		flex-wrap: wrap;
		align-items: flex-start;
		gap: 0.8rem;
	}
	h2 {
		font-size: 1.05rem;
		margin: 0;
	}
	.section {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}
	.legend {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem 1.1rem;
		font-size: 0.85rem;
		color: var(--text-dim);
	}
	.dot {
		display: inline-block;
		width: 0.65rem;
		height: 0.65rem;
		border-radius: 50%;
		margin-right: 0.35rem;
		vertical-align: -0.05rem;
		border: 1px solid var(--border);
		background: var(--bg-elev-2);
	}
	.dot.red {
		background: var(--danger);
		border-color: var(--danger);
	}
	.dot.yellow {
		background: #e3c341;
		border-color: #e3c341;
	}
	.share {
		position: relative;
	}
	.share summary {
		list-style: none;
		cursor: pointer;
	}
	.share summary::-webkit-details-marker {
		display: none;
	}
	.share-panel {
		position: absolute;
		right: 0;
		top: calc(100% + 0.4rem);
		z-index: 10;
		width: min(18rem, calc(100vw - 2rem));
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		padding: 0.8rem;
		background: var(--bg-elev);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		box-shadow: var(--shadow);
	}
	.share-panel p {
		margin: 0;
	}
	.small {
		font-size: 0.85rem;
	}
	.share-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}
	.share-row input {
		flex: none;
		width: auto;
		margin: 0;
	}
	.empty {
		margin: 0;
		padding: 0.9rem;
		border: 1px dashed var(--border);
		border-radius: var(--radius);
	}
	.search {
		width: 100%;
	}
	.controls {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
	}
	.grow {
		flex: 1;
	}
	.chip {
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		color: var(--text-dim);
		border-radius: 999px;
		padding: 0.3rem 0.75rem;
		font-weight: 550;
		font-size: 0.85rem;
	}
	.chip.active {
		color: var(--text);
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 18%, var(--bg-elev-2));
	}
	.rows {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr));
		gap: 0.5rem;
	}
	.rows.inner {
		padding: 0 0.6rem 0.6rem;
	}
	.areas {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}
	.area {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-elev);
	}
	.area-head {
		width: 100%;
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.7rem 0.8rem;
		border: none;
		background: transparent;
		color: var(--text);
		text-align: left;
		cursor: default;
	}
	.area-head.clickable {
		cursor: pointer;
	}
	.arrow {
		color: var(--text-dim);
		width: 0.8rem;
	}
	.area-name {
		font-weight: 650;
		flex: 1;
		min-width: 0;
	}
	.count {
		font-size: 0.85rem;
		font-variant-numeric: tabular-nums;
	}
	.area-total {
		color: var(--text-dim);
		font-size: 0.85rem;
	}
	.toast {
		position: fixed;
		left: 50%;
		bottom: 1.2rem;
		transform: translateX(-50%);
		z-index: 50;
		display: flex;
		align-items: center;
		gap: 0.7rem;
		max-width: calc(100vw - 2rem);
		padding: 0.6rem 0.6rem 0.6rem 1rem;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		box-shadow: var(--shadow);
	}
	.toast-error {
		border-color: var(--danger);
	}
	.close {
		border: none;
		background: transparent;
		color: var(--text-dim);
		cursor: pointer;
		padding: 0.2rem 0.4rem;
	}
</style>
