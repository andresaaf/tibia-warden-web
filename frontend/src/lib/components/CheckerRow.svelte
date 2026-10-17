<script lang="ts">
	import { formatAgo, formatRemaining, type StatusInfo } from '$lib/checker';
	import type { SubareaCheck } from '$lib/types';

	interface Props {
		name: string;
		/** Shown muted under the name (Favorites list, where context is lost). */
		areaName?: string;
		/** Active checks of this subarea, newest first. */
		checks: SubareaCheck[];
		info: StatusInfo;
		now: number;
		meId?: number;
		favorite: boolean;
		oncheck: () => void;
		ontogglefavorite: () => void;
	}

	let { name, areaName, checks, info, now, meId, favorite, oncheck, ontogglefavorite }: Props =
		$props();

	const MAX_AVATARS = 3;

	let latest = $derived(checks[0]);
	let subtitle = $derived(
		[
			areaName && areaName !== name ? areaName : '',
			latest ? `${who(latest)} · ${ago(latest)}` : '',
			checks.length > 1 ? `+${checks.length - 1}` : ''
		]
			.filter(Boolean)
			.join(' · ')
	);
	let shown = $derived(checks.slice(0, MAX_AVATARS));
	let label = $derived(
		info.status === 'red' ? 'Cooldown' : info.status === 'yellow' ? 'May respawn' : 'Ready'
	);

	function ago(c: SubareaCheck): string {
		return formatAgo(now - Date.parse(c.checkedAt));
	}

	function who(c: SubareaCheck): string {
		return c.userId === meId ? 'You' : c.name;
	}
</script>

<div class="row status-{info.status}">
	<button
		class="main"
		onclick={oncheck}
		title={info.status === 'ready' ? 'Mark as checked now' : 'Checked again — restart the timer'}
	>
		<span class="text">
			<span class="name">{name}</span>
			{#if subtitle}<span class="by">{subtitle}</span>{/if}
		</span>
		{#if shown.length > 0}
			<span class="avatars">
				{#each shown as c (c.userId)}
					<span class="avatar" class:me={c.userId === meId} title="{who(c)} · {ago(c)}">
						{#if c.avatar}
							<img src={c.avatar} alt="" referrerpolicy="no-referrer" loading="lazy" />
						{:else}
							{c.name.charAt(0).toUpperCase()}
						{/if}
					</span>
				{/each}
				{#if checks.length > MAX_AVATARS}
					<span
						class="avatar more"
						title={checks
							.slice(MAX_AVATARS)
							.map((c) => `${who(c)} · ${ago(c)}`)
							.join('\n')}>+{checks.length - MAX_AVATARS}</span
					>
				{/if}
			</span>
		{/if}
		<span class="pill" aria-label={label}>
			{#if info.status === 'ready'}
				Ready
			{:else}
				<span class="pill-label">{label}</span>
				<span class="time">{formatRemaining(info.remainingMs)}</span>
			{/if}
		</span>
		{#if info.status !== 'ready'}
			<span class="bar" style="width: {info.progress * 100}%" aria-hidden="true"></span>
		{/if}
	</button>
	<button
		class="star"
		class:on={favorite}
		aria-pressed={favorite}
		aria-label={favorite ? 'Remove from favorites' : 'Add to favorites'}
		title={favorite ? 'Remove from favorites' : 'Add to favorites'}
		onclick={ontogglefavorite}>{favorite ? '★' : '☆'}</button
	>
</div>

<style>
	.row {
		--tone: var(--border);
		display: flex;
		align-items: stretch;
		border: 1px solid var(--border);
		border-left: 4px solid var(--tone);
		border-radius: var(--radius);
		background: var(--bg-elev);
		overflow: hidden;
	}
	.row.status-red {
		--tone: var(--danger);
		background: color-mix(in srgb, var(--danger) 13%, var(--bg-elev));
	}
	.row.status-yellow {
		--tone: var(--chk-yellow, #e3c341);
		background: color-mix(in srgb, var(--chk-yellow, #e3c341) 12%, var(--bg-elev));
	}
	.main {
		position: relative;
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.55rem 0.7rem;
		min-height: 3rem;
		border: none;
		border-radius: 0;
		background: transparent;
		color: var(--text);
		text-align: left;
		cursor: pointer;
	}
	.main:hover {
		background: color-mix(in srgb, var(--text) 5%, transparent);
	}
	.main:active {
		background: color-mix(in srgb, var(--text) 9%, transparent);
	}
	.text {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
	}
	.name {
		font-weight: 600;
		overflow-wrap: anywhere;
	}
	.by {
		color: var(--text-dim);
		font-size: 0.8rem;
		overflow-wrap: anywhere;
	}
	.avatars {
		display: flex;
		flex: none;
	}
	.avatar {
		width: 1.6rem;
		height: 1.6rem;
		border-radius: 50%;
		background: var(--bg-elev-2);
		border: 2px solid var(--bg-elev);
		margin-left: -0.45rem;
		display: grid;
		place-items: center;
		font-size: 0.75rem;
		font-weight: 700;
		color: var(--text-dim);
		overflow: hidden;
	}
	.avatar:first-child {
		margin-left: 0;
	}
	.avatar.me {
		border-color: var(--accent);
	}
	.avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}
	.avatar.more {
		font-size: 0.7rem;
	}
	.pill {
		flex: none;
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		min-width: 4.5rem;
		font-size: 0.85rem;
		color: var(--text-dim);
	}
	.pill-label {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		color: var(--tone);
	}
	.time {
		font-variant-numeric: tabular-nums;
		font-weight: 700;
		font-size: 1rem;
		color: var(--text);
	}
	.bar {
		position: absolute;
		left: 0;
		bottom: 0;
		height: 3px;
		background: var(--tone);
		opacity: 0.7;
		transition: width 1s linear;
	}
	.star {
		flex: none;
		width: 2.75rem;
		border: none;
		border-left: 1px solid var(--border);
		border-radius: 0;
		background: transparent;
		color: var(--text-dim);
		font-size: 1.2rem;
		cursor: pointer;
	}
	.star:hover {
		color: var(--accent-hover);
		background: color-mix(in srgb, var(--text) 5%, transparent);
	}
	.star.on {
		color: var(--accent);
	}
	/* Phones: the row tint already says the status; give the name the room. */
	@media (max-width: 500px) {
		.main {
			gap: 0.45rem;
			padding: 0.5rem 0.55rem;
		}
		.pill {
			min-width: 0;
		}
		.pill-label {
			display: none;
		}
	}
</style>
