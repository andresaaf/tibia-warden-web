// Respawn checker timing. A wild echo can respawn in a subarea 20-30 minutes
// after someone stepped on it: red while it can't, yellow while it may, and
// ready (neutral) once the full cooldown has passed. COOLDOWN_MIN mirrors
// checkWindow in backend internal/store/checker.go.
export const COOLDOWN_MIN = 30;
export const WINDOW_MIN = 20;

const COOLDOWN_MS = COOLDOWN_MIN * 60_000;
const WINDOW_MS = WINDOW_MIN * 60_000;

export type CheckStatus = 'ready' | 'yellow' | 'red';

export interface StatusInfo {
	status: CheckStatus;
	/** Milliseconds until the full cooldown ends (0 when ready). */
	remainingMs: number;
	/** Fraction of the cooldown elapsed, 0..1 (1 when ready). */
	progress: number;
}

/** Status of a subarea whose latest visible check was at checkedAtMs (null = never). */
export function statusOf(checkedAtMs: number | null, now: number): StatusInfo {
	if (checkedAtMs === null) return { status: 'ready', remainingMs: 0, progress: 1 };
	const elapsed = Math.max(0, now - checkedAtMs);
	if (elapsed >= COOLDOWN_MS) return { status: 'ready', remainingMs: 0, progress: 1 };
	return {
		status: elapsed < WINDOW_MS ? 'red' : 'yellow',
		remainingMs: COOLDOWN_MS - elapsed,
		progress: elapsed / COOLDOWN_MS
	};
}

/** Whether a check made at checkedAtMs still counts at `now`. */
export function isActive(checkedAtMs: number, now: number): boolean {
	return now - checkedAtMs < COOLDOWN_MS;
}

const RANK: Record<CheckStatus, number> = { ready: 0, yellow: 1, red: 2 };

/** Actionable first: ready (keeps the incoming order), then yellow (closest to
 * ready first), then red (soonest to expire first). Use with a stable sort. */
export function compareByStatus(a: StatusInfo, b: StatusInfo): number {
	return RANK[a.status] - RANK[b.status] || a.remainingMs - b.remainingMs;
}

/** Countdown as m:ss. */
export function formatRemaining(ms: number): string {
	const total = Math.ceil(ms / 1000);
	const m = Math.floor(total / 60);
	const s = total % 60;
	return `${m}:${s.toString().padStart(2, '0')}`;
}

/** Compact "how long ago", e.g. "just now", "6 min ago". */
export function formatAgo(ms: number): string {
	const min = Math.floor(ms / 60_000);
	return min < 1 ? 'just now' : `${min} min ago`;
}
