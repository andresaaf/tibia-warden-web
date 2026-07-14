package auth
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/baz/tibia-warden-web/backend/internal/config"
	"golang.org/x/oauth2"
)

const discordAuthURL = "https://discord.com/api/oauth2/authorize"
const discordTokenURL = "https://discord.com/api/oauth2/token"
const discordUserURL = "https://discord.com/api/users/@me"

// DiscordProvider wraps the OAuth2 config for Discord login.
type DiscordProvider struct {
	oauth *oauth2.Config
}

// DiscordUser is the subset of the Discord user object we consume.
type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// NewDiscordProvider builds a Discord OAuth2 provider from configuration.
func NewDiscordProvider(cfg *config.Config) *DiscordProvider {
	return &DiscordProvider{
		oauth: &oauth2.Config{
			ClientID:     cfg.DiscordClientID,
			ClientSecret: cfg.DiscordClientSecret,
			RedirectURL:  cfg.DiscordRedirectURL,
			Scopes:       []string{"identify"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  discordAuthURL,
				TokenURL: discordTokenURL,
			},
		},
	}
}

// AuthCodeURL returns the Discord authorization URL for the given state.
func (p *DiscordProvider) AuthCodeURL(state string) string {
	return p.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange trades an authorization code for the authenticated Discord user.
func (p *DiscordProvider) Exchange(ctx context.Context, code string) (*DiscordUser, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	token, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	client := p.oauth.Client(ctx, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discordUserURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch discord user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord user endpoint returned status %d", resp.StatusCode)
	}

	var u DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode discord user: %w", err)
	}
	return &u, nil
}

// AvatarURL builds a CDN URL for a Discord user's avatar, or empty if none.
func (u *DiscordUser) AvatarURL() string {
	if u.Avatar == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", u.ID, u.Avatar)
}
