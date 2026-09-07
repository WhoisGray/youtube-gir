#!/usr/bin/env bash

set -euo pipefail

REPOSITORY="WhoisGray/youtube-gir"
RAW_ROOT="https://raw.githubusercontent.com/$REPOSITORY"
INSTALL_DIR="${YOUTUBE_GIR_BIN_DIR:-$HOME/.local/bin}"
INSTALL_DEPS="true"
ASSUME_YES="false"
FORCE_INSTALL="false"
# Bash 3.2 with nounset treats an entirely empty array as unbound.
TEMPORARY_PATHS=("")

for argument in "$@"; do
  case "$argument" in
    --no-deps) INSTALL_DEPS="false" ;;
    --yes|-y) ASSUME_YES="true" ;;
    --force) FORCE_INSTALL="true" ;;
    --help|-h)
      printf 'Usage: install.sh [--yes] [--no-deps] [--force]\n'
      exit 0
      ;;
    *) printf 'Unknown option: %s\n' "$argument" >&2; exit 2 ;;
  esac
done

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'macos' ;;
    Linux) printf 'linux' ;;
    MINGW*|MSYS*|CYGWIN*) printf 'windows' ;;
    *) printf 'unknown' ;;
  esac
}

OS_NAME="${YOUTUBE_GIR_OS:-$(detect_os)}"

log() { printf 'ℹ️  %s\n' "$*"; }
done_message() { printf '✅ %s\n' "$*"; }
fail() { printf '❌ %s\n' "$*" >&2; exit 1; }
warn() { printf '⚠️  %s\n' "$*" >&2; }

cleanup() {
  local path
  for path in "${TEMPORARY_PATHS[@]}"; do
    [[ -n "$path" && "$path" == "${TMPDIR:-/tmp}"/youtube-gir-* ]] && rm -rf "$path"
  done
  return 0
}
trap cleanup EXIT

download_file() {
  local url="$1" destination="$2"
  curl -fL --retry 3 --connect-timeout 15 -o "$destination" "$url"
}

github_latest_tag() {
  curl -fsSL --connect-timeout 10 --max-time 25 \
    -H 'Accept: application/vnd.github+json' \
    "https://api.github.com/repos/$1/releases/latest" 2>/dev/null \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1
}

version_without_v() { printf '%s' "${1#v}"; }

bootstrap_brew() {
  command -v brew >/dev/null 2>&1 && return
  case "$OS_NAME" in macos|linux) ;; *) fail "Homebrew is supported only on macOS and Linux." ;; esac
  if [[ "$ASSUME_YES" != "true" && -t 0 ]]; then
    printf 'Homebrew is not installed. Run the official brew.sh installer? [y/N] '
    read -r answer
    [[ "$answer" =~ ^[Yy]$ ]] || fail "Installation cancelled."
  fi
  local temporary_dir installer
  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-brew.XXXXXX")"
  TEMPORARY_PATHS+=("$temporary_dir")
  installer="$temporary_dir/install.sh"
  log "Downloading the official Homebrew installer…"
  download_file 'https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh' "$installer"
  if [[ "$ASSUME_YES" == "true" ]]; then NONINTERACTIVE=1 /bin/bash "$installer"; else /bin/bash "$installer"; fi
  if [[ -x /opt/homebrew/bin/brew ]]; then eval "$(/opt/homebrew/bin/brew shellenv)"; fi
  if [[ -x /home/linuxbrew/.linuxbrew/bin/brew ]]; then eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"; fi
  command -v brew >/dev/null 2>&1 || fail "Homebrew was installed but is not in PATH. Open a new terminal and run this installer again."
}

update_brew_formula() {
  local formula="$1" label="$2"
  if ! brew list "$formula" >/dev/null 2>&1; then
    log "Installing $label with Homebrew…"
    brew install "$formula"
  elif brew outdated --quiet "$formula" | grep -q .; then
    log "A newer $label version is available; upgrading…"
    brew upgrade "$formula"
  else
    done_message "$label is up to date."
  fi
}

install_or_update_deno() {
  if brew list deno >/dev/null 2>&1; then
    update_brew_formula deno Deno
    return
  fi
  if command -v deno >/dev/null 2>&1; then
    log "Checking Deno with its official updater…"
    deno upgrade
    return
  fi
  local temporary_dir installer
  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-deno.XXXXXX")"
  TEMPORARY_PATHS+=("$temporary_dir")
  installer="$temporary_dir/install.sh"
  log "Downloading Deno from deno.land…"
  download_file 'https://deno.land/install.sh' "$installer"
  sh "$installer"
  export PATH="$HOME/.deno/bin:$PATH"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}';
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

install_binary() {
  local source_file="$1" destination="$2"
  if command -v install >/dev/null 2>&1; then
    install -m 0755 "$source_file" "$destination"
  else
    cp -f "$source_file" "$destination"
    chmod 0755 "$destination"
  fi
}

