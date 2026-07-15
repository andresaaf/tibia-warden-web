<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api';
	import { currentUser, authLoading } from '$lib/stores';
	import { FeedRoom, type RoomEvent } from '$lib/ws';
	import AnnouncementCard from '$lib/components/AnnouncementCard.svelte';
	import CreatureCombobox from '$lib/components/CreatureCombobox.svelte';
	import type { Announcement, Creature, Group } from '$lib/types';

	let feed = $state<Announcement[]>([]);
	let groups = $state<Group[]>([]);
	let creatures = $state<Creature[]>([]);
	let killedIds = $state<number[]>([]);
	let loading = $state(true);
	let error = $state('');
	let room: FeedRoom | null = null;

	// Composer.
	let creatureId = $state<number | ''>('');
	let note = $state('');
	let target = $state<string>('all');
	let posting = $state(false);
	let postError = $state('');
	let composerKey = $state(0);

	let me = $derived($currentUser);
	let showLogin = $derived(!$authLoading && !$currentUser);

	// Onboard users who haven't set a character name yet.
	$effect(() => {
		if (!$authLoading && $currentUser && !$currentUser.characterName) {
			goto('/onboarding', { replaceState: true });
		}
	});

	onMount(() => {
		return () => room?.close();
	});

	// Initialise once auth has resolved and the user is onboarded.
	let started = false;
	$effect(() => {
		if (!started && !$authLoading && $currentUser?.characterName) {
			started = true;
			init();
		}
	});

	async function init() {
		loading = true;
		try {
			const [f, g, c, k] = await Promise.all([
				api.feed(),
				api.listGroups('mine'),
				api.creatures('', []),
				api.killedCreatures()
			]);
			feed = f;
			groups = g;
			creatures = c;
			killedIds = k;
			room = new FeedRoom(handleEvent);
			room.connect();
		} catch {
			error = 'Failed to load your dashboard.';
		} finally {
			loading = false;
		}
	}

	function handleEvent(event: RoomEvent) {
		const a = event.payload;
		const idx = feed.findIndex((x) => x.id === a.id);
		if (idx >= 0) {
			feed[idx] = a;
			feed = [...feed];
		} else if (event.type === 'announcement.created') {
			feed = [a, ...feed];
		}
	}

	async function post(e: SubmitEvent) {
		e.preventDefault();
		postError = '';
		if (!creatureId) {
			postError = 'Choose a creature.';
			return;
		}
		posting = true;
		try {
			await api.broadcastAnnouncement({
				creatureId: Number(creatureId),
				note: note.trim(),
				goldCost: 0,
				groupIds: target === 'all' ? undefined : [Number(target)]
			});
			creatureId = '';
			note = '';
			composerKey++;
		} catch (err) {
			postError = err instanceof ApiError ? err.message : 'Failed to post.';
		} finally {
			posting = false;
		}
	}

	function canManage(a: Announcement): boolean {
		return a.viewerRole === 'owner' || a.viewerRole === 'admin';
	}
</script>

{#if showLogin}
	<div class="hero">
		<div class="hero-card card">
			<div class="mark">◈</div>
			<h1>Echo Warden Tracker</h1>
			<p class="muted">
				Coordinate Echo Warden reveals with your Tibia community. Track your Bestiary progress and
				rally your group the moment a Warden appears.
			</p>
			<a class="btn btn-primary discord" href="/api/auth/discord/login">
				<span>Continue with Discord</span>
			</a>
		</div>
	</div>
{:else if $currentUser?.characterName}
	<div class="container stack">
		<h1>Home</h1>

		{#if error}<p class="error">{error}</p>{/if}

		{#if groups.length === 0}
			<div class="card">
				<p class="muted">You're not in any groups yet.</p>
				<a class="btn btn-primary" href="/groups">Find or create a group</a>
			</div>
		{:else}
			<form class="card stack post-form" onsubmit={post}>
				<h3>Announce an Echo Warden</h3>
				{#key composerKey}
					<CreatureCombobox {creatures} bind:value={creatureId} />
				{/key}
				<input type="text" placeholder="Note (optional)" bind:value={note} />
				<div class="row target-row">
					<span class="muted small">Post to</span>
					<select bind:value={target}>
						<option value="all">All my groups</option>
						{#each groups as g (g.id)}
							<option value={String(g.id)}>{g.name}</option>
						{/each}
					</select>
				</div>
				{#if postError}<p class="error">{postError}</p>{/if}
				<button class="btn btn-primary" type="submit" disabled={posting}>
					{posting ? 'Posting…' : 'Post announcement'}
				</button>
			</form>
		{/if}

		<div class="stack feed">
			{#if loading}
				<p class="muted">Loading…</p>
			{:else if feed.length === 0}
				<p class="muted">No announcements yet across your groups.</p>
			{:else}
				{#each feed as a (a.id)}
					<AnnouncementCard
						{a}
						meId={me?.id}
						canManage={canManage(a)}
						showGroup={true}
						alreadyKilled={killedIds.includes(a.creatureId)}
						onactionerror={(m) => (error = m)}
						onclaimed={(cid) => {
							if (!killedIds.includes(cid)) killedIds = [...killedIds, cid];
						}}
					/>
				{/each}
			{/if}
		</div>
	</div>
{/if}

<style>
	.post-form {
		margin-bottom: 0.5rem;
	}
	.target-row {
		gap: 0.6rem;
	}
	.target-row select {
		width: auto;
	}
	.feed {
		gap: 0.7rem;
	}
	.small {
		font-size: 0.85rem;
	}
	.hero {
		display: flex;
		justify-content: center;
		padding: 4rem 1.5rem;
	}
	.hero-card {
		max-width: 440px;
		text-align: center;
		padding: 2.5rem 2rem;
	}
	.mark {
		font-size: 2.5rem;
		color: var(--accent);
		margin-bottom: 0.5rem;
	}
	.hero-card p {
		margin: 0.75rem 0 1.75rem;
	}
	.discord {
		width: 100%;
		justify-content: center;
		background: #5865f2;
		border-color: #5865f2;
		color: #fff;
		padding: 0.7rem;
		font-size: 1rem;
	}
	.discord:hover {
		background: #4752c4;
		border-color: #4752c4;
	}
</style>
