#!/usr/bin/env bash
set -euo pipefail

REPO="nqhuy44/mcpx"
INSTALL_DIR="${MCPX_HOME:-$HOME/.mcpx}"
BIN_DIR="$INSTALL_DIR/bin"

# ── helpers ───────────────────────────────────────────────────────────────────

info()    { printf '\033[0;34m[mcpx]\033[0m %s\n' "$*"; }
success() { printf '\033[0;32m[mcpx]\033[0m %s\n' "$*"; }
warn()    { printf '\033[0;33m[mcpx]\033[0m %s\n' "$*"; }
fatal()   { printf '\033[0;31m[mcpx]\033[0m %s\n' "$*" >&2; exit 1; }

require() { command -v "$1" &>/dev/null || fatal "required tool not found: $1"; }

# ── detect platform ───────────────────────────────────────────────────────────

detect_platform() {
  local os arch
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)

  case "$os" in
    linux)  os="linux"  ;;
    darwin) os="darwin" ;;
    mingw*|msys*|cygwin*) os="windows" ;;
    *) fatal "unsupported OS: $os" ;;
  esac

  case "$arch" in
    x86_64)          arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *) fatal "unsupported arch: $arch" ;;
  esac

  echo "${os}_${arch}"
}

# ── download ──────────────────────────────────────────────────────────────────

download() {
  local platform=$1
  require curl

  info "fetching latest release..."
  local version
  version=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | sed 's/.*"tag_name": *"v\([^"]*\)".*/\1/')
  [ -n "$version" ] || fatal "could not determine latest version"

  local ext="tar.gz"
  [[ "$platform" == windows* ]] && ext="zip"
  local archive="mcpx_${version}_${platform}.${ext}"
  local url="https://github.com/${REPO}/releases/download/v${version}/${archive}"

  info "downloading mcpx v${version} (${platform})..."
  local tmp
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT

  curl -sSfL "$url" -o "$tmp/$archive"

  if [ "$ext" = "zip" ]; then
    require unzip
    unzip -q "$tmp/$archive" -d "$tmp/out"
  else
    mkdir -p "$tmp/out"
    tar -xzf "$tmp/$archive" -C "$tmp/out"
  fi

  mkdir -p "$BIN_DIR"
  local bin_ext=""
  [[ "$platform" == windows* ]] && bin_ext=".exe"

  for bin in mcpx-proxy mcpx-exec mcpx-debug; do
    cp "$tmp/out/${bin}${bin_ext}" "$BIN_DIR/"
    chmod +x "$BIN_DIR/${bin}${bin_ext}"
  done
  cp "$tmp/out/gateway.yaml" "$BIN_DIR/"

  if [ -d "$tmp/out/commands/mcpx" ]; then
    mkdir -p "$INSTALL_DIR/commands"
    cp -r "$tmp/out/commands/mcpx" "$INSTALL_DIR/commands/"
  fi

  success "installed to $INSTALL_DIR"
}

# ── PATH setup ───────────────────────────────────────────────────────────────

setup_path() {
  local added=0
  for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
    [ -f "$rc" ] || continue
    grep -qF "$BIN_DIR" "$rc" && continue
    printf '\n# mcpx\nexport PATH="$PATH:%s"\n' "$BIN_DIR" >> "$rc"
    info "added $BIN_DIR to PATH in $rc"
    added=1
  done
  if [ "$added" -eq 1 ]; then
    warn "restart your shell or run: export PATH=\"\$PATH:$BIN_DIR\""
  fi
}

# ── MCP client config injection ───────────────────────────────────────────────

PROXY_BIN="$BIN_DIR/mcpx-proxy"

mcp_entry() {
  cat <<EOF
{
  "mcpServers": {
    "mcpx": {
      "command": "$PROXY_BIN"
    }
  }
}
EOF
}

# merge { "mcpServers": { "mcpx": ... } } into an existing JSON config file.
# creates the file if it doesn't exist. requires jq.
inject_json() {
  local file=$1
  mkdir -p "$(dirname "$file")"
  if [ -f "$file" ]; then
    local tmp
    tmp=$(mktemp "$(dirname "$file")/.mcpx-XXXXXX")
    jq --argjson new "$(mcp_entry)" \
      '.mcpServers = ((.mcpServers // {}) + $new.mcpServers)' \
      "$file" > "$tmp" && mv "$tmp" "$file"
  else
    mcp_entry > "$file"
  fi
}

