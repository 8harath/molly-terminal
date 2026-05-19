package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/ploglabs/molly-terminal/internal/auth/discord"
	"github.com/ploglabs/molly-terminal/internal/commands"
	"github.com/ploglabs/molly-terminal/internal/config"
	"github.com/ploglabs/molly-terminal/internal/db"
	"github.com/ploglabs/molly-terminal/internal/history"
	"github.com/ploglabs/molly-terminal/internal/setup"
	"github.com/ploglabs/molly-terminal/internal/tui"
	"github.com/ploglabs/molly-terminal/internal/webhook"
	"github.com/ploglabs/molly-terminal/internal/wsclient"
)

func main() {
	_ = godotenv.Load()

	forceSetup := false
	for _, arg := range os.Args[1:] {
		if arg == "--setup" || arg == "-s" {
			forceSetup = true
		}
	}

	configPath, err := config.ConfigPathFromArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if err := discord.EnsureUserConfig(context.Background(), cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "discord auth error: %v\n", err)
		os.Exit(1)
	}

	auth := discord.New(cfg)

	if err := setup.RunSetup(context.Background(), cfg, configPath, auth, forceSetup); err != nil {
		fmt.Fprintf(os.Stderr, "setup error: %v\n", err)
	}

	if len(cfg.ConfiguredGuilds) > 1 && !forceSetup {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println()
		fmt.Printf("  Current server: %s\n", cfg.General.GuildName)
		fmt.Println("  Configured servers:")
		for i, g := range cfg.ConfiguredGuilds {
			marker := " "
			if g.ID == cfg.General.GuildID {
				marker = "*"
			}
			fmt.Printf("    %s [%d] %s  (%s)\n", marker, i+1, g.Name, g.Channel)
		}
		fmt.Println()
		fmt.Println("  [Enter] Continue with current server")
		fmt.Println("  [number] Switch to a different server")
		fmt.Print("  Choice: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		if choice != "" && choice != "\n" {
			var idx int
			if _, err := fmt.Sscanf(choice, "%d", &idx); err == nil && idx >= 1 && idx <= len(cfg.ConfiguredGuilds) {
				g := cfg.ConfiguredGuilds[idx-1]
				cfg.General.GuildID = g.ID
				cfg.General.GuildName = g.Name
				cfg.General.Channel = g.Channel
				_ = cfg.Save(configPath)
				fmt.Printf("  Switched to: %s / %s\n", g.Name, g.Channel)
			}
		}
		fmt.Println()
	}

	dbPath, err := config.GuildDBPath(cfg.General.GuildID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database path error: %v\n", err)
		os.Exit(1)
	}
	store, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	client := wsclient.New(cfg.Server.WebsocketURL, cfg.General.Username, cfg.General.Channel)
	sender := webhook.New(cfg.Server.WebhookURL, cfg.Server.RelayURL, cfg.Server.APIKey, cfg.General.Username, cfg.General.DiscordAvatarURL, cfg.General.GuildID)
	fetcher := history.New(cfg.Server.RelayURL)
	if cfg.General.GuildID != "" {
		fetcher = fetcher.WithGuild(cfg.General.GuildID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.ConnectWithRetry(ctx)

	registry := commands.NewRegistry()
	registry.Register(commands.NewHelpCmd(registry))
	registry.Register(commands.NewJoinCmd(client, fetcher, store))
	registry.Register(commands.NewHistoryCmd())
	registry.Register(commands.NewSearchCmd(store))
	registry.Register(commands.NewQuitCmd())
	registry.Register(commands.NewLeaveCmd(store))
	registry.Register(commands.NewStatusCmd())
	registry.Register(commands.NewFileCmd())
	registry.Register(commands.NewOpenCmd())
	registry.Register(commands.NewSnippetCmd())
	registry.Register(commands.NewLogoutCmd(cfg, configPath))
	registry.Register(commands.NewClearMentionsCmd())
	registry.Register(commands.NewEditCmd())
	registry.Register(commands.NewServersCmd(cfg, configPath))
	registry.Register(commands.NewSetupCmd())

	tui.InitImageProtocol(cfg.UI.ImageProtocol)

	model := tui.New(client, sender, store, fetcher, registry,
		cfg.General.Channel, cfg.General.Username, cfg.General.DiscordID,
		cfg.General.DiscordUsername, cfg.General.DiscordGlobalName,
		cfg.ConfiguredGuilds,
		cfg.Auth.Discord.AccessToken, cfg.Server.BotClientID,
		configPath, cfg,
	)
	if cfg.Github.Repo != "" {
		model = model.WithGithub(cfg.Github.Repo, cfg.Github.Token)
	}
	p := tea.NewProgram(model, tea.WithAltScreen())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		_ = client.Close()
		cancel()
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	flagFile := configPath + ".setup-flag"
	if _, err := os.Stat(flagFile); err == nil {
		_ = os.Remove(flagFile)
		runSetupRestart(configPath)
	}
}

func runSetupRestart(configPath string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	if err := discord.EnsureUserConfig(context.Background(), cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "discord auth error: %v\n", err)
		os.Exit(1)
	}
	auth := discord.New(cfg)
	if err := setup.RunSetup(context.Background(), cfg, configPath, auth, true); err != nil {
		fmt.Fprintf(os.Stderr, "setup error: %v\n", err)
	}
}
