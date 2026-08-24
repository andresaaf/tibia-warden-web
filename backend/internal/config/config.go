package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	ListenAddr  string
	DatabaseURL string

	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURL  string

	// DiscordBotToken enables the Discord announcement bot when set. Optional.
	DiscordBotToken string

	// CreaturesAPIURL, when set, is fetched at startup to sync the creature list.
	// Set to empty to disable and rely solely on manual seeding.
	CreaturesAPIURL string

	// SessionSecret is used to sign session cookies. Must be stable across restarts.
	SessionSecret string

	// PublicBaseURL is the externally reachable base URL of the app (used for redirects).
	PublicBaseURL string

	// AllowedOrigins is the list of origins permitted for CORS and WebSocket handshakes.
	AllowedOrigins []string

	// StaticDir, when set, causes the server to serve the built SPA from this directory.
	// When empty, the embedded SPA (if built into the binary) is used.
	StaticDir string

	// Secure controls whether session cookies require HTTPS. Disable only for local dev.
	Secure bool

	// AutoKillMinutes is how long after a reveal an announcement is automatically
	// marked killed when no one did so manually. An Echo Warden only lives ~10
	// minutes after spawning, so the default (15) leaves some slack. 0 disables it.
	AutoKillMinutes int
}

// Load reads configuration from the environment and validates required values.
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:          getEnv("LISTEN_ADDR", ":8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:  os.Getenv("DISCORD_REDIRECT_URL"),
		DiscordBotToken:     os.Getenv("DISCORD_BOT_TOKEN"),
		CreaturesAPIURL:     getEnv("CREATURES_API_URL", "https://tibiawiki.dev/api/creatures?expand=true"),
		SessionSecret:       os.Getenv("SESSION_SECRET"),
		PublicBaseURL:       getEnv("PUBLIC_BASE_URL", "http://localhost:5173"),
		StaticDir:           os.Getenv("STATIC_DIR"),
		Secure:              getEnv("COOKIE_SECURE", "false") == "true",
		AutoKillMinutes:     getEnvInt("AUTO_KILL_MINUTES", 15),
	}

	origins := getEnv("ALLOWED_ORIGINS", "http://localhost:5173")
	for _, o := range strings.Split(origins, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
		}
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.DiscordClientID == "" {
		missing = append(missing, "DISCORD_CLIENT_ID")
	}
	if cfg.DiscordClientSecret == "" {
		missing = append(missing, "DISCORD_CLIENT_SECRET")
	}
	if cfg.DiscordRedirectURL == "" {
		missing = append(missing, "DISCORD_REDIRECT_URL")
	}
	if cfg.SessionSecret == "" {
		missing = append(missing, "SESSION_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt reads an integer env var, falling back when unset or unparsable.
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
