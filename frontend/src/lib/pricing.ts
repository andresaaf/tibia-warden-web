import type { Difficulty, Group, Rarity } from '$lib/types';

/** Pay-to-attend price (gold per attendee) a group charges for a Warden of the
 *  given difficulty/rarity; 0 when free. Mirrors the snapshot computed in
 *  AnnouncementStore.Create: difficulty price, × the uncommon multiplier for
 *  Uncommon creatures, rounded to whole gold. */
export function attendPriceFor(group: Group, difficulty: Difficulty, rarity: Rarity | ''): number {
	if (group.accessMode !== 'pay_to_attend') return 0;
	const base = group.attendPrices?.[difficulty] ?? 0;
	if (rarity !== 'Uncommon') return base;
	// Integer maths (the multiplier has 2 decimals) so rounding matches Postgres.
	const pct = Math.round((group.uncommonMultiplier ?? 1) * 100);
	return Math.round((base * pct) / 100);
}
