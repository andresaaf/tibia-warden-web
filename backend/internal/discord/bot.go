// Package discord implements the optional announcement bot that mirrors group
// Echo Warden announcements into a linked Discord channel and keeps them in sync
// with the website in both directions.
package discord

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/baz/tibia-warden-web/backend/internal/config"
	"github.com/baz/tibia-warden-web/backend/internal/models"
	"github.com/baz/tibia-warden-web/backend/internal/store"
	"github.com/baz/tibia-warden-web/backend/internal/ws"
	"github.com/bwmarrin/discordgo"
)

// Bot wraps a Discord gateway session and the shared application state it needs
// to record interactions and broadcast live updates back to website clients.
type Bot struct {
	session *discordgo.Session
	stores  *store.Stores
	hub     *ws.Hub
	appID   string
}

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

	b := &Bot{session: session, stores: stores, hub: hub}
	session.AddHandler(b.onReady)
	session.AddHandler(b.onInteraction)
	return b, nil
}

// Start opens the gateway connection. Safe to call on a nil bot.
func (b *Bot) Start() error {
	if b == nil {
		return nil
	}
	return b.session.Open()
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := i.Member.User
	u, err := b.stores.Users.UpsertByDiscord(ctx, user.ID, user.Username, user.AvatarURL(""))
	if err != nil {
		b.ephemeral(s, i, "Something went wrong. Please try again.")
		return
	}

	ann, err := b.stores.Announcements.GetByID(ctx, annID)
	if err != nil {
		b.ephemeral(s, i, "That announcement no longer exists.")
		return
	}

	switch action {
	case models.ResponseComing, models.ResponseReady:
		current := ""
		for _, r := range ann.Responses {
			if r.UserID == u.ID {
				current = r.Status
			}
		}
		if current == action {
			_ = b.stores.Announcements.ClearResponse(ctx, annID, u.ID)
		} else {
			_ = b.stores.Announcements.SetResponse(ctx, annID, u.ID, action)
		}
	case "killed":
		role, _ := b.stores.Groups.Role(ctx, ann.GroupID, u.ID)
		if u.ID != ann.AuthorID && role != models.RoleOwner && role != models.RoleAdmin {
			b.ephemeral(s, i, "Only the person who announced it or a group admin can mark it killed.")
			return
		}
		if err := b.stores.Announcements.MarkKilled(ctx, annID); err != nil {
			b.ephemeral(s, i, "This is already marked killed.")
			return
		}
	case "claim":
		if err := b.stores.Announcements.Claim(ctx, annID, u.ID); err != nil {
			b.ephemeral(s, i, "This can only be claimed after it's marked killed.")
			return
		}
	default:
		return
	}

	ann, err = b.stores.Announcements.GetByID(ctx, annID)
	if err != nil {
		return
	}
	b.hub.Broadcast(ann.GroupID, ws.EventAnnouncementUpdated, ann)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{buildEmbed(ann)},
			Components: buildComponents(ann),
		},
	})
}

// PostAnnouncement posts a new announcement to the group's linked channel (if any)
// and records the resulting message ID. Safe to call on a nil bot.
func (b *Bot) PostAnnouncement(ctx context.Context, ann *models.Announcement) {
	if b == nil || b.session == nil || ann == nil {
		return
	}
	_, channelID, err := b.stores.Groups.DiscordChannel(ctx, ann.GroupID)
	if err != nil || channelID == "" {
		return
	}
	msg, err := b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{buildEmbed(ann)},
		Components: buildComponents(ann),
	})
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
	_, channelID, err := b.stores.Groups.DiscordChannel(ctx, ann.GroupID)
	if err != nil || channelID == "" {
		return
	}
	embeds := []*discordgo.MessageEmbed{buildEmbed(ann)}
	components := buildComponents(ann)
	if _, err := b.session.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    channelID,
		ID:         ann.DiscordMessageID,
		Embeds:     &embeds,
		Components: &components,
	}); err != nil {
		slog.Error("discord: failed to edit announcement message", "error", err)
	}
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

func buildEmbed(a *models.Announcement) *discordgo.MessageEmbed {
	color := 0x4eb87a
	status := "🟢 Open — on your way?"
	if a.Status == models.StatusKilled {
		color = 0x9aa2b1
		status = "💀 Killed"
	}

	var fields []*discordgo.MessageEmbedField
	if a.Location != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "📍 Location", Value: a.Location})
	}
	if a.Note != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Note", Value: a.Note})
	}
	if coming := namesByStatus(a, models.ResponseComing); coming != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "🏃 Coming", Value: coming, Inline: true})
	}
	if ready := namesByStatus(a, models.ResponseReady); ready != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "✅ Ready", Value: ready, Inline: true})
	}
	if a.Status == models.StatusKilled {
		claims := claimNames(a)
		if claims == "" {
			claims = "—"
		}
		fields = append(fields, &discordgo.MessageEmbedField{Name: "🎯 Got the kill", Value: claims})
	}

	return &discordgo.MessageEmbed{
		Title:       "Echo Warden: " + a.CreatureName,
		Description: status,
		Color:       color,
		Fields:      fields,
		Footer:      &discordgo.MessageEmbedFooter{Text: "Announced by " + a.AuthorName},
	}
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

func namesByStatus(a *models.Announcement, status string) string {
	var names []string
	for _, r := range a.Responses {
		if r.Status == status {
			names = append(names, r.CharacterName)
		}
	}
	return strings.Join(names, ", ")
}

func claimNames(a *models.Announcement) string {
	var names []string
	for _, c := range a.Claims {
		names = append(names, c.CharacterName)
	}
	return strings.Join(names, ", ")
}
