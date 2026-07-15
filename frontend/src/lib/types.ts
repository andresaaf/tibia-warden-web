export type Difficulty =
	| 'Harmless'
	| 'Trivial'
	| 'Easy'
	| 'Medium'
	| 'Hard'
	| 'Challenging';

export const DIFFICULTIES: Difficulty[] = [
	'Harmless',
	'Trivial',
	'Easy',
	'Medium',
	'Hard',
	'Challenging'
];

export interface User {
	id: number;
	discordId: string;
	discordUsername: string;
	discordAvatar: string;
	characterName: string;
	createdAt: string;
}

export interface Creature {
	id: number;
	name: string;
	difficulty: Difficulty;
	imageUrl: string;
	killed: boolean;
}

export type Visibility = 'public' | 'private';
export type Role = 'owner' | 'admin' | 'member';

export interface Group {
	id: number;
	name: string;
	description: string;
	visibility: Visibility;
	ownerId: number;
	createdAt: string;
	memberCount: number;
	role?: Role | '';
	discordGuildId?: string;
	discordChannelId?: string;
}

export interface GroupMember {
	userId: number;
	characterName: string;
	discordName: string;
	role: Role;
	joinedAt: string;
}

export interface InviteCode {
	id: number;
	groupId: number;
	code: string;
	createdBy: number;
	usedBy?: number | null;
	usedAt?: string | null;
	expiresAt?: string | null;
	createdAt: string;
}

export type AnnouncementStatus = 'open' | 'killed';
export type ResponseStatus = 'coming' | 'ready';

export interface AnnouncementResponse {
	userId: number;
	characterName: string;
	status: ResponseStatus;
}

export interface AnnouncementClaim {
	userId: number;
	characterName: string;
}

export interface Announcement {
	id: number;
	groupId: number;
	creatureId: number;
	creatureName: string;
	authorId: number;
	authorName: string;
	location: string;
	note: string;
	goldCost: number;
	status: AnnouncementStatus;
	killedAt?: string | null;
	createdAt: string;
	responses: AnnouncementResponse[];
	claims: AnnouncementClaim[];
}
