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

	type SortKey = 'kills' | 'charmPoints' | 'score';
	const NUMERIC_KEYS: SortKey[] = ['kills', 'charmPoints', 'score'];
	let sortKey = $state<SortKey>('kills');

	// Warden sightings view.
	type SightingKey = 'sightings' | 'hunters' | 'lastSeen';
	let sightingSort = $state<SightingKey>('sightings');
	let sightingSearch = $state('');
	let reportedOnly = $state(true);
	let showAll = $state(false);
	/** Rows shown before "Show all" is used; a search always shows every match. */
	const TOP_N = 20;

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

	// Sort by the active column (desc), tie-breaking on the other numeric columns
	// (desc), then player name (asc) for a stable order.
	let sorted = $derived.by(() => {
		const rest = NUMERIC_KEYS.filter((k) => k !== sortKey);
		return [...entries].sort((a, b) => {
			if (b[sortKey] !== a[sortKey]) return b[sortKey] - a[sortKey];
			for (const k of rest) {
				if (b[k] !== a[k]) return b[k] - a[k];
			}
			return a.characterName.localeCompare(b.characterName);
		});
	});

	function seenAt(s: CreatureSighting): number {
		return s.lastSeen ? Date.parse(s.lastSeen) : 0;
	}

	// Active column (desc; most recent first for Last seen), then times reported
	// and name so the order is stable across sorts.
	function compareSightings(a: CreatureSighting, b: CreatureSighting): number {
		if (sightingSort === 'lastSeen') {
			if (seenAt(b) !== seenAt(a)) return seenAt(b) - seenAt(a);
		} else if (b[sightingSort] !== a[sightingSort]) {
			return b[sightingSort] - a[sightingSort];
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
							<th class="num">
								<button class="sort" class:active={sortKey === 'kills'} onclick={() => (sortKey = 'kills')}>
									Wardens{sortKey === 'kills' ? ' ▼' : ''}
								</button>
							</th>
							<th class="num">
								<button class="sort" class:active={sortKey === 'charmPoints'} onclick={() => (sortKey = 'charmPoints')}>
									Charm Points{sortKey === 'charmPoints' ? ' ▼' : ''}
								</button>
							</th>
							<th class="num">
								<button class="sort" class:active={sortKey === 'score'} onclick={() => (sortKey = 'score')}>
									Score{sortKey === 'score' ? ' ▼' : ''}
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
								<th class="num">
									<button
										class="sort"
										class:active={sightingSort === 'sightings'}
										title="Times announced (a broadcast counts once)"
										onclick={() => (sightingSort = 'sightings')}
									>
										Reported{sightingSort === 'sightings' ? ' ▼' : ''}
									</button>
								</th>
								<th class="num">
									<button
										class="sort"
										class:active={sightingSort === 'hunters'}
										title="Players with this Warden ticked on their Warden List"
										onclick={() => (sightingSort = 'hunters')}
									>
										Players{sightingSort === 'hunters' ? ' ▼' : ''}
									</button>
								</th>
								<th class="num">
									<button
										class="sort"
										class:active={sightingSort === 'lastSeen'}
										title="When it was last announced"
										onclick={() => (sightingSort = 'lastSeen')}
									>
										Last seen{sightingSort === 'lastSeen' ? ' ▼' : ''}
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
