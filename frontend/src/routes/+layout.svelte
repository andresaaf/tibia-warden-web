<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { currentUser, authLoading, loadCurrentUser, logout } from '$lib/stores';

	let { children } = $props();

	onMount(() => {
		loadCurrentUser().catch(() => {});
	});

	async function handleLogout() {
		await logout();
		goto('/');
	}

	const navItems = [
		{ href: '/groups', label: 'Groups' },
		{ href: '/wardens', label: 'Warden List' }
	];
</script>

<div class="app">
	<header class="topbar">
		<div class="topbar-inner">
			<a class="brand" href="/groups">
				<span class="brand-mark">◈</span> Echo Warden Tracker
			</a>

			{#if $currentUser}
				<nav class="nav">
					{#each navItems as item}
						<a
							class="nav-link"
							class:active={$page.url.pathname.startsWith(item.href)}
							href={item.href}>{item.label}</a
						>
					{/each}
				</nav>
				<div class="user">
					<span class="muted">{$currentUser.characterName || $currentUser.discordUsername}</span>
					<button class="btn btn-sm" onclick={handleLogout}>Log out</button>
				</div>
			{/if}
		</div>
	</header>

	<main>
		{#if $authLoading && !$currentUser}
			<div class="container muted">Loading…</div>
		{:else}
			{@render children()}
		{/if}
	</main>
</div>

<style>
	.app {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
	}
	.topbar {
		border-bottom: 1px solid var(--border);
		background: var(--bg-elev);
		position: sticky;
		top: 0;
		z-index: 10;
	}
	.topbar-inner {
		max-width: 960px;
		margin: 0 auto;
		padding: 0.7rem 1.5rem;
		display: flex;
		align-items: center;
		gap: 1.25rem;
	}
	.brand {
		font-weight: 700;
		color: var(--text);
		font-size: 1.05rem;
		white-space: nowrap;
	}
	.brand-mark {
		color: var(--accent);
	}
	.nav {
		display: flex;
		gap: 0.35rem;
		margin-left: 0.5rem;
	}
	.nav-link {
		color: var(--text-dim);
		padding: 0.35rem 0.7rem;
		border-radius: 8px;
		font-weight: 550;
	}
	.nav-link:hover {
		color: var(--text);
		background: var(--bg-elev-2);
	}
	.nav-link.active {
		color: var(--accent);
		background: var(--bg-elev-2);
	}
	.user {
		margin-left: auto;
		display: flex;
		align-items: center;
		gap: 0.6rem;
	}
</style>
