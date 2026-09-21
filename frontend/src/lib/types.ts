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

/** Other trackers' file formats the Warden List can be exported to / imported
 * from. Ids match backend/internal/formats. */
export const WARDEN_FORMATS = [
	{ id: 'tibiadraptor', label: 'TibiaDraptor', hint: 'Echo Warden JSON export from tibiadraptor.com' }
] as const;

export type WardenFormat = (typeof WARDEN_FORMATS)[number]['id'];

export interface WardenExport {
	blob: Blob;
	filename: string;
	/** Killed creatures the format has no identifier for (left out of the file). */
	unmapped: string[];
}

/** 'add' only adds marks; 'replace' also unmarks wardens the file doesn't list. */
export type WardenImportMode = 'add' | 'replace';

/** Changes an import makes — or, when dryRun, would make. */
export interface WardenImportResult {
	mode: WardenImportMode;
	dryRun: boolean;
	added: number;
	alreadyMarked: number;
	removed: number;
	/** Marked wardens a replace keeps because the format can't list them. */
	keptUnsupported: number;
	/** Entries in the file that don't match any creature on our list. */
	unknown: number;
	addedNames: string[];
	removedNames: string[];
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
	/** Pay-to-attend: attendees pay attendPrices[difficulty] gold per Warden
	 *  (× uncommonMultiplier for Uncommon creatures), in-game after the kill.
	 *  Prices are kept while the mode is free. */
	accessMode?: AccessMode;
	attendPrices?: Partial<Record<Difficulty, number>>;
	uncommonMultiplier?: number;
}

export type AccessMode = 'free' | 'pay_to_attend';

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

/** One row of the Warden sightings panel on the statistics page. Every creature
 *  is returned, including ones nobody has ever announced (all counts 0). */
export interface CreatureSighting {
	creatureId: number;
	name: string;
	difficulty: Difficulty;
	rarity: Rarity;
	imageUrl: string;
	/** Difficulty-weighted charm value of this Warden. */
	charmPoints: number;
	/** Times announced across all groups; a multi-group broadcast counts once. */
	sightings: number;
	/** Players who have this Warden ticked on their Warden List. */
	hunters: number;
	/** When it was last announced, or null if it never has been. */
	lastSeen: string | null;
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
	/** Pay-to-attend price (gold per attendee) computed from the group's
	 *  pricing when posted; 0 = free. */
	attendPrice: number;
	status: AnnouncementStatus;
	killedAt?: string | null;
	createdAt: string;
	responses: AnnouncementResponse[];
	claims: AnnouncementClaim[];
	groupName?: string;
	viewerRole?: Role | '';
	broadcastId?: string | null;
}
