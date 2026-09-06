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

log() { printf 'ℹ️  %s\n' "$*"; }
done_message() { printf '✅ %s\n' "$*"; }
fail() { printf '❌ %s\n' "$*" >&2; exit 1; }

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
  [[ "$(uname -s)" == "Darwin" || "$(uname -s)" == "Linux" ]] || fail "Homebrew فقط روی macOS/Linux پشتیبانی می‌شود."
  if [[ "$ASSUME_YES" != "true" && -t 0 ]]; then
    printf 'Homebrew نصب نیست. نصب‌کنندهٔ رسمی brew.sh اجرا شود؟ [y/N] '
    read -r answer
    [[ "$answer" =~ ^[Yy]$ ]] || fail "نصب لغو شد."
  fi
  local temporary_dir installer
  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-brew.XXXXXX")"
  TEMPORARY_PATHS+=("$temporary_dir")
  installer="$temporary_dir/install.sh"
  log "دریافت نصب‌کنندهٔ رسمی Homebrew…"
  download_file 'https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh' "$installer"
  if [[ "$ASSUME_YES" == "true" ]]; then NONINTERACTIVE=1 /bin/bash "$installer"; else /bin/bash "$installer"; fi
  if [[ -x /opt/homebrew/bin/brew ]]; then eval "$(/opt/homebrew/bin/brew shellenv)"; fi
  if [[ -x /home/linuxbrew/.linuxbrew/bin/brew ]]; then eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"; fi
  command -v brew >/dev/null 2>&1 || fail "Homebrew نصب شد اما هنوز در PATH نیست؛ ترمینال را باز کنید و دوباره اجرا کنید."
}

update_brew_formula() {
  local formula="$1" label="$2"
  if ! brew list "$formula" >/dev/null 2>&1; then
    log "نصب $label از Homebrew…"
    brew install "$formula"
  elif brew outdated --quiet "$formula" | grep -q .; then
    log "نسخهٔ جدید $label پیدا شد؛ در حال ارتقا…"
    brew upgrade "$formula"
  else
    done_message "$label به‌روز است."
  fi
}

install_or_update_deno() {
  if brew list deno >/dev/null 2>&1; then
    update_brew_formula deno Deno
    return
  fi
  if command -v deno >/dev/null 2>&1; then
    log "بررسی نسخهٔ Deno با updater رسمی…"
    deno upgrade
    return
  fi
  local temporary_dir installer
  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-deno.XXXXXX")"
  TEMPORARY_PATHS+=("$temporary_dir")
  installer="$temporary_dir/install.sh"
  log "دریافت Deno از deno.land…"
  download_file 'https://deno.land/install.sh' "$installer"
  sh "$installer"
  export PATH="$HOME/.deno/bin:$PATH"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}';
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

install_or_update_ytdlp() {
  local system machine asset temporary_dir binary sums expected actual latest installed
  system="$(uname -s)"
  machine="$(uname -m)"
  case "$system:$machine" in
    Darwin:*) asset="yt-dlp_macos" ;;
    Linux:x86_64|Linux:amd64) asset="yt-dlp_linux" ;;
    Linux:aarch64|Linux:arm64) asset="yt-dlp_linux_aarch64" ;;
    *) fail "باینری رسمی yt-dlp برای $system/$machine در این نصب‌کننده تعریف نشده است." ;;
  esac

  latest="$(github_latest_tag 'yt-dlp/yt-dlp' || true)"
  installed=""
  if [[ -x "$INSTALL_DIR/yt-dlp" ]]; then installed="$("$INSTALL_DIR/yt-dlp" --version 2>/dev/null || true)"; fi
  if [[ "$FORCE_INSTALL" != "true" && -n "$latest" && "$(version_without_v "$latest")" == "$(version_without_v "$installed")" ]]; then
    done_message "yt-dlp به‌روز است ($installed)."
    return
  fi

  temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-ytdlp.XXXXXX")"
  TEMPORARY_PATHS+=("$temporary_dir")
  binary="$temporary_dir/$asset"
  sums="$temporary_dir/SHA2-256SUMS"
  log "دریافت آخرین باینری رسمی yt-dlp و checksum از GitHub…"
  download_file "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$asset" "$binary"
  download_file 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS' "$sums"
  expected="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$sums")"
  [[ -n "$expected" ]] || fail "checksum مربوط به $asset پیدا نشد."
  actual="$(sha256_file "$binary")"
  [[ "$actual" == "$expected" ]] || fail "بررسی SHA-256 باینری yt-dlp ناموفق بود."
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$binary" "$INSTALL_DIR/yt-dlp"
  done_message "yt-dlp ${latest:-latest} نصب شد."
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
  log "دریافت youtube-gir از GitHub (${ref})…"
  download_file "$RAW_ROOT/$ref/bin/youtube-gir" "$candidate"
  SOURCE_CLI="$candidate"
}

install_or_update_app() {
  local source_cli="$1" candidate_version installed_version
  candidate_version="$(sed -n 's/^readonly APP_VERSION="\([^"]*\)"/\1/p' "$source_cli" | head -n 1)"
  [[ -n "$candidate_version" ]] || fail "نسخهٔ youtube-gir در فایل دریافتی پیدا نشد."
  installed_version=""
  if [[ -x "$INSTALL_DIR/youtube-gir" ]]; then
    installed_version="$("$INSTALL_DIR/youtube-gir" --version 2>/dev/null | awk '{print $2}' || true)"
  fi
  mkdir -p "$INSTALL_DIR"
  if [[ "$FORCE_INSTALL" != "true" && "$installed_version" == "$candidate_version" ]]; then
    done_message "youtube-gir به‌روز است ($installed_version)."
  else
    install -m 0755 "$source_cli" "$INSTALL_DIR/youtube-gir"
    done_message "youtube-gir $candidate_version نصب شد."
  fi
  ln -sf "$INSTALL_DIR/youtube-gir" "$INSTALL_DIR/youtube-git"
}

command -v curl >/dev/null 2>&1 || fail "curl برای نصب لازم است."
SOURCE_CLI=""
resolve_cli_source

if [[ "$INSTALL_DEPS" == "true" ]]; then
  bootstrap_brew
  log "به‌روزرسانی فهرست بسته‌های Homebrew…"
  brew update
  update_brew_formula node Node.js
  update_brew_formula ffmpeg FFmpeg
  install_or_update_deno
  install_or_update_ytdlp
fi

install_or_update_app "$SOURCE_CLI"

done_message "فرمان اصلی: $INSTALL_DIR/youtube-gir"
done_message "alias سازگار: $INSTALL_DIR/youtube-git"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf '⚠️  این خط را به ~/.zshrc یا ~/.bashrc اضافه کنید:\n    export PATH="%s:%s"\n' "$INSTALL_DIR" "\$PATH" ;;
esac
printf '\nشروع: %s/youtube-gir doctor\n' "$INSTALL_DIR"
