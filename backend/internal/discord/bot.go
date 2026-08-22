// Package discord implements the optional announcement bot that mirrors group
// Echo Warden announcements into a linked Discord channel and keeps them in sync
// with the website in both directions.
package discord

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/config"
	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
	"github.com/andresaaf/tibia-warden-web/backend/internal/ws"
	"github.com/bwmarrin/discordgo"
)

// ErrBotDisabled is returned when a bot operation is attempted while the bot is
// not configured.
var ErrBotDisabled = errors.New("discord bot is not enabled")

// Bot wraps a Discord gateway session and the shared application state it needs
// to record interactions and broadcast live updates back to website clients.
type Bot struct {
	session *discordgo.Session
	stores  *store.Stores
	hub     *ws.Hub
	appID   string
	siteURL string // PublicBaseURL, linked in the "register" nudge

	// nickMu guards nickCache, a short-lived cache of guild server nicknames
	// (keyed "guildID:discordID") so repainting a message on every button click
	// doesn't issue a REST lookup per unregistered responder each time.
	nickMu    sync.Mutex
	nickCache map[string]nickEntry
}

// nickEntry is a cached guild nickname lookup with the time it was fetched.
type nickEntry struct {
	name string
	at   time.Time
}

// nickTTL is how long a cached server nickname is reused before re-fetching.
const nickTTL = 10 * time.Minute

// New constructs the bot. It returns (nil, nil) when no bot token is configured,
// leaving the application to run without Discord integration.
func New(cfg *config.Config, stores *store.Stores, hub *ws.Hub) (*Bot, error) {
	if cfg.DiscordBotToken == "" {
		return nil, nil
	}
	session, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentsGuilds

	b := &Bot{session: session, stores: stores, hub: hub, siteURL: cfg.PublicBaseURL, nickCache: map[string]nickEntry{}}
	session.AddHandler(b.onReady)
	session.AddHandler(b.onInteraction)
	return b, nil
}

// Start opens the gateway connection and launches the auto-delete sweeper.
// Safe to call on a nil bot.
func (b *Bot) Start(ctx context.Context) error {
	if b == nil {
		return nil
	}
	if err := b.session.Open(); err != nil {
		return err
	}
	go b.runSweeper(ctx)
	return nil
}

// Stop closes the gateway connection. Safe to call on a nil bot.
func (b *Bot) Stop() {
	if b == nil || b.session == nil {
		return
	}
	_ = b.session.Close()
}

// onReady records the application ID and (re)registers slash commands.
func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	b.appID = r.User.ID
	_, err := s.ApplicationCommandCreate(b.appID, "", &discordgo.ApplicationCommand{
		Name:        "link",
		Description: "Link this channel to a Tibia Warden group using a code from the website",
		Options: []*discordgo.ApplicationCommandOption{{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "code",
			Description: "The link code from your group's Discord settings",
			Required:    true,
		}},
	})
	if err != nil {
		slog.Error("discord: failed to register /link command", "error", err)
		return
	}
	slog.Info("discord bot ready", "user", r.User.Username)
}

// onInteraction dispatches slash commands and button clicks.
func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("discord interaction panic", "recover", rec)
		}
	}()
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(s, i)
	case discordgo.InteractionMessageComponent:
		b.handleComponent(s, i)
	}
}

