<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import { formatAgo, formatK } from '$lib/format';
	import type { CreatureSighting, HighscoreEntry } from '$lib/types';

	let entries = $state<HighscoreEntry[]>([]);
	let sightings = $state<CreatureSighting[]>([]);
	let loading = $state(true);
	let error = $state('');

	/** Which leaderboard is on screen: the players table or the Warden sightings. */
	type StatsView = 'players' | 'wardens';
	let view = $state<StatsView>('players');

	/** Clicking the active column flips it; a new column starts descending,
	 *  which is what you want first for every column here. */
	type Dir = 'desc' | 'asc';

	type SortKey = 'kills' | 'charmPoints' | 'score';
	const NUMERIC_KEYS: SortKey[] = ['kills', 'charmPoints', 'score'];
	let sortKey = $state<SortKey>('kills');
	let sortDir = $state<Dir>('desc');

	// Warden sightings view.
	type SightingKey = 'sightings' | 'hunters' | 'lastSeen';
	let sightingSort = $state<SightingKey>('sightings');
	let sightingDir = $state<Dir>('desc');
	let sightingSearch = $state('');
	let reportedOnly = $state(true);
	let showAll = $state(false);
	/** Rows shown before "Show all" is used; a search always shows every match. */
	const TOP_N = 20;

	function sortPlayersBy(key: SortKey) {
		if (sortKey === key) sortDir = sortDir === 'desc' ? 'asc' : 'desc';
		else ((sortKey = key), (sortDir = 'desc'));
	}

	function sortSightingsBy(key: SightingKey) {
		if (sightingSort === key) sightingDir = sightingDir === 'desc' ? 'asc' : 'desc';
		else ((sightingSort = key), (sightingDir = 'desc'));
	}

	function arrow(active: boolean, dir: Dir): string {
		return active ? (dir === 'desc' ? ' ▼' : ' ▲') : '';
	}

	function ariaSort(active: boolean, dir: Dir): 'ascending' | 'descending' | 'none' {
		return active ? (dir === 'desc' ? 'descending' : 'ascending') : 'none';
	}

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
			[entries, sightings] = await Promise.all([api.highscores(), api.creatureStats()]);
		} catch {
			error = 'Failed to load the statistics.';
		} finally {
			loading = false;
		}
	}

	// Sort by the active column in the chosen direction, tie-breaking on the other
	// numeric columns (always desc) then player name (asc) for a stable order —
	// only the column you picked flips, so ties don't reshuffle with it.
	let sorted = $derived.by(() => {
		const rest = NUMERIC_KEYS.filter((k) => k !== sortKey);
		const flip = sortDir === 'asc' ? -1 : 1;
		return [...entries].sort((a, b) => {
			if (b[sortKey] !== a[sortKey]) return (b[sortKey] - a[sortKey]) * flip;
			for (const k of rest) {
				if (b[k] !== a[k]) return b[k] - a[k];
			}
			return a.characterName.localeCompare(b.characterName);
		});
	});

	function seenAt(s: CreatureSighting): number {
		return s.lastSeen ? Date.parse(s.lastSeen) : 0;
	}

	// Active column in the chosen direction (descending Last seen = most recent
	// first), then times reported and name so ties stay stable across sorts.
	function compareSightings(a: CreatureSighting, b: CreatureSighting): number {
		const flip = sightingDir === 'asc' ? -1 : 1;
		if (sightingSort === 'lastSeen') {
			const [an, bn] = [seenAt(a), seenAt(b)];
			// Wardens nobody has announced have no date: keep them last either way
			// rather than letting ascending order lead with a wall of dashes.
			if ((an === 0) !== (bn === 0)) return an === 0 ? 1 : -1;
			if (an !== bn) return (bn - an) * flip;
		} else if (b[sightingSort] !== a[sightingSort]) {
			return (b[sightingSort] - a[sightingSort]) * flip;
		}
		if (b.sightings !== a.sightings) return b.sightings - a.sightings;
		return a.name.localeCompare(b.name);
	}

	let searching = $derived(sightingSearch.trim() !== '');

	let filteredSightings = $derived.by(() => {
		const q = sightingSearch.trim().toLowerCase();
		return sightings
			.filter((s) => (!reportedOnly || s.sightings > 0) && (!q || s.name.toLowerCase().includes(q)))
			.sort(compareSightings);
	});

	// A search is already narrow, so it never gets truncated to the top N.
	let visibleSightings = $derived(
		showAll || searching ? filteredSightings : filteredSightings.slice(0, TOP_N)
	);

	let totals = $derived.by(() => {
		let reported = 0;
		let count = 0;
		for (const s of sightings) {
			if (s.sightings > 0) reported++;
			count += s.sightings;
		}
		return { wardens: sightings.length, reported, count };
	});
