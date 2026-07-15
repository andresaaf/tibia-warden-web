<script lang="ts">
	import type { Creature } from '$lib/types';

	let {
		creatures,
		value = $bindable(),
		placeholder = 'Search creature…'
	}: {
		creatures: Creature[];
		value: number | '';
		placeholder?: string;
	} = $props();

	let query = $state('');
	let show = $state(false);
	let highlight = $state(0);

	let filtered = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return creatures.slice(0, 8);
		return creatures
			.map((c) => ({ c, score: matchScore(c.name.toLowerCase(), q) }))
			.filter((x) => x.score < 4)
			.sort(
				(a, b) =>
					a.score - b.score ||
					a.c.name.length - b.c.name.length ||
					a.c.name.localeCompare(b.c.name)
			)
			.slice(0, 8)
			.map((x) => x.c);
	});

	// Lower score = better match: exact, prefix, word-start, then substring.
	function matchScore(name: string, q: string): number {
		if (name === q) return 0;
		if (name.startsWith(q)) return 1;
		if (name.split(/\s+/).some((w) => w.startsWith(q))) return 2;
		if (name.includes(q)) return 3;
		return 4;
	}

	function select(c: Creature) {
		value = c.id;
		query = c.name;
		show = false;
	}
	function onInput() {
		value = '';
		show = true;
		highlight = 0;
	}
	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			show = true;
			highlight = Math.min(highlight + 1, filtered.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			highlight = Math.max(highlight - 1, 0);
		} else if (e.key === 'Enter') {
			if (show && filtered[highlight]) {
				e.preventDefault();
				select(filtered[highlight]);
			}
		} else if (e.key === 'Escape') {
			show = false;
		}
	}
</script>

<div class="combobox">
	<input
		type="text"
		{placeholder}
		bind:value={query}
		autocomplete="off"
		oninput={onInput}
		onfocus={() => (show = true)}
		onblur={() => setTimeout(() => (show = false), 120)}
		onkeydown={onKeydown}
	/>
	{#if show && (filtered.length > 0 || query.trim())}
		<div class="combobox-list">
			{#each filtered as c, i (c.id)}
				<button
					type="button"
					class="opt"
					class:highlight={i === highlight}
					onclick={() => select(c)}
					onmousemove={() => (highlight = i)}
				>
					<span class="opt-name">{c.name}</span>
					<span class="badge diff" data-diff={c.difficulty}>{c.difficulty}</span>
				</button>
			{/each}
			{#if filtered.length === 0}
				<div class="opt empty muted">No creatures match</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.combobox {
		position: relative;
	}
	.combobox-list {
		position: absolute;
		z-index: 20;
		top: calc(100% + 4px);
		left: 0;
		right: 0;
		background: var(--bg-elev-2);
		border: 1px solid var(--border);
		border-radius: 8px;
		box-shadow: var(--shadow);
		max-height: 260px;
		overflow-y: auto;
		padding: 4px;
	}
	.opt {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		width: 100%;
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 0.45rem 0.55rem;
		border-radius: 6px;
	}
	.opt.highlight {
		background: var(--bg-elev);
	}
	.opt.empty {
		cursor: default;
	}
	.opt-name {
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