// handleCommand processes the /link slash command.
func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "link" || len(data.Options) == 0 {
		return
	}
	if i.Member == nil || i.Member.User == nil {
		b.ephemeral(s, i, "Use this command inside a server channel.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code := strings.TrimSpace(data.Options[0].StringValue())
	groupID, err := b.stores.Groups.PeekDiscordLinkCode(ctx, code)
	if err != nil {
		b.ephemeral(s, i, "That link code is invalid or has expired. Generate a new one on the website.")
		return
	}

	user := i.Member.User
	u, err := b.stores.Users.UpsertByDiscord(ctx, user.ID, user.Username, user.AvatarURL(""))
	if err != nil {
		b.ephemeral(s, i, "Something went wrong. Please try again.")
		return
	}
	role, err := b.stores.Groups.Role(ctx, groupID, u.ID)
	if err != nil || (role != models.RoleOwner && role != models.RoleAdmin) {
		b.ephemeral(s, i, "Only an owner or admin of the group can link it. Log into the website with this Discord account first.")
		return
	}

	if err := b.stores.Groups.SetDiscordLink(ctx, groupID, i.GuildID, i.ChannelID); err != nil {
		b.ephemeral(s, i, "Failed to link this channel. Please try again.")
		return
	}
	_, _ = b.stores.Groups.ConsumeDiscordLinkCode(ctx, code)
	b.ephemeral(s, i, "✅ Linked this channel. New Echo Warden announcements from your group will appear here.")
}

// handleComponent processes Coming / Ready / Killed / Got-kill button clicks.
func (b *Bot) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, ":")
	if len(parts) != 3 || parts[0] != "ann" {
		return
	}
	annID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}
	action := parts[2]

	if i.Member == nil || i.Member.User == nil {
		b.ephemeral(s, i, "Use this inside a server channel.")
		return
	}

	// Acknowledge the interaction immediately. Discord discards the interaction
	// token ~3s after the click, but the database work below (a kill can run
	// several sequential queries) may take longer. Deferring first, then editing
	// the message once the work is done, keeps us inside that window — otherwise
	// the response is rejected as an "Unknown interaction", the mirrored message
	// is never repainted to the killed/claim state, and the post lingers in its
	// stale "open" form until auto-delete silently removes it.
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		slog.Error("discord: failed to acknowledge interaction", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := i.Member.User
	u, err := b.stores.Users.UpsertByDiscord(ctx, user.ID, user.Username, user.AvatarURL(""))
	if err != nil {
		b.followupError(s, i, "Something went wrong. Please try again.")
		return
	}

	ann, err := b.stores.Announcements.GetByID(ctx, annID)
	if err != nil {
		b.followupError(s, i, "That announcement no longer exists.")
		return
	}

	var affected []int64
	switch action {
	case models.ResponseComing, models.ResponseReady:
		current := ""
		for _, r := range ann.Responses {
			if r.UserID == u.ID {
				current = r.Status
			}
		}
		if current == action {
			affected, _ = b.stores.Announcements.ClearResponse(ctx, annID, u.ID)
		} else {
			affected, _ = b.stores.Announcements.SetResponse(ctx, annID, u.ID, action)
		}
	case "killed":
		role, _ := b.stores.Groups.Role(ctx, ann.GroupID, u.ID)
		if u.ID != ann.AuthorID && role != models.RoleOwner && role != models.RoleAdmin {
			b.followupError(s, i, "Only the person who announced it or a group admin can mark it killed.")
			return
		}
		affected, err = b.stores.Announcements.MarkKilledWithSiblings(ctx, annID)
		if err != nil {
			b.followupError(s, i, "This is already marked killed.")
			return
		}
	case "claim":
		if err := b.stores.Announcements.Claim(ctx, annID, u.ID); err != nil {
			b.followupError(s, i, "This can only be claimed after it's marked killed.")
			return
		}
	default:
		return
	}

	// Nudge Discord-only users to register: they can still take part, but keep
	// getting this ephemeral (and miss charm-point credit) until they link a
	// Tibia character on the website.
	if u.CharacterName == "" {
		b.nudgeUnregistered(s, i)
	}

	ann, err = b.stores.Announcements.GetByID(ctx, annID)
	if err != nil {
		return
	}
	b.hub.Broadcast(ann.GroupID, ws.EventAnnouncementUpdated, ann)

	// Repaint the clicked message via the deferred interaction's webhook.
	guildID, _, roleID, _ := b.stores.Groups.DiscordSettings(ctx, ann.GroupID)
	content := messageContent(roleID, ann)
	embeds := []*discordgo.MessageEmbed{b.buildEmbed(ann, guildID)}
	components := buildComponents(ann)
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:         &content,
		Embeds:          &embeds,
		Components:      &components,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}); err != nil {
		slog.Error("discord: failed to update message after interaction", "error", err)
	}

	// Propagate to any affected broadcast siblings (kills, and responses that
	// cascade to groups where the user had already reacted). The primary was
	// already pushed above via the interaction edit and hub broadcast.
	for _, id := range affected {
		if id != annID {
			if sib, sErr := b.stores.Announcements.GetByID(ctx, id); sErr == nil {
				b.hub.Broadcast(sib.GroupID, ws.EventAnnouncementUpdated, sib)
				b.SyncAnnouncement(ctx, sib)
			}
		}
		if action == "killed" {
			b.OnAnnouncementKilled(ctx, id)
		}
	}
}

