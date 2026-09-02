package models

import "time"

// Difficulty enumerates the Tibia Bestiary difficulty classes.
const (
	DifficultyHarmless    = "Harmless"
	DifficultyTrivial     = "Trivial"
	DifficultyEasy        = "Easy"
	DifficultyMedium      = "Medium"
	DifficultyHard        = "Hard"
	DifficultyChallenging = "Challenging"
)

// Rarity enumerates the creature occurrences we track (from TibiaWiki).
const (
	RarityCommon   = "Common"
	RarityUncommon = "Uncommon"
)

// Group visibility values.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// Group member roles.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Announcement statuses.
const (
	StatusOpen   = "open"
	StatusKilled = "killed"
)

// Announcement response statuses.
const (
	ResponseComing = "coming"
	ResponseReady  = "ready"
)

type User struct {
	ID              int64     `json:"id"`
	DiscordID       string    `json:"discordId"`
	DiscordUsername string    `json:"discordUsername"`
	DiscordAvatar   string    `json:"discordAvatar"`
	CharacterName   string    `json:"characterName"`
	IsAdmin         bool      `json:"isAdmin"`
	Banned          bool      `json:"banned"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Creature struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Difficulty string `json:"difficulty"`
	Rarity     string `json:"rarity"`
	ImageURL   string `json:"imageUrl"`
	Killed     bool   `json:"killed"`
}

// Subarea is one echo-raid spawn point within an area. The same creature may
// appear in several subareas (and across areas); each nested Creature carries the
// requesting user's killed state.
type Subarea struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Creatures []Creature `json:"creatures"`
}

// Area groups the Echo Warden creatures found at one in-game location. The same
// creature may appear in several areas. Creatures is the DISTINCT union across
// the area's subareas (for the Areas view); Subareas breaks the same data out per
// spawn (for the Subareas view). Each nested Creature carries the requesting
// user's killed state, so the client can compute area/subarea completion.
type Area struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Creatures []Creature `json:"creatures"`
	Subareas  []Subarea  `json:"subareas"`
}

// HighscoreEntry is one row of the statistics leaderboard: a user's total
// killed Wardens, the Charm Points those kills are worth, and the Charm Points
// they've "given away" as an announcer (deduped per broadcast).
type HighscoreEntry struct {
	UserID        int64  `json:"userId"`
	CharacterName string `json:"characterName"`
	Kills         int    `json:"kills"`
	CharmPoints   int    `json:"charmPoints"`
	Score         int    `json:"score"`
}

// CreatureSighting is one row of the Warden sightings panel: how often a
// creature has been announced across every group (a multi-group broadcast counts
// once, deduped by broadcast_id), how many players have it ticked on their
// Warden List, and when it was last announced. Creatures nobody has ever
// announced are returned too, with zeroes.
type CreatureSighting struct {
	CreatureID  int64      `json:"creatureId"`
	Name        string     `json:"name"`
	Difficulty  string     `json:"difficulty"`
	Rarity      string     `json:"rarity"`
	ImageURL    string     `json:"imageUrl"`
	CharmPoints int        `json:"charmPoints"`
	Sightings   int        `json:"sightings"`
	Hunters     int        `json:"hunters"`
	LastSeen    *time.Time `json:"lastSeen"`
}

type Group struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	OwnerID     int64     `json:"ownerId"`
	CreatedAt   time.Time `json:"createdAt"`
	MemberCount int       `json:"memberCount"`
	// Role is the requesting user's role in the group, when applicable.
	Role string `json:"role,omitempty"`
	// Discord link (populated on single-group fetches).
	DiscordGuildID   string `json:"discordGuildId,omitempty"`
	DiscordChannelID string `json:"discordChannelId,omitempty"`
	DiscordRoleID    string `json:"discordRoleId,omitempty"`
	DiscordRoleName  string `json:"discordRoleName,omitempty"`
	// DiscordAutodeleteSeconds: -1 Never, 0 immediately on kill, else seconds after kill.
	DiscordAutodeleteSeconds int `json:"discordAutodeleteSeconds"`
}

type GroupMember struct {
	UserID        int64     `json:"userId"`
	CharacterName string    `json:"characterName"`
	DiscordName   string    `json:"discordName"`
	Role          string    `json:"role"`
	JoinedAt      time.Time `json:"joinedAt"`
	// Share ratio inputs, scoped to this group: Attended = killed announcements
	// the member claimed or reacted 'ready' to; Announced = announcements the
	// member authored themselves. The frontend renders Attended/Announced.
	Attended  int `json:"attended"`
	Announced int `json:"announced"`
	// Charm-weighted equivalents of Attended/Announced (sum of each Warden's
	// charm value) for the roster's Count/Charm toggle.
	AttendedCharm  int `json:"attendedCharm"`
	AnnouncedCharm int `json:"announcedCharm"`
	// Score is the charm value this member has "given away" as an announcer,
	// within the requested period: for each killed announcement they authored,
	// the creature's charm weight times the number of other members who claimed
	// it or reacted Ready.
	Score int `json:"score"`
}

type InviteCode struct {
	ID        int64      `json:"id"`
	GroupID   int64      `json:"groupId"`
	Code      string     `json:"code"`
	CreatedBy int64      `json:"createdBy"`
	UsedBy    *int64     `json:"usedBy,omitempty"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	// MaxUses is nil for unlimited-use codes; UseCount tracks redemptions.
	MaxUses  *int `json:"maxUses"`
	UseCount int  `json:"useCount"`
}

