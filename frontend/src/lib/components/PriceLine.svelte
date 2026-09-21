<script lang="ts">
	import { formatK } from '$lib/format';

	// Pay-to-attend price line for an announcement card. Renders nothing when free.
	let { price }: { price: number } = $props();
</script>

{#if price > 0}
	<div class="price-line" title={`${price.toLocaleString()} gold per attendee`}>
		💰 Price: <strong>{formatK(price)}</strong> · parcel after kill
	</div>
{/if}

<style>
	.price-line {
		margin-top: 0.2rem;
		font-size: 0.9rem;
	}
	.price-line strong {
		color: var(--accent);
	}
</style>
