<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import {
		DIFFICULTIES,
		RARITIES,
		type Area,
		type Creature,
		type Difficulty,
		type Rarity
	} from '$lib/types';

	type StatusFilter = 'all' | 'remaining' | 'found';
	type ViewMode = 'flat' | 'area';

	let viewMode = $state<ViewMode>('flat');

	let creatures = $state<Creature[]>([]);
	let displayed = $state<Creature[]>([]);
	let search = $state('');
	let activeDifficulties = $state<Set<Difficulty>>(new Set());
	let activeRarities = $state<Set<Rarity>>(new Set());
	let statusFilter = $state<StatusFilter>('all');
	let loading = $state(true);
	let error = $state('');
	let debounce: ReturnType<typeof setTimeout>;

	/** Authoritative killed state across both views, keyed by creature id. A
	 * creature can appear in several areas (and the flat list) at once, so all
	 * rendered instances read their state from here rather than a per-object flag. */
	let killedIds = $state<Set<number>>(new Set());

	let areas = $state<Area[]>([]);
	let areasLoaded = $state(false);
	let areasLoading = $state(false);
	let areasError = $state('');
	/** Completed areas the user has manually expanded (see areaOpen). */
	let expandedAreas = $state<Set<number>>(new Set());
	let areaSearch = $state('');

	$effect(() => {
		if (!$authLoading && !$currentUser) goto('/', { replaceState: true });
	});

	onMount(() => {
		loadKilled();
		load();
	});

	/** Seed the authoritative killed set from the full (unfiltered) warden list. */
	async function loadKilled() {
		try {
			const ids = await api.killedCreatures();
			killedIds = new Set(ids);
		} catch {
			// Non-fatal: the per-list payloads below still fill in killed state for
			// the creatures they return.
		}
	}

	async function load() {
		loading = true;
		error = '';
		try {
			creatures = await api.creatures(search.trim(), [...activeDifficulties], [...activeRarities]);
			mergeKilled(creatures);
			applyStatusFilter();
		} catch {
			error = 'Failed to load the warden list.';
		} finally {
			loading = false;
		}
	}

	async function loadAreas() {
		areasLoading = true;
		areasError = '';
		try {
			areas = await api.areas();
			for (const a of areas) mergeKilled(a.creatures);
			areasLoaded = true;
		} catch {
			areasError = 'Failed to load areas.';
		} finally {
			areasLoading = false;
		}
	}

	/** Union the killed creatures from a payload into the authoritative set.
	 * Only adds (never removes), so filtered flat-list payloads can't drop state. */
	function mergeKilled(list: Creature[]) {
		let changed = false;
		const next = new Set(killedIds);
		for (const c of list) {
			if (c.killed && !next.has(c.id)) {
				next.add(c.id);
				changed = true;
			}
		}
		if (changed) killedIds = next;
	}

	function setViewMode(next: ViewMode) {
		viewMode = next;
		if (next === 'area' && !areasLoaded && !areasLoading) loadAreas();
	}

	/** Snapshot the status filter over the loaded set. Called on load and when the
	 * status filter changes — not on every kill toggle, so items stay put until refilter. */
	function applyStatusFilter() {
		displayed = creatures.filter((c) =>
			statusFilter === 'all' ? true : statusFilter === 'remaining' ? !killedIds.has(c.id) : killedIds.has(c.id)
		);
	}

	function setStatusFilter(next: StatusFilter) {
		statusFilter = next;
		applyStatusFilter();
	}

	function onSearchInput() {
		clearTimeout(debounce);
		debounce = setTimeout(load, 250);
	}

	function toggleDifficulty(d: Difficulty) {
		const next = new Set(activeDifficulties);
		if (next.has(d)) next.delete(d);
		else next.add(d);
		activeDifficulties = next;
		load();
	}

	function toggleRarity(r: Rarity) {
		const next = new Set(activeRarities);
		if (next.has(r)) next.delete(r);
		else next.add(r);
		activeRarities = next;
		load();
	}

	async function toggleKilled(creatureId: number) {
		const wasKilled = killedIds.has(creatureId);
		const next = new Set(killedIds);
		if (wasKilled) next.delete(creatureId);
		else next.add(creatureId);
		killedIds = next;
		try {
			if (wasKilled) await api.unmarkKilled(creatureId);
			else await api.markKilled(creatureId);
		} catch {
			const rollback = new Set(killedIds);
			if (wasKilled) rollback.add(creatureId);
			else rollback.delete(creatureId);
			killedIds = rollback;
		}
	}

	function areaKilledCount(area: Area): number {
		return area.creatures.reduce((n, c) => n + (killedIds.has(c.id) ? 1 : 0), 0);
	}

	function areaComplete(area: Area): boolean {
		return area.creatures.length > 0 && areaKilledCount(area) === area.creatures.length;
	}

	/** Completed areas collapse by default; the arrow expands them. Incomplete
	 * areas are always open (their monsters are what you still need to hunt). */
	function areaOpen(area: Area): boolean {
		return !areaComplete(area) || expandedAreas.has(area.id);
	}

	function toggleArea(area: Area) {
		if (!areaComplete(area)) return; // incomplete areas stay expanded
		const next = new Set(expandedAreas);
		if (next.has(area.id)) next.delete(area.id);
		else next.add(area.id);
		expandedAreas = next;
	}

	let searching = $derived(areaSearch.trim() !== '');

	// Search matches area names first, then creatures within areas. A name match
	// shows the whole area; a creature-only match narrows the area to just the
	// matching monsters. Ordering: name matches, then creature-only matches, and
	// within each group incomplete areas before complete (the default sort).
	// Completion/progress is always over an area's full membership, never `shown`.
	let filteredAreas = $derived.by(() => {
		const term = areaSearch.trim().toLowerCase();
		const withMeta = areas.map((area) => {
			const nameMatch = term !== '' && area.name.toLowerCase().includes(term);
			const matches =
				term === '' ? area.creatures : area.creatures.filter((c) => c.name.toLowerCase().includes(term));
			return {
				area,
				shown: nameMatch ? area.creatures : matches,
				nameMatch,
				creatureMatch: matches.length > 0
			};
		});
		const visible = term === '' ? withMeta : withMeta.filter((m) => m.nameMatch || m.creatureMatch);
		return visible.sort((a, b) => {
			if (term !== '' && a.nameMatch !== b.nameMatch) return a.nameMatch ? -1 : 1;
			return Number(areaComplete(a.area)) - Number(areaComplete(b.area));
		});
	});
	let completedAreaCount = $derived(areas.filter((a) => areaComplete(a)).length);
	let killedCount = $derived(displayed.filter((c) => killedIds.has(c.id)).length);
