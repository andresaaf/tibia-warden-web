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
	CreatedAt       time.Time `json:"createdAt"`
}

type Creature struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Difficulty string `json:"difficulty"`
	ImageURL   string `json:"imageUrl"`
	Killed     bool   `json:"killed"`
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
	AuthorID         int64                  `json:"authorId"`
	AuthorName       string                 `json:"authorName"`
	Location         string                 `json:"location"`
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
}

type AnnouncementClaim struct {
	UserID        int64  `json:"userId"`
	CharacterName string `json:"characterName"`
}

// DiscordRole is a selectable role in a linked Discord guild.
type DiscordRole struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Mentionable bool   `json:"mentionable"`
}
