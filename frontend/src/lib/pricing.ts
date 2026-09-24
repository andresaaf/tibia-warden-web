import type { Difficulty, Group, Rarity } from '$lib/types';

/** Pay-to-attend price (gold per attendee) a group charges for a Warden of the
 *  given difficulty and rarity; 0 when free. Mirrors the snapshot computed in
 *  AnnouncementStore.Create. Creatures with no synced rarity count as Common. */
export function attendPriceFor(group: Group, difficulty: Difficulty, rarity: Rarity | ''): number {
	if (group.accessMode !== 'pay_to_attend') return 0;
	const column = rarity === 'Uncommon' ? 'Uncommon' : 'Common';
	return group.attendPrices?.[column]?.[difficulty] ?? 0;
}
