<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import { DIFFICULTIES, type Creature, type Difficulty } from '$lib/types';

	let creatures = $state<Creature[]>([]);
	let search = $state('');
	let activeDifficulties = $state<Set<Difficulty>>(new Set());
	let loading = $state(true);
	let error = $state('');
	let debounce: ReturnType<typeof setTimeout>;

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
			creatures = await api.creatures(search.trim(), [...activeDifficulties]);
		} catch {
			error = 'Failed to load the warden list.';
		} finally {
			loading = false;
		}
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

	async function toggleKilled(creature: Creature) {
		const previous = creature.killed;
		creature.killed = !previous;
		creatures = [...creatures];
		try {
			if (creature.killed) await api.markKilled(creature.id);
			else await api.unmarkKilled(creature.id);
		} catch {
			creature.killed = previous;
			creatures = [...creatures];
		}
	}

	let killedCount = $derived(creatures.filter((c) => c.killed).length);
</script>

<div class="container stack">
	<div class="spread">
		<div>
			<h1>Warden List</h1>
			<p class="muted">
				{killedCount} of {creatures.length} shown creatures marked
			</p>
		</div>
	</div>

	<div class="card stack">
		<input
			type="text"
			placeholder="Search creatures…"
			bind:value={search}
			oninput={onSearchInput}
		/>
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
	</div>

	{#if error}
		<p class="error">{error}</p>
	{:else if loading}
		<p class="muted">Loading…</p>
	{:else if creatures.length === 0}
		<p class="muted">No creatures match your filters.</p>
	{:else}
		<div class="grid">
			{#each creatures as creature (creature.id)}
				<button
					class="creature"
					class:killed={creature.killed}
					onclick={() => toggleKilled(creature)}
				>
					<span class="check" aria-hidden="true">{creature.killed ? '✓' : ''}</span>
					<span class="name">{creature.name}</span>
					<span class="badge diff" data-diff={creature.difficulty}>{creature.difficulty}</span>
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
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