type Announcement struct {
	ID               int64                  `json:"id"`
	GroupID          int64                  `json:"groupId"`
	CreatureID       int64                  `json:"creatureId"`
	CreatureName     string                 `json:"creatureName"`
	CreatureImageURL string                 `json:"creatureImageUrl,omitempty"`
	Difficulty       string                 `json:"difficulty"`
	// CharmPoints is the difficulty-weighted charm value of this Warden, derived
	// from charm_weights. Surfaced for display and future pay-per-charm pricing.
	CharmPoints int `json:"charmPoints"`
	AuthorID         int64                  `json:"authorId"`
	AuthorName       string                 `json:"authorName"`
	Location         string                 `json:"location"`
	// MapX, MapY, MapZ are the optional marked map spot (absolute Tibia world
	// coordinates and floor). All nil together when no spot was marked.
	MapX             *int                   `json:"mapX,omitempty"`
	MapY             *int                   `json:"mapY,omitempty"`
	MapZ             *int                   `json:"mapZ,omitempty"`
	Note             string                 `json:"note"`
	GoldCost         int                    `json:"goldCost"`
	Status           string                 `json:"status"`
	KilledAt         *time.Time             `json:"killedAt,omitempty"`
	CreatedAt        time.Time              `json:"createdAt"`
	Responses        []AnnouncementResponse `json:"responses"`
	Claims           []AnnouncementClaim    `json:"claims"`
	// DiscordMessageID is the mirrored Discord message, when the group is linked.
	DiscordMessageID string `json:"-"` // GroupName and ViewerRole are populated for the aggregated home feed.
	GroupName        string `json:"groupName,omitempty"`
	ViewerRole       string `json:"viewerRole,omitempty"`
	// BroadcastID links announcements from one multi-group broadcast (home feed grouping).
	BroadcastID *string `json:"broadcastId,omitempty"`
}

type AnnouncementResponse struct {
	UserID        int64  `json:"userId"`
	CharacterName string `json:"characterName"`
	Status        string `json:"status"`
	// DiscordID and Registered are bot-only (hidden from the web API): the bot
	// renders unregistered responders by their Discord server nickname.
	DiscordID  string `json:"-"` // for server-nickname lookup by the Discord bot
	Registered bool   `json:"-"` // true when the user has a website character name
}

type AnnouncementClaim struct {
	UserID        int64  `json:"userId"`
	CharacterName string `json:"characterName"`
	// DiscordID and Registered are bot-only (hidden from the web API); see
	// AnnouncementResponse.
	DiscordID  string `json:"-"`
	Registered bool   `json:"-"`
}

// DiscordRole is a selectable role in a linked Discord guild.
type DiscordRole struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Mentionable bool   `json:"mentionable"`
}