// PostAnnouncement posts a new announcement to the group's linked channel (if any)
// and records the resulting message ID. Safe to call on a nil bot.
func (b *Bot) PostAnnouncement(ctx context.Context, ann *models.Announcement) {
	if b == nil || b.session == nil || ann == nil {
		return
	}
	guildID, channelID, roleID, err := b.stores.Groups.DiscordSettings(ctx, ann.GroupID)
	if err != nil || channelID == "" {
		return
	}
	send := &discordgo.MessageSend{
		Content:    messageContent(roleID, ann),
		Embeds:     []*discordgo.MessageEmbed{b.buildEmbed(ann, guildID)},
		Components: buildComponents(ann),
	}
	if roleID != "" {
		send.AllowedMentions = &discordgo.MessageAllowedMentions{Roles: []string{roleID}}
	}
	msg, err := b.session.ChannelMessageSendComplex(channelID, send)
	if err != nil && roleID != "" {
		// The mention may be rejected (e.g. missing permission); retry without it
		// so the announcement is still mirrored (creature name is kept).
		slog.Warn("discord: post with role mention failed, retrying without", "error", err)
		send.Content = messageContent("", ann)
		send.AllowedMentions = nil
		msg, err = b.session.ChannelMessageSendComplex(channelID, send)
	}
	if err != nil {
		slog.Error("discord: failed to post announcement", "error", err)
		return
	}
	if err := b.stores.Announcements.SetDiscordMessageID(ctx, ann.ID, msg.ID); err != nil {
		slog.Error("discord: failed to store message id", "error", err)
	}
}

// SyncAnnouncement edits the mirrored Discord message to reflect current state.
// Safe to call on a nil bot or an unlinked announcement.
func (b *Bot) SyncAnnouncement(ctx context.Context, ann *models.Announcement) {
	if b == nil || b.session == nil || ann == nil || ann.DiscordMessageID == "" {
		return
	}
	guildID, channelID, roleID, err := b.stores.Groups.DiscordSettings(ctx, ann.GroupID)
	if err != nil || channelID == "" {
		return
	}
	embeds := []*discordgo.MessageEmbed{b.buildEmbed(ann, guildID)}
	components := buildComponents(ann)
	content := messageContent(roleID, ann)
	if _, err := b.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:         channelID,
		ID:              ann.DiscordMessageID,
		Content:         &content,
		Embeds:          &embeds,
		Components:      &components,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}); err != nil {
		slog.Error("discord: failed to edit announcement message", "error", err)
	}
}

