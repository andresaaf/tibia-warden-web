<script lang="ts">
	import TibiaMap from './TibiaMap.svelte';

	// Display-side map for an announcement card: a lightweight static preview
	// (the same PNG the backend serves to Discord) that expands into the
	// interactive TibiaMap on click. Keeps a feed of many cards cheap — plain
	// images until the viewer opts to explore one.
	let { id, x, y, z }: { id: number; x: number; y: number; z: number } = $props();

	let expanded = $state(false);
</script>

<div class="ann-map">
	{#if expanded}
		<TibiaMap mode="view" {x} {y} {z} height="300px" />
		<button type="button" class="toggle" onclick={() => (expanded = false)}>Collapse map</button>
	{:else}
		<button type="button" class="preview" onclick={() => (expanded = true)} title="Expand map">
			<img src={`/api/announcements/${id}/map.png`} alt="Map location" loading="lazy" />
			<span class="hint">🗺️ Expand</span>
		</button>
	{/if}
</div>

<style>
	.ann-map {
		margin: 0.5rem 0;
	}
	.preview {
		position: relative;
		display: block;
		width: 100%;
		max-width: 460px;
		padding: 0;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
		background: #181a1e;
		cursor: pointer;
	}
	.preview img {
		display: block;
		width: 100%;
		height: auto;
	}
	.hint {
		position: absolute;
		right: 6px;
		bottom: 6px;
		font-size: 0.75rem;
		padding: 0.1rem 0.4rem;
		border-radius: 6px;
		background: rgba(0, 0, 0, 0.6);
		color: #fff;
	}
	.toggle {
		margin-top: 0.35rem;
		background: none;
		border: none;
		color: var(--muted, var(--text));
		font-size: 0.8rem;
		cursor: pointer;
		padding: 0;
	}
</style>
