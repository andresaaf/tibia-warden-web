<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import type { HighscoreEntry } from '$lib/types';

	let entries = $state<HighscoreEntry[]>([]);
	let loading = $state(true);
	let error = $state('');

	type SortKey = 'kills' | 'charmPoints' | 'announced';
	const NUMERIC_KEYS: SortKey[] = ['kills', 'charmPoints', 'announced'];
	let sortKey = $state<SortKey>('kills');

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
			entries = await api.highscores();
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
</script>

<div class="container stack">
	<div>
		<h1>Statistics</h1>
		<p class="muted">Echo Wardens killed and announced across all groups.</p>
	</div>

	{#if error}
		<p class="error">{error}</p>
	{:else if loading}
		<p class="muted">Loading…</p>
	{:else if entries.length === 0}
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
							<button class="sort" class:active={sortKey === 'announced'} onclick={() => (sortKey = 'announced')}>
								Announced{sortKey === 'announced' ? ' ▼' : ''}
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
							<td class="num">{e.announced}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	.table-wrap {
		padding: 0;
		overflow-x: auto;
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
	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	th.num {
		text-align: right;
	}
	.charm {
		color: var(--accent);
		font-weight: 600;
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
</style>
