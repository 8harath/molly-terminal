package discord

import (
	"net/url"
	"os"
	"testing"

	"github.com/ploglabs/molly-terminal/internal/config"
)

func TestAuthorizationURLIncludesDiscordOAuthFields(t *testing.T) {
	auth := &Authenticator{
		ClientID:    "12345",
		RedirectURL: "http://127.0.0.1:53682/callback",
	}

	raw := auth.AuthorizationURL("state-1")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	q := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "discord.com" {
		t.Fatalf("expected discord authorization host, got %s", parsed.Host)
	}
	if q.Get("client_id") != "12345" {
		t.Fatalf("expected client_id to be set, got %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "http://127.0.0.1:53682/callback" {
		t.Fatalf("expected redirect_uri to be set, got %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") != "identify guilds" {
		t.Fatalf("expected identify guilds scope, got %q", q.Get("scope"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("expected response_type=code, got %q", q.Get("response_type"))
	}
}

func TestApplyDiscordIdentityUsesDiscordUsernameOverGlobalName(t *testing.T) {
	cfg := config.Default()

	applyDiscordIdentity(cfg, User{
		ID:         "1",
		Username:   "discorduser",
		GlobalName: "Display Name",
		Avatar:     "hash",
	})

	if cfg.General.Username != "discorduser" {
		t.Fatalf("expected discord username to become terminal username, got %q", cfg.General.Username)
	}
	if cfg.General.DiscordGlobalName != "Display Name" {
		t.Fatalf("expected global_name to be preserved as alias, got %q", cfg.General.DiscordGlobalName)
	}
	if cfg.General.DiscordAvatarURL == "" {
		t.Fatal("expected avatar URL to be populated")
	}
}

func TestApplyDiscordIdentityFallsBackToGlobalNameWhenUsernameMissing(t *testing.T) {
	cfg := config.Default()

	applyDiscordIdentity(cfg, User{
		ID:         "1",
		GlobalName: "Display Name",
	})

	if cfg.General.Username != "Display Name" {
		t.Fatalf("expected global_name fallback, got %q", cfg.General.Username)
	}
}

func TestSaveConfigMergedPreservesDefaultFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.toml"

	// Create a minimal config file on disk
	initialTOML := `[server]
websocket_url = "ws://custom-websocket"
relay_url = "http://custom-relay"
`
	if err := os.WriteFile(configPath, []byte(initialTOML), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Prepare a config struct
	cfg := config.Default()
	cfg.Auth.Discord.AccessToken = "test-token"

	// Run saveConfigMerged
	if err := saveConfigMerged(cfg, configPath); err != nil {
		t.Fatalf("saveConfigMerged: %v", err)
	}

	// Load the config back from disk using standard config.Load
	os.Args = []string{"molly", "-c", configPath}
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify that the custom values are preserved
	if loaded.Server.WebsocketURL != "ws://custom-websocket" {
		t.Errorf("expected WebsocketURL to be 'ws://custom-websocket', got %q", loaded.Server.WebsocketURL)
	}
	if loaded.Server.RelayURL != "http://custom-relay" {
		t.Errorf("expected RelayURL to be 'http://custom-relay', got %q", loaded.Server.RelayURL)
	}

	// Verify that the default values for fields not in the original file are preserved
	if loaded.Server.WebSetupURL != "https://molly.ploglabs.com" {
		t.Errorf("expected WebSetupURL to retain default 'https://molly.ploglabs.com', got %q", loaded.Server.WebSetupURL)
	}
	if loaded.Auth.Discord.AccessToken != "test-token" {
		t.Errorf("expected AccessToken to be 'test-token', got %q", loaded.Auth.Discord.AccessToken)
	}
}

