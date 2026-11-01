#!/usr/bin/env bash
# Start one built server and check health plus the admin account.
# Usage: scripts/smoke.sh <binary> <expected-version>
set -euo pipefail

bin=${1:?binary path}
expect=${2:?expected version}
if [[ ! -x "$bin" && ! -f "$bin" ]]; then
  echo "binary not found: $bin" >&2
  exit 1
fi

dir=$(mktemp -d)
log="${dir}/server.log"
body="${dir}/body.json"
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
key=$(openssl rand -hex 16)

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

echo "smoke ok ${bin}"