// OnAnnouncementKilled applies the group's auto-delete policy to a mirrored
// message once its announcement is marked killed. Safe to call on a nil bot.
func (b *Bot) OnAnnouncementKilled(ctx context.Context, announcementID int64) {
	if b == nil || b.session == nil {
		return
	}
	ann, err := b.stores.Announcements.GetByID(ctx, announcementID)
	if err != nil || ann.DiscordMessageID == "" {
		return
	}
	seconds, err := b.stores.Groups.DiscordAutodelete(ctx, ann.GroupID)
	if err != nil || seconds < 0 {
		return // never
	}
	if seconds == 0 {
		b.deleteMirrored(ctx, announcementID, ann.GroupID, ann.DiscordMessageID)
		return
	}
	_ = b.stores.Announcements.ScheduleDiscordDelete(ctx, announcementID,
		time.Now().Add(time.Duration(seconds)*time.Second))
}

// reconcileOrphanedKills applies the auto-delete policy to mirrored messages of
// announcements that were marked killed while the bot could not act on them —
// e.g. the process restarted between the kill and its (immediate or scheduled)
// removal, so no discord_delete_at was ever persisted and the sweeper would
// never see them. It runs once at startup. The delay is measured from the actual
// kill time, so an already-overdue message is swept on the next tick.
func (b *Bot) reconcileOrphanedKills(ctx context.Context) {
	if b == nil || b.session == nil {
		return
	}
	orphans, err := b.stores.Announcements.KilledAwaitingDiscordCleanup(ctx)
	if err != nil {
		slog.Error("discord: failed to query orphaned killed announcements", "error", err)
		return
	}
	for _, o := range orphans {
		seconds, err := b.stores.Groups.DiscordAutodelete(ctx, o.GroupID)
		if err != nil || seconds < 0 {
			continue // never
		}
		if seconds == 0 {
			b.deleteMirrored(ctx, o.AnnouncementID, o.GroupID, o.MessageID)
			continue
		}
		_ = b.stores.Announcements.ScheduleDiscordDelete(ctx, o.AnnouncementID,
			o.KilledAt.Add(time.Duration(seconds)*time.Second))
	}
}

// runSweeper periodically deletes mirrored messages whose scheduled delete time
// has passed. Restart-safe because the schedule is persisted in the database. A
// one-time reconcile first re-schedules any kills orphaned across a restart.
func (b *Bot) runSweeper(ctx context.Context) {
	b.reconcileOrphanedKills(ctx)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.sweep(ctx)
		}
	}
}

func (b *Bot) sweep(ctx context.Context) {
	due, err := b.stores.Announcements.DueDiscordDeletes(ctx)
	if err != nil {
		slog.Error("discord: failed to query due deletions", "error", err)
		return
	}
	for _, d := range due {
		b.deleteMirrored(ctx, d.AnnouncementID, d.GroupID, d.MessageID)
	}
}

// deleteMirrored removes a mirrored Discord message and then forgets its ID. The
// pointer is only cleared once we know the message is gone — a successful delete
// or a Discord "Unknown Message" (already deleted). Any other failure (a rate
// limit, a network blip, a transient settings-lookup error) leaves the pointer
// intact and pushes the scheduled delete a few minutes out so the sweeper
// retries, instead of orphaning the message in the channel forever.
func (b *Bot) deleteMirrored(ctx context.Context, announcementID, groupID int64, messageID string) {
	_, channelID, _, err := b.stores.Groups.DiscordSettings(ctx, groupID)
	if err != nil {
		_ = b.stores.Announcements.ScheduleDiscordDelete(ctx, announcementID, time.Now().Add(5*time.Minute))
		return
	}
	if channelID == "" {
		// The channel is no longer linked; there is nothing left to delete.
		_ = b.stores.Announcements.ClearDiscordMessage(ctx, announcementID)
		return
	}
	if err := b.session.ChannelMessageDelete(channelID, messageID); err != nil && !isUnknownMessage(err) {
		slog.Error("discord: failed to delete mirrored message, will retry", "error", err)
		_ = b.stores.Announcements.ScheduleDiscordDelete(ctx, announcementID, time.Now().Add(5*time.Minute))
		return
	}
	_ = b.stores.Announcements.ClearDiscordMessage(ctx, announcementID)
}

