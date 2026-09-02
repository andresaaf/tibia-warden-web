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

/** Compact "time since" label for a timestamp: "just now", "5m", "3h", "12d",
 *  "8mo", "2y". Meant for dense table cells — pair it with a title showing the
 *  full local date. */
export function formatAgo(iso: string): string {
	const then = new Date(iso).getTime();
	if (Number.isNaN(then)) return '';
	const secs = Math.max(0, Math.round((Date.now() - then) / 1000));
	if (secs < 60) return 'just now';
	const mins = Math.floor(secs / 60);
	if (mins < 60) return `${mins}m`;
	const hours = Math.floor(mins / 60);
	if (hours < 24) return `${hours}h`;
	const days = Math.floor(hours / 24);
	if (days < 30) return `${days}d`;
	const months = Math.floor(days / 30);
	if (months < 12) return `${months}mo`;
	return `${Math.floor(days / 365)}y`;
}
