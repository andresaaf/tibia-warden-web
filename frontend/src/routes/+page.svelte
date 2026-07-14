<script lang="ts">
	import { goto } from '$app/navigation';
	import { currentUser, authLoading } from '$lib/stores';

	// Redirect authenticated users away from the landing page.
	$effect(() => {
		if ($authLoading) return;
		if ($currentUser) {
			goto($currentUser.characterName ? '/groups' : '/onboarding', { replaceState: true });
		}
	});

	let showLogin = $derived(!$authLoading && !$currentUser);
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
{/if}

<style>
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
