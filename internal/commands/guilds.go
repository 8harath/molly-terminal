package commands

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ploglabs/molly-terminal/internal/config"
)

type GuildsCmd struct {
	cfg        *config.Config
	configPath string
	relayURL   string
	apiKey     string
}

func NewGuildsCmd(cfg *config.Config, configPath, relayURL, apiKey string) *GuildsCmd {
	return &GuildsCmd{
		cfg:        cfg,
		configPath: configPath,
		relayURL:   relayURL,
		apiKey:     apiKey,
	}
}

func (c *GuildsCmd) Name() string { return "guilds" }

func (c *GuildsCmd) Description() string {
	return "Discover servers the bot is in: /guilds"
}

func (c *GuildsCmd) Execute(args []string) (tea.Cmd, error) {
	if c.relayURL == "" {
		return nil, fmt.Errorf("server.relay_url is not configured")
	}

	return func() tea.Msg {
		return GuildsDiscoverMsg{}
	}, nil
}

type GuildsDiscoverMsg struct{}

