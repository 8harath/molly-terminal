package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ploglabs/molly-terminal/internal/config"
	"github.com/ploglabs/molly-terminal/internal/model"
)

type setupStep int

const (
	setupIdle setupStep = iota
	setupFetching
	setupPicking
	setupConfirming
	setupDone
)

type setupGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Owner       bool   `json:"owner"`
	Permissions string `json:"permissions"`
}

type setupGuildsMsg struct {
	Guilds []setupGuild
	Err    error
}

func (m *Model) openSetupWizard() (tea.Model, tea.Cmd) {
	if m.setupStep != setupIdle {
		return m, nil
	}
	m.setupStep = setupFetching
	m.setupGuilds = nil
	m.setupSelectedIdx = 0
	m.setupErr = ""

	if m.discordAccessToken == "" {
		m.setupErr = "Discord access token not available. Restart Molly to re-authenticate."
		m.setupStep = setupDone
		return m, nil
	}

	return m, func() tea.Msg {
		guilds, err := fetchDiscordGuilds(m.discordAccessToken)
		if err != nil {
			return setupGuildsMsg{Err: err}
		}
		adminGuilds := filterAdminGuilds(guilds)
		return setupGuildsMsg{Guilds: adminGuilds}
	}
}

func fetchDiscordGuilds(accessToken string) ([]setupGuild, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/v10/users/@me/guilds", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting guilds: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("guilds request failed: %s: %s", resp.Status, string(body))
	}

	var guilds []setupGuild
	if err := json.NewDecoder(resp.Body).Decode(&guilds); err != nil {
		return nil, fmt.Errorf("decoding guilds: %w", err)
	}
	return guilds, nil
}

const (
	setupPermAdmin      = 0x8
	setupPermManageGuild = 0x20
)