// isUnknownMessage reports whether a Discord API error means the message no
// longer exists (already deleted), which we treat the same as a success.
func isUnknownMessage(err error) bool {
	var rest *discordgo.RESTError
	if errors.As(err, &rest) && rest.Message != nil {
		return rest.Message.Code == discordgo.ErrCodeUnknownMessage
	}
	return false
}

// GuildRoles returns the assignable roles of a guild, most-prominent first,
// excluding @everyone and integration-managed roles.
func (b *Bot) GuildRoles(guildID string) ([]models.DiscordRole, error) {
	if b == nil || b.session == nil {
		return nil, ErrBotDisabled
	}
	roles, err := b.session.GuildRoles(guildID)
	if err != nil {
		return nil, err
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Position > roles[j].Position })

	out := make([]models.DiscordRole, 0, len(roles))
	for _, r := range roles {
		if r.ID == guildID || r.Managed {
			continue
		}
		out = append(out, models.DiscordRole{
			ID:          r.ID,
			Name:        r.Name,
			Color:       r.Color,
			Mentionable: r.Mentionable,
		})
	}
	return out, nil
}

func (b *Bot) ephemeral(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

// nudgeUnregistered sends the clicking user a (deliberately repetitive)
// ephemeral reminder that they're taking part as an unregistered Discord user.
// It fires on every reaction from a user with no linked character, so it keeps
// nagging until they register. Only the clicking user sees it.
func (b *Bot) nudgeUnregistered(s *discordgo.Session, i *discordgo.InteractionCreate) {
	msg := "👋 You're taking part as an **unregistered** Discord user. You can still join every hunt — but until you register on the website you won't get automatic tracking of killed wardens, and you'll keep getting this reminder every time you react."
	if b.siteURL != "" {
		msg += "\n\nRegister here: " + b.siteURL
	}
	if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: msg,
		Flags:   discordgo.MessageFlagsEphemeral,
	}); err != nil {
		slog.Error("discord: failed to send register nudge", "error", err)
	}
}

// followupError sends an ephemeral error to the clicking user after the
// interaction has already been acknowledged (deferred). Use this instead of
// ephemeral once handleComponent has deferred, since the interaction can no
// longer take a fresh response — only followups.
func (b *Bot) followupError(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: msg,
		Flags:   discordgo.MessageFlagsEphemeral,
	}); err != nil {
		slog.Error("discord: failed to send followup message", "error", err)
	}
}

func (b *Bot) buildEmbed(a *models.Announcement, guildID string) *discordgo.MessageEmbed {
	color := 0x4eb87a
	status := "🟢 Open"
	if a.Status == models.StatusKilled {
		color = 0x9aa2b1
		status = "💀 Killed"
	}
	if a.Difficulty != "" {
		status += fmt.Sprintf(" — %s ★ %d", a.Difficulty, a.CharmPoints)
	}

	var fields []*discordgo.MessageEmbedField
	if a.Location != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "📍 Location", Value: a.Location})
	}
	if a.Note != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Note", Value: a.Note})
	}
	if a.AuthorName != "" {
		// Copyable exiva command to locate the finder in-game. Discord has no
		// click-to-copy button, but inline-code is tap-to-copy on mobile and
		// easy to select on desktop. Open quote only, matching in-game typing.
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "🔍 Locate finder",
			Value: fmt.Sprintf("`exiva \"%s`", a.AuthorName),
		})
	}
	if coming := b.namesByStatus(a, models.ResponseComing, guildID); coming != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "🏃 Coming", Value: coming, Inline: true})
	}
	if ready := b.namesByStatus(a, models.ResponseReady, guildID); ready != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "✅ Ready", Value: ready, Inline: true})
	}
	if a.Status == models.StatusKilled {
		claims := b.claimNames(a, guildID)
		if claims == "" {
			claims = "—"
		}
		fields = append(fields, &discordgo.MessageEmbedField{Name: "🎯 Got the kill", Value: claims})
	}

	embed := &discordgo.MessageEmbed{
		Title:       a.CreatureName + " — Echo Warden",
		Description: status,
		Color:       color,
		Fields:      fields,
		Footer:      &discordgo.MessageEmbedFooter{Text: "Announced by " + a.AuthorName},
	}
	if a.CreatureImageURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: a.CreatureImageURL}
	}
	return embed
}

