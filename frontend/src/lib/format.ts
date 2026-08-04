/** Collapse large integers Tibia-style: 1234 → "1.234k", 12000 → "12k",
 *  1000000 → "1kk". Values under 1000 are shown as-is. Up to 3 decimals,
 *  trailing zeros trimmed. */
export function formatK(n: number): string {
	const abs = Math.abs(n);
	if (abs < 1000) return String(n);
	const [div, suffix] = abs < 1_000_000 ? [1000, 'k'] : [1_000_000, 'kk'];
	return (n / div).toFixed(3).replace(/\.?0+$/, '') + suffix;
}
