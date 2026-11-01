#!/usr/bin/env bash
# Build the server and the desktop app for macOS, Linux, and Windows, and zip the extension.
# VERSION is the release tag (v1.2.3) or "dev".
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

version="${VERSION:-dev}"
if [[ ! "$version" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "VERSION must be letters, numbers, dots, underscores, or hyphens" >&2
  exit 1
fi
internal="${version#v}"

rm -rf dist
mkdir -p dist

build_bin() {
  local prefix=$1
  local pkg=$2
  local goos=$3
  local goarch=$4
  local ext=""
  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
  fi
  local name="${prefix}-${version}-${goos}-${goarch}${ext}"
  echo "build ${name}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath \
    -ldflags="-s -w -X github.com/WhoisGray/youtube-gir/internal/config.Version=${internal}" \
    -o "dist/${name}" "$pkg"
}

while read -r goos goarch; do
  build_bin youtubegir ./cmd/youtubegir "$goos" "$goarch"
  build_bin youtubegir-gui ./cmd/youtubegir-gui "$goos" "$goarch"
done <<'EOF'
darwin amd64
darwin arm64
linux amd64
linux arm64
windows amd64
EOF

expect_file() {
  local path=$1
  local info
  info=$(file -b "$path")
  echo "${path}: ${info}"
  shift
  local needle
  for needle in "$@"; do
    case "$info" in
      *"$needle"*) ;;
      *)
        echo "expected '${needle}' in ${path}" >&2
        exit 1
        ;;
    esac
  done
}

expect_file "dist/youtubegir-${version}-darwin-amd64" "Mach-O" "x86_64"
expect_file "dist/youtubegir-${version}-darwin-arm64" "Mach-O" "arm64"
expect_file "dist/youtubegir-${version}-linux-amd64" "ELF" "x86-64"
expect_file "dist/youtubegir-${version}-linux-arm64" "ELF" "aarch64"
expect_file "dist/youtubegir-${version}-windows-amd64.exe" "PE32+" "x86-64"
expect_file "dist/youtubegir-gui-${version}-darwin-amd64" "Mach-O" "x86_64"
expect_file "dist/youtubegir-gui-${version}-darwin-arm64" "Mach-O" "arm64"
expect_file "dist/youtubegir-gui-${version}-linux-amd64" "ELF" "x86-64"
expect_file "dist/youtubegir-gui-${version}-linux-arm64" "ELF" "aarch64"
expect_file "dist/youtubegir-gui-${version}-windows-amd64.exe" "PE32+" "x86-64"

bundle_mac_app() (
  set -euo pipefail
  local goarch=$1
  local bin="dist/youtubegir-gui-${version}-darwin-${goarch}"
  local work
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT
  local app="${work}/youtube-gir.app"
  mkdir -p "${app}/Contents/MacOS" "${app}/Contents/Resources"
  cp "$bin" "${app}/Contents/MacOS/youtube-gir"
  chmod 755 "${app}/Contents/MacOS/youtube-gir"
  cp packaging/macos/AppIcon.icns "${app}/Contents/Resources/AppIcon.icns"
  cp packaging/macos/README.txt "${work}/README.txt"
  printf 'APPL????' > "${app}/Contents/PkgInfo"
  INTERNAL="$internal" python3 - "$app/Contents/Info.plist" packaging/macos/Info.plist <<'PY'
import os, pathlib, sys
src = pathlib.Path(sys.argv[2]).read_text(encoding="utf-8")
version = os.environ["INTERNAL"]
text = src.replace("__VERSION__", version)
if "__VERSION__" in text:
    raise SystemExit("version placeholder left in Info.plist")
pathlib.Path(sys.argv[1]).write_text(text, encoding="utf-8")
PY
  local zip_path="dist/youtube-gir-${version}-darwin-${goarch}.app.zip"
  python3 - "$work" "$zip_path" "$internal" <<'PY'
import os, sys, zipfile
from pathlib import Path
root = Path(sys.argv[1])
dest = Path(sys.argv[2])
version = sys.argv[3]
def add(zf, path):
    rel = path.relative_to(root).as_posix()
    if path.is_dir():
        rel = rel.rstrip("/") + "/"
    info = zipfile.ZipInfo(rel)
    info.create_system = 3
    info.external_attr = (path.stat().st_mode & 0xFFFF) << 16
    info.compress_type = zipfile.ZIP_DEFLATED
    if path.is_dir():
        zf.writestr(info, b"")
        return
    zf.writestr(info, path.read_bytes())
with zipfile.ZipFile(dest, "w") as zf:
    for path in sorted(root.rglob("*")):
        if path.is_dir() or path.is_file():
            add(zf, path)
need = [
    "README.txt",
    "youtube-gir.app/Contents/Info.plist",
    "youtube-gir.app/Contents/MacOS/youtube-gir",
    "youtube-gir.app/Contents/Resources/AppIcon.icns",
    "youtube-gir.app/Contents/PkgInfo",
]
archive = zipfile.ZipFile(dest)
names = set(archive.namelist())
missing = [name for name in need if name not in names]
if missing:
    raise SystemExit("app zip missing " + ", ".join(missing))
info = archive.getinfo("youtube-gir.app/Contents/MacOS/youtube-gir")
mode = (info.external_attr >> 16) & 0o777
if mode & 0o111 == 0:
    raise SystemExit("app binary is not marked executable in the zip")
plist = archive.read("youtube-gir.app/Contents/Info.plist")
if b"<string>youtube-gir</string>" not in plist or version.encode() not in plist:
    raise SystemExit("Info.plist is missing the executable name or version")
if b"__VERSION__" in plist:
    raise SystemExit("Info.plist still has a version placeholder")
print("app zip", dest, "files", len(names), "mode", oct(mode))
PY
)

