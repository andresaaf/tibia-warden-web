import type { Announcement } from './types';

export type RoomEvent =
	| { type: 'announcement.created'; payload: Announcement }
	| { type: 'announcement.updated'; payload: Announcement };

/**
 * LiveRoom maintains a resilient WebSocket connection to a live announcement
 * feed at a given path, reconnecting automatically with backoff.
 */
class LiveRoom {
	private ws: WebSocket | null = null;
	private closed = false;
	private retry = 0;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

	constructor(
		private path: string,
		private onEvent: (event: RoomEvent) => void
	) {}

	connect() {
		this.closed = false;
		this.open();
	}

	private open() {
		const proto = location.protocol === 'https:' ? 'wss' : 'ws';
		const url = `${proto}://${location.host}${this.path}`;
		const ws = new WebSocket(url);
		this.ws = ws;

		ws.onopen = () => {
			this.retry = 0;
		};
		ws.onmessage = (ev) => {
			try {
				const data = JSON.parse(ev.data) as RoomEvent;
				this.onEvent(data);
			} catch {
				// ignore malformed messages
			}
		};
		ws.onclose = () => {
			if (this.closed) return;
			this.scheduleReconnect();
		};
		ws.onerror = () => {
			ws.close();
		};
	}

	private scheduleReconnect() {
		const delay = Math.min(1000 * 2 ** this.retry, 15000);
		this.retry++;
		this.reconnectTimer = setTimeout(() => this.open(), delay);
	}

	close() {
		this.closed = true;
		if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
		this.ws?.close();
		this.ws = null;
	}
}

/** Live updates for a single group room. */
export class GroupRoom extends LiveRoom {
	constructor(groupId: number, onEvent: (event: RoomEvent) => void) {
		super(`/api/groups/${groupId}/ws`, onEvent);
	}
}

/** Live updates across all of the user's groups (home feed). */
export class FeedRoom extends LiveRoom {
	constructor(onEvent: (event: RoomEvent) => void) {
		super('/api/feed/ws', onEvent);
	}
}
