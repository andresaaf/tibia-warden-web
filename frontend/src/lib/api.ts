import type {
	Announcement,
	Creature,
	DiscordRole,
	Group,
	GroupMember,
	InviteCode,
	ResponseStatus,
	User
} from './types';

/** Error thrown for non-2xx API responses, carrying the HTTP status. */
export class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
	const res = await fetch(path, {
		method,
		credentials: 'same-origin',
		headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
		body: body !== undefined ? JSON.stringify(body) : undefined
	});

	if (!res.ok) {
		let message = res.statusText;
		try {
			const data = await res.json();
			if (data && typeof data.error === 'string') message = data.error;
		} catch {
			// ignore JSON parse failures
		}
		throw new ApiError(res.status, message);
	}

	if (res.status === 204) return undefined as T;
	const text = await res.text();
	return text ? (JSON.parse(text) as T) : (undefined as T);
}

export const api = {
	// Auth / profile
	me: () => request<User>('GET', '/api/me'),
	updateCharacterName: (characterName: string) =>
		request<User>('PATCH', '/api/me', { characterName }),
	logout: () => request<void>('POST', '/api/auth/logout'),

	// Warden list
	creatures: (search: string, difficulties: string[]) => {
		const params = new URLSearchParams();
		if (search) params.set('search', search);
		if (difficulties.length) params.set('difficulty', difficulties.join(','));
		const qs = params.toString();
		return request<Creature[]>('GET', `/api/creatures${qs ? `?${qs}` : ''}`);
	},
	markKilled: (creatureId: number) => request<void>('PUT', `/api/wardens/${creatureId}`),
	unmarkKilled: (creatureId: number) => request<void>('DELETE', `/api/wardens/${creatureId}`),

	// Groups
	listGroups: (scope: 'public' | 'mine', search = '') => {
		const params = new URLSearchParams();
		if (scope === 'mine') params.set('scope', 'mine');
		if (search) params.set('search', search);
		const qs = params.toString();
		return request<Group[]>('GET', `/api/groups${qs ? `?${qs}` : ''}`);
	},
	createGroup: (name: string, description: string, visibility: string) =>
		request<Group>('POST', '/api/groups', { name, description, visibility }),
	getGroup: (id: number) => request<Group>('GET', `/api/groups/${id}`),
	joinPublic: (id: number) => request<void>('POST', `/api/groups/${id}/join`),
	redeemInvite: (code: string) => request<{ groupId: number }>('POST', '/api/groups/join', { code }),
	leaveGroup: (id: number) => request<void>('POST', `/api/groups/${id}/leave`),
	members: (id: number) => request<GroupMember[]>('GET', `/api/groups/${id}/members`),
	setRole: (id: number, userId: number, role: string) =>
		request<void>('PATCH', `/api/groups/${id}/members/${userId}`, { role }),
	removeMember: (id: number, userId: number) =>
		request<void>('DELETE', `/api/groups/${id}/members/${userId}`),
	invites: (id: number) => request<InviteCode[]>('GET', `/api/groups/${id}/invites`),
	createInvite: (id: number, expiresInHours = 0, maxUses = 1) =>
		request<InviteCode>('POST', `/api/groups/${id}/invites`, { expiresInHours, maxUses }),
	deleteInvite: (id: number, inviteId: number) =>
		request<void>('DELETE', `/api/groups/${id}/invites/${inviteId}`),
	createDiscordLinkCode: (id: number) =>
		request<{ code: string; expiresAt: string }>('POST', `/api/groups/${id}/discord/link-code`),
	unlinkDiscord: (id: number) => request<void>('DELETE', `/api/groups/${id}/discord`),
	discordRoles: (id: number) => request<DiscordRole[]>('GET', `/api/groups/${id}/discord/roles`),
	setDiscordRole: (id: number, roleId: string, roleName: string) =>
		request<void>('PUT', `/api/groups/${id}/discord/role`, { roleId, roleName }),
	clearDiscordRole: (id: number) => request<void>('DELETE', `/api/groups/${id}/discord/role`),
	setDiscordAutodelete: (id: number, seconds: number) =>
		request<void>('PUT', `/api/groups/${id}/discord/autodelete`, { seconds }),

	// Announcements
	announcements: (groupId: number) =>
		request<Announcement[]>('GET', `/api/groups/${groupId}/announcements`),
	feed: () => request<Announcement[]>('GET', '/api/feed'),
	broadcastAnnouncement: (payload: {
		creatureId: number;
		note: string;
		goldCost: number;
		groupIds?: number[];
	}) => request<Announcement[]>('POST', '/api/announcements/broadcast', payload),
	createAnnouncement: (
		groupId: number,
		payload: { creatureId: number; location: string; note: string; goldCost: number }
	) => request<Announcement>('POST', `/api/groups/${groupId}/announcements`, payload),
	setResponse: (announcementId: number, status: ResponseStatus) =>
		request<void>('POST', `/api/announcements/${announcementId}/response`, { status }),
	clearResponse: (announcementId: number) =>
		request<void>('DELETE', `/api/announcements/${announcementId}/response`),
	markAnnouncementKilled: (announcementId: number) =>
		request<void>('POST', `/api/announcements/${announcementId}/killed`),
	claimAnnouncement: (announcementId: number) =>
		request<void>('POST', `/api/announcements/${announcementId}/claim`)
};
