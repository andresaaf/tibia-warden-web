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

export type Rarity = 'Common' | 'Uncommon';

export const RARITIES: Rarity[] = ['Common', 'Uncommon'];

export interface User {
	id: number;
	discordId: string;
	discordUsername: string;
	discordAvatar: string;
	characterName: string;
	isAdmin: boolean;
	banned: boolean;
	createdAt: string;
}

export interface Creature {
	id: number;
	name: string;
	difficulty: Difficulty;
	rarity: Rarity;
	imageUrl: string;
	killed: boolean;
}

export interface Subarea {
	id: number;
	name: string;
	creatures: Creature[];
}

export interface Area {
	id: number;
	name: string;
	/** DISTINCT union of the area's subarea creatures (Areas view). */
	creatures: Creature[];
	/** Per-spawn breakdown; duplicates across subareas are kept (Subareas view). */
	subareas: Subarea[];
}

export type Visibility = 'public' | 'private';
export type Role = 'owner' | 'admin' | 'member';
export type RosterPeriod = 'lifetime' | 'current_month' | 'previous_month';

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
	discordRoleId?: string;
	discordRoleName?: string;
	discordAutodeleteSeconds?: number;
}

export interface DiscordRole {
	id: string;
	name: string;
	color: number;
	mentionable: boolean;
}

export interface GroupMember {
	userId: number;
	characterName: string;
	discordName: string;
	role: Role;
	joinedAt: string;
	/** Killed announcements in this group the member claimed or reacted 'ready' to. */
	attended: number;
	/** Announcements in this group the member authored themselves. */
	announced: number;
	/** Charm-weighted equivalents of attended/announced (sum of each Warden's charm value). */
	attendedCharm: number;
	announcedCharm: number;
	/** Charm value "given away" as an announcer (within the group's score window):
	 *  sum over their killed announcements of (Warden charm × others who claimed / reacted Ready). */
	score: number;
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
	maxUses: number | null;
	useCount: number;
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

export interface HighscoreEntry {
	userId: number;
	characterName: string;
	kills: number;
	charmPoints: number;
	/** Charm points "given away" as an announcer, global and deduped per broadcast. */
	score: number;
}

export interface Announcement {
	id: number;
	groupId: number;
	creatureId: number;
	creatureName: string;
	creatureImageUrl?: string;
	difficulty: Difficulty;
	/** Difficulty-weighted charm value of this Warden. */
	charmPoints: number;
	authorId: number;
	authorName: string;
	location: string;
	/** Optional marked map spot: absolute Tibia world coords and floor. All
	 * absent together when no spot was marked. */
	mapX?: number | null;
	mapY?: number | null;
	mapZ?: number | null;
	note: string;
	goldCost: number;
	status: AnnouncementStatus;
	killedAt?: string | null;
	createdAt: string;
	responses: AnnouncementResponse[];
	claims: AnnouncementClaim[];
	groupName?: string;
	viewerRole?: Role | '';
	broadcastId?: string | null;
}