</script>

<div class="container stack">
	<div class="spread header">
		<div>
			<h1>Statistics</h1>
			<p class="muted">
				{#if view === 'players'}
					Wardens killed, and charm points given away, across all groups.
				{:else if totals.wardens > 0}
					{totals.reported} of {totals.wardens} Wardens seen · {totals.count.toLocaleString()} sightings.
					A broadcast to several groups counts once.
				{:else}
					How often each Echo Warden has been announced, across every group.
				{/if}
			</p>
		</div>
		<div class="segmented" role="group" aria-label="Statistics view">
			<button
				class="segment"
				class:active={view === 'players'}
				aria-pressed={view === 'players'}
				onclick={() => (view = 'players')}
			>
				Players
			</button>
			<button
				class="segment"
				class:active={view === 'wardens'}
				aria-pressed={view === 'wardens'}
				onclick={() => (view = 'wardens')}
			>
				Wardens
			</button>
		</div>
	</div>

	{#if error}
		<p class="error">{error}</p>
	{:else if loading}
		<p class="muted">Loading…</p>
	{:else if view === 'players'}
		{#if entries.length === 0}
			<p class="muted">No Wardens have been killed or announced yet.</p>
		{:else}
			<div class="card table-wrap">
				<table class="scores">
					<thead>
						<tr>
							<th class="rank">#</th>
							<th class="player">Player</th>
							<th class="num" aria-sort={ariaSort(sortKey === 'kills', sortDir)}>
								<button class="sort" class:active={sortKey === 'kills'} onclick={() => sortPlayersBy('kills')}>
									Wardens{arrow(sortKey === 'kills', sortDir)}
								</button>
							</th>
							<th class="num" aria-sort={ariaSort(sortKey === 'charmPoints', sortDir)}>
								<button class="sort" class:active={sortKey === 'charmPoints'} onclick={() => sortPlayersBy('charmPoints')}>
									Charm Points{arrow(sortKey === 'charmPoints', sortDir)}
								</button>
							</th>
							<th class="num" aria-sort={ariaSort(sortKey === 'score', sortDir)}>
								<button class="sort" class:active={sortKey === 'score'} onclick={() => sortPlayersBy('score')}>
									Score{arrow(sortKey === 'score', sortDir)}
								</button>
							</th>
						</tr>
					</thead>
					<tbody>
						{#each sorted as e, i (e.userId)}
							<tr class:me={e.userId === $currentUser?.id}>
								<td class="rank">{i + 1}</td>
								<td class="player">{e.characterName}</td>
								<td class="num">{e.kills}</td>
								<td class="num charm">{e.charmPoints}</td>
								<td class="num" title={`${e.score.toLocaleString()} charm points given away`}>{formatK(e.score)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{:else}
		<div class="card stack">
			<div class="toolbar">
				<input
					class="warden-search"
					type="text"
					placeholder="Search Wardens…"
					bind:value={sightingSearch}
				/>
				<button
					class="chip"
					class:active={reportedOnly}
					aria-pressed={reportedOnly}
					title="Hide Wardens nobody has announced yet"
					onclick={() => (reportedOnly = !reportedOnly)}
				>
					Reported only
				</button>
			</div>

			{#if filteredSightings.length === 0}
				<p class="muted">
					{#if searching}
						No Wardens match your search.
					{:else}
						No Wardens have been announced yet.
					{/if}
				</p>
			{:else}
				<div class="table-wrap flush">
					<table class="scores">
						<thead>
							<tr>
								<th class="rank">#</th>
								<th class="player">Warden</th>
								<th class="num" aria-sort={ariaSort(sightingSort === 'sightings', sightingDir)}>
									<button
										class="sort"
										class:active={sightingSort === 'sightings'}
										title="Times announced (a broadcast counts once)"
										onclick={() => sortSightingsBy('sightings')}
									>
										Reported{arrow(sightingSort === 'sightings', sightingDir)}
									</button>
								</th>
								<th class="num" aria-sort={ariaSort(sightingSort === 'hunters', sightingDir)}>
									<button
										class="sort"
										class:active={sightingSort === 'hunters'}
										title="Players with this Warden ticked on their Warden List"
										onclick={() => sortSightingsBy('hunters')}
									>
										Players{arrow(sightingSort === 'hunters', sightingDir)}
									</button>
								</th>
								<th class="num" aria-sort={ariaSort(sightingSort === 'lastSeen', sightingDir)}>
									<button
										class="sort"
										class:active={sightingSort === 'lastSeen'}
										title="When it was last announced"
										onclick={() => sortSightingsBy('lastSeen')}
									>
										Last seen{arrow(sightingSort === 'lastSeen', sightingDir)}
									</button>
								</th>
							</tr>
						</thead>
						<tbody>
							{#each visibleSightings as s, i (s.creatureId)}
								<tr>
									<td class="rank">{i + 1}</td>
									<td class="player">
										<span class="warden">
											{#if s.imageUrl}
												<img
													class="creature-img"
													src={s.imageUrl}
													alt=""
													loading="lazy"
													onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
												/>
											{/if}
											<span class="name">{s.name}</span>
											<span class="badge diff" data-diff={s.difficulty} title="Difficulty · charm points"
												>{s.difficulty} ★ {s.charmPoints}</span
											>
											{#if s.rarity === 'Uncommon'}
												<span class="badge rare" title="Uncommon spawn">Uncommon</span>
											{/if}
										</span>
									</td>
									<td class="num strong">{s.sightings}</td>
									<td class="num">{s.hunters}</td>
									<td class="num dim" title={s.lastSeen ? new Date(s.lastSeen).toLocaleString() : ''}>
										{s.lastSeen ? formatAgo(s.lastSeen) : '—'}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				{#if !searching && filteredSightings.length > TOP_N}
					<button class="btn btn-sm more" onclick={() => (showAll = !showAll)}>
						{showAll ? `Show top ${TOP_N}` : `Show all ${filteredSightings.length}`}
					</button>
				{/if}
			{/if}
		</div>
	{/if}
</div>

<style>
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
	.table-wrap {
		padding: 0;
		overflow-x: auto;
	}
	/* The sightings table lives inside a padded card, so it bleeds to the card's
	   edges instead of carrying padding of its own. */
	.table-wrap.flush {
		margin: 0 -1.2rem;
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
	}
	.warden-search {
		flex: 1;
		min-width: 12rem;
	}
	.chip {
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 999px;
		color: var(--text-dim);
		font: inherit;
		font-size: 0.85rem;
		padding: 0.3rem 0.75rem;
		cursor: pointer;
		white-space: nowrap;
	}
	.chip:hover {
		color: var(--text);
	}
	.chip.active {
		border-color: var(--accent);
		color: var(--accent);
	}
	.more {
		align-self: flex-start;
	}
	.scores {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.95rem;
	}
	.scores th,
	.scores td {
		padding: 0.6rem 0.9rem;
		text-align: left;
		white-space: nowrap;
	}
	.scores thead th {
		border-bottom: 1px solid var(--border);
		color: var(--text-dim);
		font-weight: 600;
		font-size: 0.85rem;
	}
	.scores tbody tr {
		border-bottom: 1px solid var(--border);
	}
	.scores tbody tr:last-child {
		border-bottom: none;
	}
	.scores tbody tr.me {
		background: color-mix(in srgb, var(--accent) 12%, transparent);
	}
	.rank {
		width: 1%;
		color: var(--text-dim);
		font-variant-numeric: tabular-nums;
	}
	.player {
		font-weight: 550;
	}
	.warden {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.creature-img {
		width: 28px;
		height: 28px;
		object-fit: contain;
		flex: none;
		image-rendering: pixelated;
	}
	.name {
		font-weight: 550;
	}
	.rare {
		color: var(--info);
		border-color: var(--info);
		font-weight: 550;
	}
	/* Two classes (0,2,0) so this beats the base `.scores td`/`.scores th`
	   (0,1,1) rule and right-aligns both header and body numeric cells. */
	.scores .num {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	.charm {
		color: var(--accent);
		font-weight: 600;
	}
	.strong {
		font-weight: 600;
	}
	.dim {
		color: var(--text-dim);
	}
	.sort {
		background: none;
		border: none;
		color: inherit;
		font: inherit;
		font-weight: 600;
		font-size: 0.85rem;
		cursor: pointer;
		padding: 0;
	}
	.sort:hover {
		color: var(--text);
	}
	.sort.active {
		color: var(--accent);
	}
	/* On a phone the counts are the point of this table, so drop the difficulty
	   and rarity pills and tighten the cells rather than push the numbers further
	   off the side-scroll. */
	@media (max-width: 640px) {
		/* Stack the header so the view blurb gets the full width instead of
		   wrapping into a narrow column beside the switcher. */
		.header {
			flex-direction: column;
			align-items: flex-start;
		}
		.warden .badge {
			display: none;
		}
		.scores th,
		.scores td {
			padding: 0.55rem 0.5rem;
		}
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