configure_claude_code() {
  if command -v claude &>/dev/null; then
    info "configuring Claude Code..."
    claude mcp add mcpx "$PROXY_BIN" --scope user 2>/dev/null \
      || claude mcp add mcpx "$PROXY_BIN" 2>/dev/null \
      || { warn "claude mcp add failed — add manually (see below)"; return 1; }
    success "Claude Code: registered mcpx"
    install_claude_commands
  fi
}

install_claude_commands() {
  local src="$INSTALL_DIR/commands/mcpx"
  local dest="$HOME/.claude/commands/mcpx"
  [ -d "$src" ] || return 0
  mkdir -p "$HOME/.claude/commands"
  cp -r "$src" "$HOME/.claude/commands/"
  success "Claude Code: installed slash commands → $dest"
  info "  Use /mcpx:exec, /mcpx:debug"
}

configure_antigravity() {
  local cfg="$HOME/.gemini/antigravity/mcp_config.json"
  if [ -d "$HOME/.gemini/antigravity" ] || [ -f "$cfg" ]; then
    info "configuring Google Antigravity..."
    if command -v jq &>/dev/null; then
      inject_json "$cfg"
      success "Google Antigravity: updated $cfg"
    else
      warn "Google Antigravity detected but jq not found — add manually (see below)"
    fi
  fi
}

configure_cursor() {
  local cfg="$HOME/.cursor/mcp.json"
  if [ -d "$HOME/.cursor" ] || [ -f "$cfg" ]; then
    info "configuring Cursor..."
    if command -v jq &>/dev/null; then
      inject_json "$cfg"
      success "Cursor: updated $cfg"
    else
      warn "Cursor detected but jq not found — add manually (see below)"
    fi
  fi
}

configure_windsurf() {
  local cfg="$HOME/.codeium/windsurf/mcp_config.json"
  if [ -d "$HOME/.codeium/windsurf" ] || [ -f "$cfg" ]; then
    info "configuring Windsurf..."
    if command -v jq &>/dev/null; then
      inject_json "$cfg"
      success "Windsurf: updated $cfg"
    else
      warn "Windsurf detected but jq not found — add manually (see below)"
    fi
  fi
}

configure_zed() {
  local cfg="$HOME/.config/zed/settings.json"
  if [ -d "$HOME/.config/zed" ] || [ -f "$cfg" ]; then
    info "configuring Zed..."
    if command -v jq &>/dev/null; then
      inject_json "$cfg"
      success "Zed: updated $cfg"
    else
      warn "Zed detected but jq not found — add manually (see below)"
    fi
  fi
}

configure_vscode() {
  local cfg
  case "$(uname -s)" in
    Darwin) cfg="$HOME/Library/Application Support/Code/User/settings.json" ;;
    Linux)  cfg="$HOME/.config/Code/User/settings.json" ;;
    *)      return ;;
  esac
  if [ -f "$cfg" ]; then
    info "configuring VS Code..."
    if command -v jq &>/dev/null; then
      inject_json "$cfg"
      success "VS Code: updated $cfg"
    else
      warn "VS Code detected but jq not found — add manually (see below)"
    fi
  fi
}

print_manual_config() {
  cat <<EOF

──────────────────────────────────────────────────
  Manual setup — add to your MCP client config:
──────────────────────────────────────────────────
$(mcp_entry)

  Config file locations by client:
    Claude Code        : run 'claude mcp add mcpx $PROXY_BIN'
    Google Antigravity : ~/.gemini/antigravity/mcp_config.json
    Cursor             : ~/.cursor/mcp.json
    Windsurf           : ~/.codeium/windsurf/mcp_config.json
    Zed                : ~/.config/zed/settings.json
    VS Code            : ~/Library/Application Support/Code/User/settings.json (macOS)
                         ~/.config/Code/User/settings.json (Linux)

  Claude Code slash commands (optional):
    cp -r ~/.mcpx/commands/mcpx ~/.claude/commands/
    Enables: /mcpx:exec, /mcpx:debug
    (All clients also get /mcpx_exec, /mcpx_debug via MCP Prompts — no copy needed)

  Admin dashboard (when proxy is running):
    http://localhost:9090/ui
──────────────────────────────────────────────────
EOF
}

# ── main ──────────────────────────────────────────────────────────────────────

main() {
  local platform
  platform=$(detect_platform)

  download "$platform"
  setup_path
  configure_claude_code
  configure_antigravity
  configure_cursor
  configure_windsurf
  configure_zed
  configure_vscode
  print_manual_config

  success "done! restart your MCP client to pick up the new server."
}

main "$@"
