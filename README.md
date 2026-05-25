<p align="center">
  <img src="logo.png" alt="molly" width="120">
</p>

# molly

Terminal-native realtime chat client for Discord.

## Install

**macOS**
```bash
brew install ploglabs/tap/molly
```

**Linux (deb/rpm/arch)**
```bash
# Debian/Ubuntu
curl -LO https://github.com/ploglabs/molly-terminal/releases/latest/download/molly_$(curl -s https://api.github.com/repos/ploglabs/molly-terminal/releases/latest | grep tag_name | cut -d'"' -f4 | sed 's/^v//')_linux_amd64.deb
sudo dpkg -i molly_*_linux_amd64.deb

# Arch
yay -S molly-bin

# Fedora/RHEL
sudo rpm -i molly_*_linux_amd64.rpm
```

**Windows**
```powershell
scoop bucket add ploglabs https://github.com/ploglabs/scoop-bucket
scoop install molly
```

**Go**
```bash
go install github.com/ploglabs/molly-terminal/cmd/molly@latest
```

## Quick Start

```bash
molly
```

First run opens your browser for Discord auth, then you're in the chat. No config needed.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message / run command |
| `Ctrl+C` / `Ctrl+Q` | Quit |
| `Ctrl+B` | Toggle channels sidebar |
| `Ctrl+Y` | Toggle users panel |
| `Ctrl+L` | Jump to bottom |
| `Ctrl+P` / `Ctrl+N` | Previous / next channel |
| `Ctrl+W` | Delete word backwards |
| `Ctrl+K` | Delete to end of line |
| `Ctrl+U` | Clear input |
| `Tab` | Complete `@user` or `#channel` |
| `↑` / `↓` | Scroll history |
| `PgUp` / `PgDn` | Scroll half page |

## Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/join #channel` | Switch to a channel |
| `/history` | Load older messages |
| `/search <query>` | Search local history |
| `/clear` | Clear chat view |
| `/quit` | Exit |

## Config (optional)

Config file at `~/.config/molly/config.toml`. Everything is pre-configured — only edit if needed.

```toml
[general]
username = "anon"          # overridden by Discord auth

[server]
websocket_url = "wss://molly.ploglabs.com:8443/ws"
webhook_url = "https://discord.com/api/webhooks/..."
relay_url = "https://molly.ploglabs.com:8443"

[auth]
enabled = true             # Discord OAuth2 login on first run
provider = "discord"

[ui]
theme = "default"          # default, dracula, solarized
history_limit = 100
image_protocol = "auto"    # auto, iterm2, kitty, none
```

Override any field via env vars (`MOLLY_USERNAME`, `MOLLY_THEME`, etc.).

## Run locally

```bash
git clone https://github.com/ploglabs/molly-terminal.git
cd molly-terminal
make build          # → bin/molly
./bin/molly
```

Requires [Go](https://go.dev) 1.21+.

## License

[Apache License 2.0](LICENSE)
