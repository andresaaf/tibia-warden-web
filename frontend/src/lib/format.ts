/** Collapse large integers Tibia-style into a compact label: 1040 → "1k",
 *  1560 → "1.6k", 12345 → "12k", 1000000 → "1kk", 2500000 → "2.5kk". Values
 *  under 1000 are shown as-is. At most one decimal (only below 10 of a unit),
 *  rounded — pair it with a tooltip showing the exact value. */
export function formatK(n: number): string {
	const abs = Math.abs(n);
	if (abs < 1000) return String(n);
	const [div, suffix] = abs < 1_000_000 ? [1000, 'k'] : [1_000_000, 'kk'];
	const scaled = n / div;
	// One decimal while the scaled value is small, whole numbers once it's ≥ 10,
	// so we never show more than a couple of significant digits.
	const rounded = Math.abs(scaled) < 10 ? Math.round(scaled * 10) / 10 : Math.round(scaled);
	return String(rounded) + suffix;
}
