<script lang="ts">
	import { formatK } from '$lib/format';

	// Pay-to-attend price badge for an announcement. Renders nothing when free.
	let { price }: { price: number } = $props();
</script>

{#if price > 0}
	<span
		class="badge price"
		title={`Pay-to-attend: ${price.toLocaleString()} gold per attendee, paid in-game after the kill`}
		>💰 {formatK(price)} to attend</span
	>
{/if}

<style>
	.price {
		color: var(--accent);
		border-color: var(--accent);
	}
</style>
