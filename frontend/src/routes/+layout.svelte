<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { currentUser, authLoading, loadCurrentUser, logout } from '$lib/stores';

	let { children } = $props();

	let menuOpen = $state(false);

	onMount(() => {
		loadCurrentUser().catch(() => {});
	});

	async function handleLogout() {
		menuOpen = false;
		await logout();
		goto('/');
	}

	const navItems = [
		{ href: '/', label: 'Home', short: 'Home' },
		{ href: '/groups', label: 'Groups', short: 'Groups' },
		{ href: '/wardens', label: 'Warden List', short: 'Wardens' },
		{ href: '/statistics', label: 'Statistics', short: 'Stats' }
	];

	function isActive(pathname: string, href: string): boolean {
		return href === '/' ? pathname === '/' : pathname.startsWith(href);
	}
</script>

<div class="app">
	<header class="topbar">
		<div class="topbar-inner">
			<a class="brand" href="/">
				<span class="brand-mark">◈</span><span class="brand-text"> Echo Warden Tracker</span>
			</a>

			{#if $currentUser}
				<nav class="nav">
					{#each navItems as item}
						<a
							class="nav-link"
							class:active={isActive($page.url.pathname, item.href)}
							href={item.href}
							><span class="lbl-full">{item.label}</span><span class="lbl-short"
								>{item.short}</span
							></a
						>
					{/each}
				</nav>
				<button
					class="menu-toggle"
					aria-label="Account menu"
					aria-expanded={menuOpen}
					onclick={() => (menuOpen = !menuOpen)}>☰</button
				>
				<div class="user" class:open={menuOpen}>
					<a
						class="user-name"
						href="/settings"
						title="Account settings"
						onclick={() => (menuOpen = false)}
					>
						{$currentUser.characterName || $currentUser.discordUsername}
					</a>
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
		white-space: nowrap;
	}
	.lbl-short {
		display: none;
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
	.user-name {
		color: var(--text-dim);
		font-weight: 550;
		padding: 0.25rem 0.5rem;
		border-radius: 8px;
	}
	.user-name:hover {
		color: var(--text);
		background: var(--bg-elev-2);
	}
	.menu-toggle {
		display: none;
		background: transparent;
		border: none;
		color: var(--text-dim);
		font-size: 1.2rem;
		line-height: 1;
		padding: 0.35rem 0.55rem;
		border-radius: 8px;
	}
	.menu-toggle:hover {
		color: var(--text);
		background: var(--bg-elev-2);
	}

	@media (max-width: 700px) {
		.topbar-inner {
			flex-wrap: wrap;
			padding: 0.6rem 1rem;
			gap: 0.5rem;
		}
		.brand-text {
			display: none;
		}
		.nav {
			margin-left: 0.25rem;
			gap: 0.15rem;
		}
		.nav-link {
			padding: 0.35rem 0.5rem;
		}
		.lbl-full {
			display: none;
		}
		.lbl-short {
			display: inline;
		}
		.menu-toggle {
			display: inline-flex;
			margin-left: auto;
		}
		.user {
			display: none;
			margin-left: 0;
		}
		.user.open {
			display: flex;
			flex-direction: column;
			align-items: stretch;
			flex-basis: 100%;
			gap: 0.5rem;
			padding-top: 0.6rem;
			margin-top: 0.1rem;
			border-top: 1px solid var(--border);
		}
	}
</style>