</script>

<div class="container stack">
	<div class="spread">
		<div>
			<h1>Warden List</h1>
			<p class="muted">
				{#if viewMode === 'flat'}
					{killedCount} of {displayed.length} shown creatures marked
				{:else}
					{completedAreaCount} of {areas.length} areas complete
				{/if}
			</p>
		</div>
		<div class="segmented" role="group" aria-label="View mode">
			<button
				class="segment"
				class:active={viewMode === 'flat'}
				aria-pressed={viewMode === 'flat'}
				onclick={() => setViewMode('flat')}
			>
				List
			</button>
			<button
				class="segment"
				class:active={viewMode === 'area'}
				aria-pressed={viewMode === 'area'}
				onclick={() => setViewMode('area')}
			>
				Areas
			</button>
		</div>
	</div>

	{#if viewMode === 'flat'}
		<div class="card stack">
			<input
				type="text"
				placeholder="Search creatures…"
				bind:value={search}
				oninput={onSearchInput}
			/>
			<div class="filters">
				<div class="chip-rows">
					<div class="chips">
						{#each DIFFICULTIES as d}
							<button
								class="chip"
								class:active={activeDifficulties.has(d)}
								data-diff={d}
								onclick={() => toggleDifficulty(d)}
							>
								{d}
							</button>
						{/each}
					</div>
					<div class="chips">
						{#each RARITIES as r}
							<button
								class="chip"
								class:active={activeRarities.has(r)}
								onclick={() => toggleRarity(r)}
							>
								{r}
							</button>
						{/each}
					</div>
				</div>
				<div class="segmented" role="group" aria-label="Filter by status">
					<button
						class="segment"
						class:active={statusFilter === 'all'}
						aria-pressed={statusFilter === 'all'}
						onclick={() => setStatusFilter('all')}
					>
						All
					</button>
					<button
						class="segment"
						class:active={statusFilter === 'remaining'}
						aria-pressed={statusFilter === 'remaining'}
						onclick={() => setStatusFilter('remaining')}
					>
						Remaining
					</button>
					<button
						class="segment"
						class:active={statusFilter === 'found'}
						aria-pressed={statusFilter === 'found'}
						onclick={() => setStatusFilter('found')}
					>
						Found
					</button>
				</div>
			</div>
		</div>

		{#if error}
			<p class="error">{error}</p>
		{:else if loading}
			<p class="muted">Loading…</p>
		{:else if displayed.length === 0}
			<p class="muted">No creatures match your filters.</p>
		{:else}
			<div class="grid">
				{#each displayed as creature (creature.id)}
					<button
						class="creature"
						class:killed={killedIds.has(creature.id)}
						onclick={() => toggleKilled(creature.id)}
					>
						<span class="check" aria-hidden="true">{killedIds.has(creature.id) ? '✓' : ''}</span>
						{#if creature.imageUrl}
							<img
								class="creature-img"
								src={creature.imageUrl}
								alt=""
								loading="lazy"
								onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
							/>
						{/if}
						<span class="name">{creature.name}</span>
						<span class="badge diff" data-diff={creature.difficulty}>{creature.difficulty}</span>
					</button>
				{/each}
			</div>
		{/if}
	{:else if areasError}
		<p class="error">{areasError}</p>
	{:else if areasLoading}
		<p class="muted">Loading…</p>
	{:else if areas.length === 0}
		<p class="muted">No areas have been set up yet.</p>
	{:else}
		<input
			class="area-search"
			type="text"
			placeholder="Search areas or creatures…"
			bind:value={areaSearch}
		/>
		{#if filteredAreas.length === 0}
			<p class="muted">No areas or creatures match your search.</p>
		{:else}
		<div class="areas">
			{#each filteredAreas as { area, shown } (area.id)}
				{@const done = areaComplete(area)}
				{@const open = searching || areaOpen(area)}
				<div class="area" class:complete={done}>
					<button
						class="area-head"
						class:clickable={done && !searching}
						aria-expanded={open}
						onclick={() => toggleArea(area)}
					>
						<span class="arrow" aria-hidden="true">{done ? (open ? '▾' : '▸') : ''}</span>
						<span class="area-name">{area.name}</span>
						{#if done}<span class="area-done" aria-hidden="true">✓</span>{/if}
						<span class="area-progress">{areaKilledCount(area)} / {area.creatures.length}</span>
					</button>
					{#if open}
						<div class="grid">
							{#each shown as creature (creature.id)}
								<button
									class="creature"
									class:killed={killedIds.has(creature.id)}
									onclick={() => toggleKilled(creature.id)}
								>
									<span class="check" aria-hidden="true">{killedIds.has(creature.id) ? '✓' : ''}</span>
									{#if creature.imageUrl}
										<img
											class="creature-img"
											src={creature.imageUrl}
											alt=""
											loading="lazy"
											onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
										/>
									{/if}
									<span class="name">{creature.name}</span>
									<span class="badge diff" data-diff={creature.difficulty}>{creature.difficulty}</span>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		</div>
		{/if}
	{/if}
</div>

<style>
	.area-search {
		width: 100%;
	}
	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.6rem;
	}
	.chip-rows {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.segmented {
		display: inline-flex;
		flex: none;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 2px;
	}
	.segment {
		border: none;
		background: transparent;
		color: var(--text-dim);
		border-radius: 999px;
		padding: 0.3rem 0.8rem;
		font-weight: 550;
		font-size: 0.85rem;
	}
	.segment.active {
		color: var(--text);
		background: color-mix(in srgb, var(--accent) 22%, var(--bg-elev-2));
		box-shadow: inset 0 0 0 1px var(--accent);
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
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
		gap: 0.6rem;
	}
	.areas {
		display: flex;
		flex-direction: column;
		gap: 0.8rem;
	}
	.area {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--bg-elev);
		padding: 0.6rem;
	}
	.area.complete {
		border-color: var(--success);
		background: color-mix(in srgb, var(--success) 8%, var(--bg-elev));
	}
	.area-head {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		width: 100%;
		background: transparent;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 0.3rem 0.4rem;
		cursor: default;
	}
	.area-head.clickable {
		cursor: pointer;
	}
	.arrow {
		width: 1rem;
		flex: none;
		color: var(--text-dim);
	}
	.area-name {
		flex: 1;
		font-weight: 650;
	}
	.area-done {
		color: var(--success);
		font-weight: 700;
	}
	.area-progress {
		color: var(--text-dim);
		font-size: 0.85rem;
		font-variant-numeric: tabular-nums;
	}
	.area .grid {
		margin-top: 0.6rem;
	}
	.creature {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		background: var(--bg-elev);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 0.7rem 0.9rem;
		text-align: left;
		color: var(--text);
		transition: border-color 0.15s;
	}
	.creature:hover {
		border-color: var(--accent);
	}
	.creature.killed {
		border-color: var(--success);
		background: color-mix(in srgb, var(--success) 10%, var(--bg-elev));
	}
	.check {
		width: 22px;
		height: 22px;
		flex: none;
		border-radius: 6px;
		border: 1px solid var(--border);
		display: grid;
		place-items: center;
		color: var(--success);
		font-weight: 700;
	}
	.creature.killed .check {
		border-color: var(--success);
		background: var(--success);
		color: #06210f;
	}
	.creature-img {
		width: 32px;
		height: 32px;
		object-fit: contain;
		flex: none;
		image-rendering: pixelated;
	}
	.name {
		flex: 1;
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
</style>