bundle_mac_app arm64
bundle_mac_app amd64

if ! command -v node >/dev/null 2>&1; then
  echo "node is required to check the extension" >&2
  exit 1
fi
while IFS= read -r js; do
  node --check "$js"
done < <(find extension -name '*.js' -type f | sort)
node --check internal/gui/assets/app.js

stage=$(mktemp -d)
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT
cp -R extension/. "$stage/"
find "$stage" -name '.DS_Store' -delete
INTERNAL="$internal" python3 - "$stage/manifest.json" <<'PY'
import json, os, re, sys
path = sys.argv[1]
with open(path, encoding="utf-8") as fh:
    manifest = json.load(fh)
version = os.environ["INTERNAL"]
if re.fullmatch(r"\d+(\.\d+){0,3}", version):
    manifest["version"] = version
with open(path, "w", encoding="utf-8") as fh:
    json.dump(manifest, fh, indent=2)
    fh.write("\n")
PY

zip_path="dist/youtube-gir-extension-${version}.zip"
python3 - "$stage" "$zip_path" <<'PY'
import sys, zipfile
from pathlib import Path
stage = Path(sys.argv[1])
dest = Path(sys.argv[2])
with zipfile.ZipFile(dest, "w", compression=zipfile.ZIP_DEFLATED) as zf:
    for path in sorted(stage.rglob("*")):
        if path.is_file() and path.name != ".DS_Store":
            zf.write(path, path.relative_to(stage).as_posix())
PY
trap - EXIT
rm -rf "$stage"

python3 - "$zip_path" "$internal" <<'PY'
import json, re, sys, zipfile
archive = zipfile.ZipFile(sys.argv[1])
names = set(archive.namelist())
required = [
    "manifest.json",
    "src/background.js",
    "src/content.js",
    "src/popup.js",
    "src/options.js",
    "icons/icon128.png",
]
missing = [name for name in required if name not in names]
if missing:
    raise SystemExit("extension zip missing " + ", ".join(missing))
manifest = json.loads(archive.read("manifest.json"))
if manifest.get("manifest_version") != 3:
    raise SystemExit("manifest_version is not 3")
version = sys.argv[2]
if re.fullmatch(r"\d+(\.\d+){0,3}", version) and manifest.get("version") != version:
    raise SystemExit("manifest version is " + str(manifest.get("version")))
print("zip", sys.argv[1], "files", len(names), "version", manifest.get("version"))
PY

check_dir=$(mktemp -d)
python3 - "$zip_path" "$check_dir" <<'PY'
import sys, zipfile
from pathlib import Path
dest = Path(sys.argv[2])
with zipfile.ZipFile(sys.argv[1]) as zf:
    zf.extractall(dest)
PY
while IFS= read -r js; do
  node --check "$js"
done < <(find "$check_dir" -name '*.js' -type f | sort)
rm -rf "$check_dir"

python3 - <<'PY'
import hashlib
from pathlib import Path
root = Path("dist")
lines = []
for path in sorted(root.iterdir()):
    if not path.is_file() or path.name == "SHA256SUMS":
        continue
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    lines.append(f"{digest}  {path.name}")
(root / "SHA256SUMS").write_text("\n".join(lines) + "\n", encoding="utf-8")
print("\n".join(lines))
PY

echo "dist ready"
