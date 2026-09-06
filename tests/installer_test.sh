#!/usr/bin/env bash

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/youtube-gir-installer-test.XXXXXX")"
trap 'rm -rf "$TEST_ROOT"' EXIT
mkdir -p "$TEST_ROOT/standalone" "$TEST_ROOT/mock-bin" "$TEST_ROOT/home"
cp "$PROJECT_DIR/install.sh" "$TEST_ROOT/standalone/install.sh"

cat > "$TEST_ROOT/mock-bin/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
destination=""
previous=""
url=""
for argument in "$@"; do
  if [[ "$previous" == "-o" ]]; then destination="$argument"; fi
  previous="$argument"
  case "$argument" in http://*|https://*) url="$argument" ;; esac
done
if [[ "$url" == *api.github.com* ]]; then
  printf '{"tag_name":"v1.0.0"}\n'
elif [[ "$url" == *raw.githubusercontent.com* && -n "$destination" ]]; then
  cp "$MOCK_YOUTUBE_GIR_CLI" "$destination"
else
  printf 'Unexpected curl call: %s\n' "$*" >&2
  exit 1
fi
EOF
chmod +x "$TEST_ROOT/mock-bin/curl"

export HOME="$TEST_ROOT/home"
export PATH="$TEST_ROOT/mock-bin:/usr/bin:/bin"
export MOCK_YOUTUBE_GIR_CLI="$PROJECT_DIR/bin/youtube-gir"
export YOUTUBE_GIR_BIN_DIR="$TEST_ROOT/home/.local/bin"

output="$(bash "$TEST_ROOT/standalone/install.sh" --no-deps)"
[[ "$output" == *'youtube-gir 1.0.0 نصب شد'* ]]
[[ -x "$YOUTUBE_GIR_BIN_DIR/youtube-gir" ]]
[[ -L "$YOUTUBE_GIR_BIN_DIR/youtube-git" ]]

output="$(bash "$TEST_ROOT/standalone/install.sh" --no-deps)"
[[ "$output" == *'youtube-gir به‌روز است (1.0.0)'* ]]
[[ "$("$YOUTUBE_GIR_BIN_DIR/youtube-git" --version)" == 'youtube-gir 1.0.0' ]]

rm -rf "$YOUTUBE_GIR_BIN_DIR"
output="$(cd "$TEST_ROOT" && bash -s -- --no-deps < "$TEST_ROOT/standalone/install.sh")"
[[ "$output" == *'دریافت youtube-gir از GitHub'* ]]
[[ -x "$YOUTUBE_GIR_BIN_DIR/youtube-gir" ]]

printf '✅ Installer tests passed\n'
