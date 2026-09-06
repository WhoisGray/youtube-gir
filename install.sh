#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${YOUTUBE_GIR_BIN_DIR:-$HOME/.local/bin}"
INSTALL_DEPS="true"
ASSUME_YES="false"

for argument in "$@"; do
  case "$argument" in
    --no-deps) INSTALL_DEPS="false" ;;
    --yes|-y) ASSUME_YES="true" ;;
    --help|-h)
      printf 'Usage: ./install.sh [--yes] [--no-deps]\n'
      exit 0
      ;;
    *) printf 'Unknown option: %s\n' "$argument" >&2; exit 2 ;;
  esac
done

log() { printf 'ℹ️  %s\n' "$*"; }
done_message() { printf '✅ %s\n' "$*"; }
fail() { printf '❌ %s\n' "$*" >&2; exit 1; }

download_file() {
  local url="$1" destination="$2"
  curl -fL --retry 3 --connect-timeout 15 -o "$destination" "$url"
}

bootstrap_brew() {
  command -v brew >/dev/null 2>&1 && return
  [[ "$(uname -s)" == "Darwin" || "$(uname -s)" == "Linux" ]] || fail "Homebrew فقط روی macOS/Linux پشتیبانی می‌شود."
  if [[ "$ASSUME_YES" != "true" && -t 0 ]]; then
    printf 'Homebrew نصب نیست. نصب‌کنندهٔ رسمی brew.sh اجرا شود؟ [y/N] '
    read -r answer
    [[ "$answer" =~ ^[Yy]$ ]] || fail "نصب لغو شد."
  fi
  local temporary
  temporary="$(mktemp "${TMPDIR:-/tmp}/homebrew-install.XXXXXX")"
  log "دریافت نصب‌کنندهٔ رسمی Homebrew…"
  download_file 'https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh' "$temporary"
  if [[ "$ASSUME_YES" == "true" ]]; then NONINTERACTIVE=1 /bin/bash "$temporary"; else /bin/bash "$temporary"; fi
  rm -f "$temporary"
  if [[ -x /opt/homebrew/bin/brew ]]; then eval "$(/opt/homebrew/bin/brew shellenv)"; fi
  if [[ -x /home/linuxbrew/.linuxbrew/bin/brew ]]; then eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"; fi
  command -v brew >/dev/null 2>&1 || fail "Homebrew نصب شد اما هنوز در PATH نیست؛ ترمینال را باز کنید و دوباره اجرا کنید."
}

install_deno() {
  command -v deno >/dev/null 2>&1 && return
  local temporary
  temporary="$(mktemp "${TMPDIR:-/tmp}/deno-install.XXXXXX")"
  log "دریافت Deno از deno.land…"
  download_file 'https://deno.land/install.sh' "$temporary"
  sh "$temporary"
  rm -f "$temporary"
  export PATH="$HOME/.deno/bin:$PATH"
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}';
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

install_ytdlp() {
  local system machine asset temporary binary sums expected actual
  system="$(uname -s)"
  machine="$(uname -m)"
  case "$system:$machine" in
    Darwin:*) asset="yt-dlp_macos" ;;
    Linux:x86_64|Linux:amd64) asset="yt-dlp_linux" ;;
    Linux:aarch64|Linux:arm64) asset="yt-dlp_linux_aarch64" ;;
    *) fail "باینری رسمی yt-dlp برای $system/$machine در این نصب‌کننده تعریف نشده است." ;;
  esac
  temporary="$(mktemp -d "${TMPDIR:-/tmp}/yt-dlp-install.XXXXXX")"
  binary="$temporary/$asset"
  sums="$temporary/SHA2-256SUMS"
  log "دریافت باینری رسمی yt-dlp و checksum از GitHub…"
  download_file "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$asset" "$binary"
  download_file 'https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS' "$sums"
  expected="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1; exit }' "$sums")"
  [[ -n "$expected" ]] || fail "checksum مربوط به $asset پیدا نشد."
  actual="$(sha256_file "$binary")"
  [[ "$actual" == "$expected" ]] || fail "بررسی SHA-256 باینری yt-dlp ناموفق بود."
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$binary" "$INSTALL_DIR/yt-dlp"
  rm -rf "$temporary"
}

if [[ "$INSTALL_DEPS" == "true" ]]; then
  command -v curl >/dev/null 2>&1 || fail "curl برای نصب لازم است."
  bootstrap_brew
  log "نصب Node.js و FFmpeg از Homebrew…"
  brew list node >/dev/null 2>&1 || brew install node
  brew list ffmpeg >/dev/null 2>&1 || brew install ffmpeg
  install_deno
  install_ytdlp
fi

[[ -f "$SCRIPT_DIR/bin/youtube-gir" ]] || fail "فایل bin/youtube-gir پیدا نشد."
mkdir -p "$INSTALL_DIR"
install -m 0755 "$SCRIPT_DIR/bin/youtube-gir" "$INSTALL_DIR/youtube-gir"
ln -sf "$INSTALL_DIR/youtube-gir" "$INSTALL_DIR/youtube-git"

done_message "youtube-gir نصب شد: $INSTALL_DIR/youtube-gir"
done_message "alias سازگار نصب شد: $INSTALL_DIR/youtube-git"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) printf '⚠️  این خط را به ~/.zshrc یا ~/.bashrc اضافه کنید:\n    export PATH="%s:%s"\n' "$INSTALL_DIR" "\$PATH" ;;
esac
printf '\nشروع: youtube-gir doctor\n'