func filterAdminGuilds(guilds []setupGuild) []setupGuild {
	var filtered []setupGuild
	for _, g := range guilds {
		if g.Owner {
			filtered = append(filtered, g)
			continue
		}
		var perms int64
		fmt.Sscanf(g.Permissions, "%d", &perms)
		if (perms & setupPermAdmin) != 0 || (perms & setupPermManageGuild) != 0 {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

func (m *Model) handleSetupGuilds(msg setupGuildsMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.setupErr = fmt.Sprintf("Failed to fetch servers: %v", msg.Err)
		m.setupStep = setupDone
		return m, nil
	}
	if len(msg.Guilds) == 0 {
		m.setupErr = "No servers found where you have admin permissions."
		m.setupStep = setupDone
		return m, nil
	}
	m.setupGuilds = msg.Guilds
	m.setupStep = setupPicking
	m.setupSelectedIdx = 0
	return m, nil
}

func (m *Model) handleSetupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.setupStep = setupIdle
		return m, nil

	case "up":
		if m.setupStep == setupPicking && m.setupSelectedIdx > 0 {
			m.setupSelectedIdx--
		}
		return m, nil

	case "down":
		if m.setupStep == setupPicking && m.setupSelectedIdx < len(m.setupGuilds)-1 {
			m.setupSelectedIdx++
		}
		return m, nil

	case "enter":
		switch m.setupStep {
		case setupPicking:
			if m.setupSelectedIdx >= 0 && m.setupSelectedIdx < len(m.setupGuilds) {
				m.setupStep = setupConfirming
			}
		case setupConfirming:
			return m.confirmSetupGuild()
		case setupDone:
			m.setupStep = setupIdle
		}
		return m, nil

	case "y", "Y":
		if m.setupStep == setupConfirming {
			return m.confirmSetupGuild()
		}
	case "n", "N":
		if m.setupStep == setupConfirming {
			m.setupStep = setupPicking
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) confirmSetupGuild() (tea.Model, tea.Cmd) {
	if m.setupSelectedIdx < 0 || m.setupSelectedIdx >= len(m.setupGuilds) {
		return m, nil
	}
	g := m.setupGuilds[m.setupSelectedIdx]

	entry := config.GuildEntry{
		ID:         g.ID,
		Name:       g.Name,
		Channel:    m.channel,
		Configured: true,
	}

	found := false
	for i, eg := range m.configuredGuilds {
		if eg.ID == entry.ID {
			m.configuredGuilds[i] = entry
			found = true
			break
		}
	}
	if !found {
		m.configuredGuilds = append(m.configuredGuilds, entry)
	}

	if m.setupCfg != nil {
		m.setupCfg.ConfiguredGuilds = m.configuredGuilds
		if m.setupCfg.General.GuildID == "" {
			m.setupCfg.General.GuildID = entry.ID
			m.setupCfg.General.GuildName = entry.Name
		}
		if m.setupConfigPath != "" {
			_ = m.setupCfg.Save(m.setupConfigPath)
		}
	}

	botClientID := m.discordClientID
	inviteURL := fmt.Sprintf(
		"https://discord.com/oauth2/authorize?client_id=%s&permissions=8&scope=bot&guild_id=%s",
		botClientID, g.ID,
	)

	sysMsg := model.Message{
		ID:        fmt.Sprintf("sys-setup-%d", time.Now().UnixNano()),
		Username:  "system",
		Content:   fmt.Sprintf("Server configured: %s\n\nBot invite link:\n%s\n\nInvite the bot, then restart Molly to connect.", g.Name, inviteURL),
		Timestamp: time.Now(),
	}
	m.msgs = append(m.msgs, sysMsg)
	m.scrollOffset = 0

	m.setupStep = setupIdle
	return m, nil
}

func (m Model) renderSetupWizard(width, height int) string {
	if m.setupStep == setupIdle {
		return ""
	}

	title := panelTitleStyle().Render(" Add Server ")
	innerW := borderedStyleWidth(width)
	innerH := borderedStyleHeight(height)

	var content string
	switch m.setupStep {
	case setupFetching:
		content = lipgloss.NewStyle().Foreground(themeDim).Padding(1).Render("Fetching servers you manage...")
	case setupDone:
		if m.setupErr != "" {
			content = lipgloss.NewStyle().Foreground(themeErr).Padding(1).Render(m.setupErr)
		}
	default:
		content = m.renderSetupGuildList(innerW)
	}

	hint := lipgloss.NewStyle().Foreground(themeDim).Render(m.setupHint())
	boxContent := clipLines(hint+"\n"+content, innerH)
	box := renderBorderedBox(panelStyle(), width, height, boxContent)

	if m.setupStep == setupConfirming && m.setupSelectedIdx >= 0 && m.setupSelectedIdx < len(m.setupGuilds) {
		g := m.setupGuilds[m.setupSelectedIdx]
		confirmW := width - 10
		confirmH := 6
		if confirmW < 30 {
			confirmW = 30
		}
		confirmText := fmt.Sprintf("Add server?\n\n  %s\n\n[y] yes  [n] no", g.Name)
		confirmBox := renderBorderedBox(
			lipgloss.NewStyle().BorderForeground(themeAccent).BorderStyle(lipgloss.NormalBorder()).Padding(1),
			confirmW, confirmH,
			lipgloss.NewStyle().Foreground(themeFg).Render(confirmText),
		)
		centered := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, confirmBox)
		return title + "\n" + centered
	}

	return title + "\n" + box
}

func (m Model) renderSetupGuildList(width int) string {
	if m.setupStep != setupPicking {
		return ""
	}
	if len(m.setupGuilds) == 0 {
		return lipgloss.NewStyle().Foreground(themeDim).Render("No servers found.")
	}

	names := make([]string, len(m.setupGuilds))
	for i, g := range m.setupGuilds {
		names[i] = g.Name
	}
	sort.Strings(names)

	var lines []string
	nameToIdx := make(map[string]int)
	for i, g := range m.setupGuilds {
		nameToIdx[g.Name] = i
	}
	for _, name := range names {
		idx := nameToIdx[name]
		g := m.setupGuilds[idx]
		marker := "  "
		style := lipgloss.NewStyle().Foreground(themeFg)
		if idx == m.setupSelectedIdx {
			marker = "▶ "
			style = lipgloss.NewStyle().Foreground(themeAccent).Background(themeSelectedBg)
		}
		ownerTag := ""
		if g.Owner {
			ownerTag = lipgloss.NewStyle().Foreground(themeWarn).Render(" [owner]")
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%s%s", marker, g.Name, ownerTag)))
	}

	return strings.Join(lines, "\n")
}

func (m Model) setupHint() string {
	switch m.setupStep {
	case setupFetching:
		return "Fetching your Discord servers..."
	case setupPicking:
		return "↑↓ select  enter confirm  esc close"
	case setupConfirming:
		return "y confirm  n go back"
	case setupDone:
		return "enter to close  esc close"
	}
	return ""
}

func (m Model) isSetupVisible() bool {
	return m.setupStep != setupIdle
}
