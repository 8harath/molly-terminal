package setup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ploglabs/molly-terminal/internal/auth/discord"
	"github.com/ploglabs/molly-terminal/internal/config"
)

const botPermissions = 8

type Step int

const (
	StepChooseMethod Step = iota
	StepPickGuild
	StepConfirmGuild
	StepInviteBot
	StepWaitForBot
	StepDone
)

type WizardState struct {
	Step          Step
	Method        string
	Guilds        []discord.Guild
	SelectedGuild discord.Guild
	PrevGuild     discord.Guild
}

func RunSetup(ctx context.Context, cfg *config.Config, configPath string, auth *discord.Authenticator, force bool) error {
	if !force && cfg.General.GuildID != "" {
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	state := &WizardState{Step: StepChooseMethod}

	printBanner()
	fmt.Println()

	for state.Step != StepDone {
		switch state.Step {
		case StepChooseMethod:
			handleChooseMethod(reader, cfg, state)
		case StepPickGuild:
			if err := handlePickGuild(ctx, reader, auth, cfg, state); err != nil {
				return err
			}
		case StepConfirmGuild:
			handleConfirmGuild(reader, state)
		case StepInviteBot:
			handleInviteBot(reader, cfg, state)
		case StepWaitForBot:
			if err := handleWaitForBot(ctx, reader, cfg, state); err != nil {
				return err
			}
		}
	}

	return saveConfig(cfg, configPath, state)
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════╗")
	fmt.Println("  ║        Molly Setup Wizard        ║")
	fmt.Println("  ╚══════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  Welcome! Let's connect Molly to your Discord server.")
}

func handleChooseMethod(reader *bufio.Reader, cfg *config.Config, state *WizardState) {
	state.Method = ""
	fmt.Println("  How would you like to configure Molly?")
	fmt.Println()
	fmt.Println("    [1] Configure via terminal (recommended)")
	fmt.Println("    [2] Configure via web browser")
	fmt.Println()

	for state.Method == "" {
		fmt.Print("  Enter choice (1 or 2): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			state.Method = "terminal"
			state.Step = StepPickGuild
		case "2":
			state.Method = "web"
			openBrowser(fmt.Sprintf("http://localhost:3000/setup?token=%s", cfg.Auth.Discord.AccessToken))
			fmt.Println()
			fmt.Println("  A browser window should have opened to the Molly web setup.")
			fmt.Println("  Complete the setup there, then press Enter to continue.")
			fmt.Println()
			fmt.Print("  Press Enter once you've completed web setup...")
			reader.ReadString('\n')
			state.Method = "terminal"
			state.Step = StepPickGuild
		default:
			fmt.Println()
			fmt.Println("  Invalid choice. Please enter 1 or 2.")
			fmt.Println()
		}
	}
}

func handlePickGuild(ctx context.Context, reader *bufio.Reader, auth *discord.Authenticator, cfg *config.Config, state *WizardState) error {
	fmt.Println()
	fmt.Println("  Fetching servers you manage...")

	allGuilds, err := auth.FetchUserGuilds(ctx, cfg.Auth.Discord.AccessToken)
	if err != nil {
		fmt.Printf("  Warning: Could not fetch servers: %v\n", err)
		fmt.Println("  You can skip this step and configure later.")
		fmt.Println()
		fmt.Print("  Press Enter to skip server setup...")
		reader.ReadString('\n')
		state.Step = StepDone
		return nil
	}

	state.Guilds = discord.FilterAdminGuilds(allGuilds)
	if len(state.Guilds) == 0 {
		fmt.Println()
		fmt.Println("  No servers found where you have admin permissions.")
		fmt.Println("  You need Admin or Manage Server permissions in a server.")
		fmt.Println()
		fmt.Print("  Press Enter to continue without server setup...")
		reader.ReadString('\n')
		state.Step = StepDone
		return nil
	}

	fmt.Println()
	fmt.Println("  Servers you manage:")
	fmt.Println()
	for i, g := range state.Guilds {
		fmt.Printf("    [%d] %s\n", i+1, g.Name)
	}
	fmt.Println()

	for {
		fmt.Print("  Choose a server (number): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var idx int
		_, err := fmt.Sscanf(choice, "%d", &idx)
		if err != nil || idx < 1 || idx > len(state.Guilds) {
			fmt.Println("  Invalid selection. Please choose a number from the list.")
			fmt.Println()
			continue
		}

		state.PrevGuild = state.SelectedGuild
		state.SelectedGuild = state.Guilds[idx-1]
		state.Step = StepConfirmGuild
		return nil
	}
}

func handleConfirmGuild(reader *bufio.Reader, state *WizardState) {
	fmt.Println()
	fmt.Printf("  You selected: %s\n", state.SelectedGuild.Name)
	fmt.Println()
	fmt.Println("  [y] Confirm and continue")
	fmt.Println("  [n] Go back and choose a different server")
	fmt.Print("  Enter choice (y/n): ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(strings.ToLower(choice))

	switch choice {
	case "y", "yes", "":
		state.Step = StepInviteBot
	case "n", "no":
		if state.PrevGuild.ID != "" {
			state.SelectedGuild = state.PrevGuild
		}
		state.Step = StepPickGuild
	default:
		fmt.Println("  Invalid choice. Please enter y or n.")
	}
}

func handleInviteBot(reader *bufio.Reader, cfg *config.Config, state *WizardState) {
	botClientID := cfg.Server.BotClientID
	if botClientID == "" {
		botClientID = cfg.Auth.Discord.ClientID
	}

	inviteURL := fmt.Sprintf(
		"https://discord.com/oauth2/authorize?client_id=%s&permissions=%d&scope=bot&guild_id=%s",
		botClientID, botPermissions, state.SelectedGuild.ID,
	)

	fmt.Println()
	fmt.Println("  To connect Molly to your server, you need to invite the bot.")
	fmt.Println()
	fmt.Println("  Bot Invite Link:")
	fmt.Println("  " + inviteURL)
	fmt.Println()
	fmt.Println("  A browser window has been opened with the invite link.")
	openBrowser(inviteURL)
	fmt.Println("  After inviting the bot, come back here and press Enter.")
	fmt.Println()
	fmt.Println("  [Enter] I've invited the bot — verify & continue")
	fmt.Println("  [b]     Go back to server selection")
	fmt.Print("  Enter choice: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(strings.ToLower(choice))

	switch choice {
	case "b":
		state.Step = StepPickGuild
	default:
		state.Step = StepWaitForBot
	}
}

func handleWaitForBot(ctx context.Context, reader *bufio.Reader, cfg *config.Config, state *WizardState) error {
	fmt.Println()
	fmt.Println("  Waiting for the bot to join your server...")

	maxAttempts := 30
	pollInterval := 2 * time.Second
	checkURL := fmt.Sprintf("%s/api/bot/check/%s", cfg.Server.RelayURL, state.SelectedGuild.ID)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fmt.Printf("\r  Checking... (attempt %d/%d)", attempt, maxAttempts)

		inGuild, err := pollBotPresence(ctx, checkURL)
		if err == nil && inGuild {
			fmt.Println()
			fmt.Println()
			fmt.Println("  Bot has joined your server successfully!")
			fmt.Println()
			state.Step = StepDone
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("  Bot presence not detected. This could mean:")
	fmt.Println("  - The invite link was not used")
	fmt.Println("  - The relay server is not running")
	fmt.Println("  - The bot hasn't synced channels yet")
	fmt.Println()
	fmt.Println("  [r] Retry")
	fmt.Println("  [s] Skip — save configuration and continue")
	fmt.Println("  [b] Go back")
	fmt.Print("  Enter choice: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(strings.ToLower(choice))

	switch choice {
	case "r":
		return handleWaitForBot(ctx, reader, cfg, state)
	case "b":
		state.Step = StepInviteBot
		return nil
	default:
		fmt.Println()
		fmt.Println("  Saving configuration. You can verify later.")
		fmt.Println()
		state.Step = StepDone
	}
	return nil
}

func pollBotPresence(ctx context.Context, checkURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result struct {
		BotInGuild bool `json:"bot_in_guild"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.BotInGuild, nil
}

func saveConfig(cfg *config.Config, configPath string, state *WizardState) error {
	if state.SelectedGuild.ID == "" {
		return nil
	}

	if cfg.General.GuildID == "" {
		cfg.General.GuildID = state.SelectedGuild.ID
		cfg.General.GuildName = state.SelectedGuild.Name
	}

	entry := config.GuildEntry{
		ID:         state.SelectedGuild.ID,
		Name:       state.SelectedGuild.Name,
		Channel:    cfg.General.Channel,
		Configured: true,
	}

	found := false
	for i, g := range cfg.ConfiguredGuilds {
		if g.ID == entry.ID {
			cfg.ConfiguredGuilds[i] = entry
			found = true
			break
		}
	}
	if !found {
		cfg.ConfiguredGuilds = append(cfg.ConfiguredGuilds, entry)
	}

	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}

	fmt.Println()
	fmt.Println("  Configuration saved successfully!")
	fmt.Printf("  Connected to: %s\n", state.SelectedGuild.Name)
	return nil
}

func openBrowser(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	_ = cmd.Start()
}
