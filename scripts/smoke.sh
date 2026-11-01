#!/usr/bin/env bash
# Start one built server and check health plus the admin account.
# Usage: scripts/smoke.sh <binary> <expected-version> [server|gui]
set -euo pipefail

bin=${1:?binary path}
expect=${2:?expected version}
mode=${3:-server}
case "$mode" in
  server|gui) ;;
  *)
    echo "usage: scripts/smoke.sh <binary> <expected-version> [server|gui]" >&2
    exit 1
    ;;
esac
if [[ ! -x "$bin" && ! -f "$bin" ]]; then
  echo "binary not found: $bin" >&2
  exit 1
fi

dir=$(mktemp -d)
log="${dir}/server.log"
body="${dir}/body.json"
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
key=$(openssl rand -hex 16)
if [[ "$mode" == "gui" ]]; then
  export YG_GUI_NO_BROWSER=1
fi

ADMIN_KEY="$key" \
  ADDR="127.0.0.1:${port}" \
  DATA_DIR="$dir/data" \
  JS_RUNTIME=none \
  "$bin" >"$log" 2>&1 &
pid=$!

cleanup() {
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$dir"
}
trap cleanup EXIT

ready=0
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25; do
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "server exited early" >&2
    cat "$log" >&2 || true
    exit 1
  fi
  code=$(curl -sS -o "$body" -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://127.0.0.1:${port}/health" 2>/dev/null || true)
  if [[ "$code" == "200" || "$code" == "503" ]]; then
    ready=1
    break
  fi
  sleep 0.2
done
if [[ "$ready" != "1" ]]; then
  echo "health did not respond" >&2
  cat "$log" >&2 || true
  exit 1
fi

python3 - "$body" "$expect" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
expect = sys.argv[2]
if doc.get("version") != expect:
    raise SystemExit("version %r != %r" % (doc.get("version"), expect))
if doc.get("status") not in ("ok", "degraded"):
    raise SystemExit("bad health status %r" % (doc.get("status"),))
print("health", doc.get("status"), doc.get("version"))
PY

me="${dir}/me.json"
me_code=$(curl -sS -o "$me" -w "%{http_code}" \
  -H "Authorization: Bearer ${key}" \
  "http://127.0.0.1:${port}/api/v1/me")
if [[ "$me_code" != "200" ]]; then
  echo "GET /api/v1/me returned ${me_code}" >&2
  cat "$me" >&2 || true
  exit 1
fi
python3 - "$me" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
if doc.get("username") != "admin" or doc.get("role") != "admin":
    raise SystemExit("unexpected /me: username=%r role=%r" % (doc.get("username"), doc.get("role")))
print("me", doc["username"], doc["role"])
PY

root="${dir}/root.json"
root_code=$(curl -sS -o "$root" -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://127.0.0.1:${port}/")
if [[ "$root_code" != "200" ]]; then
  echo "GET / returned ${root_code}" >&2
  exit 1
fi
python3 - "$root" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
if doc.get("name") != "youtube-gir":
    raise SystemExit("GET / name is %r" % (doc.get("name"),))
print("root", doc.get("name"))
PY

if [[ "$mode" == "gui" ]]; then
  page="${dir}/page.html"
  page_code=$(curl -sS -o "$page" -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://127.0.0.1:${port}/app/")
  if [[ "$page_code" != "200" ]]; then
    echo "GET /app/ returned ${page_code}" >&2
    exit 1
  fi
  if ! grep -F -q 'id="url"' "$page"; then
    echo "app page has no url field" >&2
    exit 1
  fi
  if ! grep -F -q "$key" "$page"; then
    echo "app page did not include the admin key" >&2
    exit 1
  fi
  js_code=$(curl -sS -o /dev/null -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://127.0.0.1:${port}/app/app.js")
  css_code=$(curl -sS -o /dev/null -w "%{http_code}" --connect-timeout 1 --max-time 2 "http://127.0.0.1:${port}/app/app.css")
  if [[ "$js_code" != "200" || "$css_code" != "200" ]]; then
    echo "app assets js=${js_code} css=${css_code}" >&2
    exit 1
  fi
  echo "app page ok"
fi

echo "smoke ok ${bin}"