install_or_update_ytdlp() {
  local machine asset yt_name temporary_dir binary sums expected actual latest installed
  machine="$(uname -m)"
  case "$OS_NAME:$machine" in
    macos:*) asset="yt-dlp_macos" ;;
    linux:x86_64|linux:amd64) asset="yt-dlp_linux" ;;
    linux:aarch64|linux:arm64) asset="yt-dlp_linux_aarch64" ;;
    windows:*) asset="yt-dlp.exe" ;;
    *) fail "No official yt-dlp binary is configured for $OS_NAME/$machine." ;;
  esac
  yt_name="yt-dlp"
  [[ "$OS_NAME" == "windows" ]] && yt_name="yt-dlp.exe"

  latest="$(github_latest_tag 'yt-dlp/yt-dlp' || true)"
  installed=""
  if [[ -x "$INSTALL_DIR/$yt_name" ]]; then installed="$("$INSTALL_DIR/$yt_name" --version 2>/dev/null || true)"; fi
  if [[ "$FORCE_INSTALL" != "true" && -n "$latest" && "$(version_without_v "$latest")" == "$(version_without_v "$installed")" ]]; then
    done_message "yt-dlp is up to date ($installed)."
    return
  fi

  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-ytdlp.XXXXXX")"
  TEMPORARY_PATHS+=("$temporary_dir")
  binary="$temporary_dir/$asset"
  sums="$temporary_dir/SHA2-256SUMS"
  log "Downloading the latest official yt-dlp binary and checksum from GitHub…"
  download_file "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$asset" "$binary"
  download_file 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS' "$sums"
  expected="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$sums")"
  [[ -n "$expected" ]] || fail "Could not find the checksum for $asset."
  actual="$(sha256_file "$binary")"
  [[ "$actual" == "$expected" ]] || fail "The yt-dlp SHA-256 verification failed."
  mkdir -p "$INSTALL_DIR"
  install_binary "$binary" "$INSTALL_DIR/$yt_name"
  done_message "yt-dlp ${latest:-latest} was installed."
}

install_windows_deps() {
  if ! command -v winget >/dev/null 2>&1; then
    warn "winget (Windows Package Manager) was not found. Install Node.js, FFmpeg and Deno manually, then run install.sh --no-deps."
    return 0
  fi
  local name package_id spec
  for spec in "node:OpenJS.NodeJS.LTS" "ffmpeg:Gyan.FFmpeg" "deno:DenoLand.Deno"; do
    name="${spec%%:*}"
    package_id="${spec#*:}"
    if command -v "$name" >/dev/null 2>&1; then
      done_message "$name is already available."
    else
      log "Installing $name with winget…"
      winget install --id "$package_id" --exact --silent --accept-package-agreements --accept-source-agreements >/dev/null 2>&1 \
        || warn "$name installation failed; install it manually and re-run install.sh."
    fi
  done
}

resolve_cli_source() {
  local script_dir latest ref temporary_dir candidate
  script_dir=""
  if [[ -n "${BASH_SOURCE[0]:-}" && -f "${BASH_SOURCE[0]}" ]]; then
    if ! script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd)"; then
      script_dir=""
    fi
    candidate="$script_dir/bin/youtube-gir"
    if [[ -f "$candidate" ]]; then
      SOURCE_CLI="$candidate"
      return
    fi
  fi

  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-app.XXXXXX")"
  TEMPORARY_PATHS+=("$temporary_dir")
  candidate="$temporary_dir/youtube-gir"
  latest="$(github_latest_tag "$REPOSITORY" || true)"
  ref="${latest:-main}"
  log "Downloading youtube-gir from GitHub (${ref})…"
  download_file "$RAW_ROOT/$ref/bin/youtube-gir" "$candidate"
  SOURCE_CLI="$candidate"
}

install_alias() {
  if [[ "$OS_NAME" == "windows" ]]; then
    cat > "$INSTALL_DIR/youtube-git" <<'EOF'
#!/usr/bin/env bash
exec "$(dirname "$0")/youtube-gir" "$@"
EOF
    chmod 0755 "$INSTALL_DIR/youtube-git"
  else
    ln -sf "$INSTALL_DIR/youtube-gir" "$INSTALL_DIR/youtube-git"
  fi
}

install_or_update_app() {
  local source_cli="$1" candidate_version installed_version
  candidate_version="$(sed -n 's/^readonly APP_VERSION="\([^"]*\)"/\1/p' "$source_cli" | head -n 1)"
  [[ -n "$candidate_version" ]] || fail "Could not determine the youtube-gir version from the downloaded file."
  installed_version=""
  if [[ -x "$INSTALL_DIR/youtube-gir" ]]; then
    installed_version="$("$INSTALL_DIR/youtube-gir" --version 2>/dev/null | awk '{print $2}' || true)"
  fi
  mkdir -p "$INSTALL_DIR"
  if [[ "$FORCE_INSTALL" != "true" && "$installed_version" == "$candidate_version" ]]; then
    done_message "youtube-gir is up to date ($installed_version)."
  else
    install_binary "$source_cli" "$INSTALL_DIR/youtube-gir"
    done_message "youtube-gir $candidate_version was installed."
  fi
  install_alias
}

command -v curl >/dev/null 2>&1 || fail "curl is required for installation."
SOURCE_CLI=""
resolve_cli_source

if [[ "$INSTALL_DEPS" == "true" ]]; then
  if [[ "$OS_NAME" == "windows" ]]; then
    install_windows_deps
  else
    bootstrap_brew
    log "Updating the Homebrew package index…"
    brew update
    update_brew_formula node Node.js
    update_brew_formula ffmpeg FFmpeg
    install_or_update_deno
  fi
  install_or_update_ytdlp
fi

install_or_update_app "$SOURCE_CLI"

done_message "Main command: $INSTALL_DIR/youtube-gir"
done_message "Compatibility alias: $INSTALL_DIR/youtube-git"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf '⚠️  Add this line to ~/.zshrc or ~/.bashrc:\n    export PATH="%s:%s"\n' "$INSTALL_DIR" "\$PATH" ;;
esac
printf '\nStart with: %s/youtube-gir doctor\n' "$INSTALL_DIR"
