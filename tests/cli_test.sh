#!/usr/bin/env bash

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-test.XXXXXX")"
trap 'rm -rf "$TEST_DIR"' EXIT
mkdir -p "$TEST_DIR/bin" "$TEST_DIR/home"

cat > "$TEST_DIR/bin/yt-dlp" <<'EOF'
#!/usr/bin/env bash
printf 'fake yt-dlp\n'
EOF
chmod +x "$TEST_DIR/bin/yt-dlp"

export HOME="$TEST_DIR/home"
export XDG_CONFIG_HOME="$TEST_DIR/config"
export XDG_CACHE_HOME="$TEST_DIR/cache"
export XDG_DATA_HOME="$TEST_DIR/data"
export YOUTUBE_GIR_YTDLP="$TEST_DIR/bin/yt-dlp"
export NO_COLOR=1

assert_contains() {
  [[ "$1" == *"$2"* ]] || { printf 'Expected output to contain: %s\nActual:\n%s\n' "$2" "$1" >&2; exit 1; }
}

output="$(bash "$PROJECT_DIR/bin/youtube-gir" --version)"
assert_contains "$output" 'youtube-gir 1.0.0'

output="$(bash "$PROJECT_DIR/bin/youtube-gir" --dry-run --no-cookies 'https://youtu.be/test')"
assert_contains "$output" 'height\<=1080'
assert_contains "$output" '--no-playlist'
assert_contains "$output" '--cache-dir'
assert_contains "$output" '--sponsorblock-remove sponsor\,selfpromo'

output="$(bash "$PROJECT_DIR/bin/youtube-gir" --no-sponsorblock --dry-run --no-cookies 'https://youtu.be/test')"
if [[ "$output" == *'--sponsorblock-remove'* ]]; then
  printf 'SponsorBlock was not disabled by --no-sponsorblock\n' >&2
  exit 1
fi

output="$(bash "$PROJECT_DIR/bin/youtube-gir" --audio -q low --sponsorblock --archive --dry-run --no-cookies 'https://youtu.be/test')"
assert_contains "$output" '--audio-format m4a'
assert_contains "$output" '-b:a\ 96k'
assert_contains "$output" '--sponsorblock-remove sponsor\,selfpromo'
assert_contains "$output" '--download-archive'

output="$(bash "$PROJECT_DIR/bin/youtube-gir" --video -q good --fast --playlist --dry-run --no-cookies 'https://youtu.be/test')"
assert_contains "$output" 'height\<=480'
assert_contains "$output" '--concurrent-fragments 4'
assert_contains "$output" '--yes-playlist'
if [[ "$output" == *'--embed-thumbnail'* ]]; then
  printf 'Fast mode unexpectedly embeds thumbnail\n' >&2
  exit 1
fi

output="$(bash "$PROJECT_DIR/bin/youtube-gir" --verbose --dry-run --no-cookies 'https://youtu.be/test')"
assert_contains "$output" '--verbose'

mkdir -p "$XDG_CONFIG_HOME/youtube-gir"
cat > "$XDG_CONFIG_HOME/youtube-gir/config" <<EOF
default_media=audio
default_quality=normal
audio_dir=$TEST_DIR/audio files
use_cookies=false
sponsorblock=true
use_archive=false
EOF
output="$(bash "$PROJECT_DIR/bin/youtube-gir" --dry-run 'https://youtu.be/test')"
assert_contains "$output" '--audio-format m4a'
assert_contains "$output" '-b:a\ 160k'
assert_contains "$output" 'audio\ files'
assert_contains "$output" '--sponsorblock-remove'

if bash "$PROJECT_DIR/bin/youtube-gir" --quality impossible --dry-run 'https://youtu.be/test' >/dev/null 2>&1; then
  printf 'Invalid quality unexpectedly succeeded\n' >&2
  exit 1
fi

printf '✅ CLI tests passed\n'