func buildComponents(a *models.Announcement) []discordgo.MessageComponent {
	id := strconv.FormatInt(a.ID, 10)
	if a.Status == models.StatusKilled {
		return []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "➕ I got it (tick my list)", Style: discordgo.SuccessButton, CustomID: "ann:" + id + ":claim"},
			}},
		}
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{Label: "🏃 Coming", Style: discordgo.PrimaryButton, CustomID: "ann:" + id + ":coming"},
			discordgo.Button{Label: "✅ Ready", Style: discordgo.SuccessButton, CustomID: "ann:" + id + ":ready"},
			discordgo.Button{Label: "💀 Killed", Style: discordgo.DangerButton, CustomID: "ann:" + id + ":killed"},
		}},
	}
}

// messageContent builds the Discord message text: the group's role ping (when
// configured) followed by the Warden's creature name, so the monster is named in
// the message itself and not only inside the embed. Re-sending it on edits
// preserves the visible mention without triggering another notification.
func messageContent(roleID string, a *models.Announcement) string {
	if roleID == "" {
		return a.CreatureName
	}
	return "<@&" + roleID + "> " + a.CreatureName
}

func (b *Bot) namesByStatus(a *models.Announcement, status, guildID string) string {
	var names []string
	for _, r := range a.Responses {
		if r.Status == status {
			names = append(names, b.displayName(r.Registered, r.CharacterName, r.DiscordID, guildID))
		}
	}
	return strings.Join(names, "\n")
}

func (b *Bot) claimNames(a *models.Announcement, guildID string) string {
	var names []string
	for _, c := range a.Claims {
		names = append(names, b.displayName(c.Registered, c.CharacterName, c.DiscordID, guildID))
	}
	return strings.Join(names, "\n")
}

// displayName renders a responder/claimant for the Discord embed. Registered
// users show their website character name unchanged; unregistered (Discord-only)
// users show their Discord server nickname wrapped in angle brackets — e.g.
// "<Some User>" — so they stay easy to distinguish. When no server nickname can
// be resolved, it falls back to characterName (the global Discord username).
func (b *Bot) displayName(registered bool, characterName, discordID, guildID string) string {
	if registered {
		return characterName
	}
	nick := b.guildNick(guildID, discordID)
	if nick == "" {
		nick = characterName
	}
	return "<" + nick + ">"
}

// guildNick resolves a user's Discord server nickname (falling back to their
// global display name, then username, via Member.DisplayName). Results are
// cached for nickTTL to avoid a REST lookup on every message repaint. It returns
// "" when the guild/user is unknown or the lookup fails, letting the caller fall
// back to a stored name.
func (b *Bot) guildNick(guildID, discordID string) string {
	if guildID == "" || discordID == "" {
		return ""
	}
	key := guildID + ":" + discordID

	b.nickMu.Lock()
	if e, ok := b.nickCache[key]; ok && time.Since(e.at) < nickTTL {
		b.nickMu.Unlock()
		return e.name
	}
	b.nickMu.Unlock()

	name := ""
	member, err := b.session.GuildMember(guildID, discordID)
	if err != nil {
		slog.Debug("discord: guild member lookup failed", "guild", guildID, "user", discordID, "error", err)
	} else if member != nil {
		name = member.DisplayName()
	}

	b.nickMu.Lock()
	b.nickCache[key] = nickEntry{name: name, at: time.Now()}
	b.nickMu.Unlock()
	return name
}
