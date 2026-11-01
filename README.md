# youtube-gir

A Chrome extension and a small Go server for queuing YouTube video and audio downloads. The extension adds a download button on the watch page. The server runs `yt-dlp`, keeps a queue in SQLite, and hands each finished file a private link.

The original command-line tool is still in this repository. Its guide is [docs/cli.fa.md](docs/cli.fa.md). Persian documentation for the server and extension is in [README.fa.md](README.fa.md).

You are responsible for having the rights to download the media you request.

## Architecture

```mermaid
flowchart LR
  ext[Chrome extension] --> api[Go API]
  api --> db[(SQLite)]
  api --> worker[Queue worker]
  worker --> ytdlp[yt-dlp]
  ytdlp --> tmp[Temp volume]
  worker --> lib[Downloads volume]
```

The extension never downloads the video itself. It talks to the server you run, locally or on another machine. Each person has an API key. A person only sees their own queue. The admin key sees every user and every download.

Finished bytes are copied onto the downloads volume. Partial `yt-dlp` files stay on the temp volume, so a pause, a crash, or a dropped network can continue from the same place.

## Phases

All four phases below are in this tree.

**Phase 1 — Core download and queue.** REST API, video and audio jobs, format and file-size lookup, SQLite queue, and a worker with a configurable concurrency limit.

**Phase 2 — Extension UI.** Manifest V3 button on YouTube watch and Shorts pages, quality picker, popup dashboard with live progress, Apple-style light and dark UI. Strings are English and Persian.

**Phase 3 — Admin and keys.** The admin key creates a user key from a username. Users are isolated. Admins can list everyone. Each download gets a file token and a direct download URL.

**Phase 4 — Resume and operations.** Dedicated temp directory, interrupt handling that keeps `.part` files, requeue after a restart, Docker volumes, health checks, and a yt-dlp update endpoint.

## Downloads

A `v*` tag publishes a GitHub release with the server and the extension. The file list and checksums are on the release. Current builds:

| File | Machine |
| --- | --- |
| `youtubegir-<tag>-darwin-arm64` | Mac, Apple silicon |
| `youtubegir-<tag>-darwin-amd64` | Mac, Intel |
| `youtubegir-<tag>-linux-amd64` | Linux, 64-bit |
| `youtubegir-<tag>-linux-arm64` | Linux, ARM 64-bit |
| `youtubegir-<tag>-windows-amd64.exe` | Windows, 64-bit |
| `youtube-gir-extension-<tag>.zip` | Chrome extension |

https://github.com/WhoisGray/youtube-gir/releases

The server binary does not include yt-dlp, ffmpeg, or deno. Install those beside it.

```bash
chmod +x youtubegir-v1.0.0-darwin-arm64
export ADMIN_KEY="$(openssl rand -hex 32)"
export ADDR=127.0.0.1:6543
export DATA_DIR="$HOME/.youtubegir"
./youtubegir-v1.0.0-darwin-arm64
```

On Windows, set `ADMIN_KEY` and `ADDR`, then run the `.exe`. Unzip the extension, open `chrome://extensions`, turn on Developer mode, and choose **Load unpacked** on the unzipped folder.

macOS may block the binary the first time because it is not notarized. Allow it under Privacy and Security. Windows SmartScreen can show the same kind of prompt.

## Run with Docker

Docker Desktop on a Mac with Apple silicon builds a native `linux/arm64` image. Do not force `linux/amd64`.

```bash
cp .env.example .env
# put a long random value in ADMIN_KEY
openssl rand -hex 32

docker compose up -d --build
docker compose ps
curl -s http://127.0.0.1:6543/health
```

Compose stores the database, finished files, and partial files in three named volumes: `youtubegir-db`, `youtubegir-downloads`, and `youtubegir-tmp`. The container restarts unless you stop it, so it keeps running in the background.

```bash
docker compose logs -f
docker compose stop
docker compose start
```

The published port is `PORT` in `.env` (default `6543`).

## Run natively

On a Mac:

```bash
brew install go ffmpeg yt-dlp deno
export ADMIN_KEY="$(openssl rand -hex 32)"
export ADDR=127.0.0.1:6543
go run ./cmd/youtubegir
```

`deno` (or Node) lets current yt-dlp solve YouTube’s player script. The server picks `deno` automatically when it is on `PATH`. Set `JS_RUNTIME=none` to turn that off.

Build a binary and leave it running:

```bash
make build
export ADMIN_KEY="$(openssl rand -hex 32)"
export ADDR=127.0.0.1:6543
export DATA_DIR="$HOME/.youtubegir"
nohup dist/youtubegir >> "$HOME/Library/Logs/youtubegir.log" 2>&1 &
```

Data lands in `DATA_DIR` (`./data` in the repository when you do not set the variable).

## Install the extension

1. Open `chrome://extensions`.
2. Turn on Developer mode.
3. Choose **Load unpacked** and select the `extension` folder in this repository.
4. Open the extension’s options (right-click the icon, or the gear in the popup).
5. Server URL: `http://127.0.0.1:6543` for a server on this Mac. For any other host, Chrome asks for permission when you save or test.
6. Paste an API key. Keys are stored with `chrome.storage.local` and are not synced to a Google account.

On a YouTube watch page or Short, a **Download** pill appears in the action row (or at the corner if YouTube’s layout has no row). It loads qualities and sizes, then queues a video or a high-quality M4A audio file. The popup shows progress, total completed size, and actions to pause, resume, cancel, save, or delete. A finished title opens the original video. The popup polls the server about once a second while it is open.

<img src="docs/images/admin.png" alt="Admin tab: add a person and set the download proxy" width="480">

<img src="docs/images/user.png" alt="Download popup with a finished video" width="320">

<img src="docs/images/extension.png" alt="Watch page button with Best video, Audio, and Car" width="340">

If the browser language is Persian, the extension UI is Persian. Otherwise it is English. The rules live in `extension/src/i18n.js`.

## Generate a user key

The admin key is `ADMIN_KEY`. It is also the admin API key.

```bash
curl -s -X POST http://127.0.0.1:6543/api/v1/users \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice"}'
```

```json
{
  "id": 2,
  "username": "alice",
  "role": "user",
  "token": "…64-byte hex…",
  "created_at": "2026-11-01T12:00:00Z"
}
```

Give `token` to that person. The server stores it, but later user-list calls do not repeat it. Usernames are 2–32 characters: letters, numbers, `_`, and `-`. `admin` is reserved.

An admin can do the same from the extension settings. Open the gear, choose **Admin**, type a username, and press **Generate key**. The new key is shown once and copied to the clipboard.

## API

Send the key as `Authorization: Bearer <key>` or `X-API-Key`. Responses are JSON. Errors look like `{"error":"…","code":"…"}`.

| Method | Path | Who | Purpose |
| --- | --- | --- | --- |
| GET | `/health` | anyone | Process, yt-dlp, and ffmpeg status |
| GET | `/api/v1/me` | user | Name, role, personal stats |
| GET | `/api/v1/stats?scope=all` | user; `scope=all` is admin | Counts and completed bytes |
| POST | `/api/v1/formats` | user | Qualities and file sizes. Body: `{"url":"…"}` |
| POST | `/api/v1/downloads` | user | Queue a job. See below |
| GET | `/api/v1/downloads?limit=100&scope=all` | user; `scope=all` is admin | Queue plus stats |
| GET | `/api/v1/downloads/{id}` | owner or admin | One job |
| POST | `/api/v1/downloads/{id}/pause` | owner or admin | Pause. Keeps temp files |
| POST | `/api/v1/downloads/{id}/resume` | owner or admin | Queue a paused or failed job again |
| POST | `/api/v1/downloads/{id}/cancel` | owner or admin | Cancel and delete temp files |
| DELETE | `/api/v1/downloads/{id}` | owner or admin | Delete the job and its file |
| GET | `/api/v1/files/{file_token}` | anyone with the token | Download the finished file |
| POST | `/api/v1/users` | admin | Create a user. Body: `{"username":"alice"}` |
| GET | `/api/v1/users` | admin | Users and download counts. No tokens |
| DELETE | `/api/v1/users/{id}` | admin | Delete a user, their rows, and their files |
| GET | `/api/v1/system/ytdlp` | admin | yt-dlp version |
| POST | `/api/v1/system/ytdlp/update` | admin | Run `yt-dlp -U` |
| GET | `/api/v1/system/proxy` | admin | Proxy used for video fetches and downloads |
| PUT | `/api/v1/system/proxy` | admin | Set or clear that proxy. Body: `{"proxy":"socks5://127.0.0.1:1080"}` |

Queue a download:

```bash
curl -s -X POST http://127.0.0.1:6543/api/v1/downloads \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.youtube.com/watch?v=VIDEO_ID","kind":"video","quality":"1080"}'
```

`kind` is `video` or `audio`. `quality` for video is `best`, `car` (H.264 up to 1080p, AAC 192 kbps, 44.1 kHz, MP4), a standard height (`144`, `240`, `360`, `480`, `720`, `1080`, `1440`, `2160`, `4320`), or a yt-dlp format id such as `137+140`. Audio always takes the best audio and writes M4A. Playlist parameters are ignored. The same active URL, kind, and quality returns the existing job with `"duplicate": true`.

The `202` body includes `download.file_token`. That token is the file’s private key:

```text
http://127.0.0.1:6543/api/v1/files/<file_token>
```

The file route does not use the API key, because browser downloads cannot set one. Treat the URL like a password.

Only `youtube.com`, `youtu.be`, `music.youtube.com`, and `youtube-nocookie.com` video URLs are accepted.

## Resume

Each job gets a directory under the temp volume. yt-dlp is started with `--continue` and that same directory on every attempt. Pause and shutdown send `SIGINT` to the process group (so ffmpeg stops too) and leave `.part` files in place. A hard kill of the server is repaired on the next start: rows stuck in `downloading` go back to `queued`, and yt-dlp continues those partial files. `docker stop` is a clean shutdown. Avoid `kill -9` while a merge is in progress.

The temp volume and the downloads volume are separate, so the server copies the finished file instead of renaming across mounts.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `ADMIN_KEY` | required | Admin API key. Use at least 32 random characters |
| `ADDR` | `:6543` | Listen address. `127.0.0.1:6543` keeps it on this machine |
| `DATA_DIR` | `data` | Root for the database, downloads, and temp files |
| `DB_PATH` | `$DATA_DIR/db/youtubegir.sqlite` | SQLite file |
| `DOWNLOAD_DIR` | `$DATA_DIR/downloads` | Finished files |
| `TEMP_DIR` | `$DATA_DIR/tmp` | Partial files used for resume |
| `MAX_CONCURRENT` | `2` | Parallel yt-dlp processes, capped at 32 |
| `YTDLP_PATH` | `yt-dlp` | Binary |
| `FFMPEG_PATH` | discovered on `PATH` | Used for merging and audio extract |
| `JS_RUNTIME` | `deno` or `node` if found | Passed to yt-dlp. `none` disables it |
| `COOKIES_FILE` | empty | Optional Netscape cookies for videos your account can open |
| `DOWNLOAD_PROXY` | empty | First-run SOCKS or HTTP proxy, for example `socks5://127.0.0.1:1080`. After a proxy is saved, the admin setting wins |

Age-restricted or member videos need a cookies file exported from a browser where you are signed in. Mount it read-only and set `COOKIES_FILE`. Do not commit that file.

An admin can set a download proxy from the extension settings, on the Admin tab, or with:

```bash
curl -s -X PUT http://127.0.0.1:6543/api/v1/system/proxy \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"proxy":"socks5://127.0.0.1:1080"}'
```

yt-dlp receives `--proxy` only while it reads formats or downloads a video. The API, file downloads, version check, and `yt-dlp -U` stay direct. Send `{"proxy":""}` to clear it. A URL that contains a password is visible in the process list on the server. `DOWNLOAD_PROXY` is copied into the database only when no proxy has been saved yet.

Update yt-dlp from the admin key:

```bash
curl -s -X POST http://127.0.0.1:6543/api/v1/system/ytdlp/update \
  -H "Authorization: Bearer $ADMIN_KEY"
```

In Docker the binary lives in `/usr/local/bin`, so `-U` can replace it until the container is recreated. Rebuild the image when you want a pinned toolchain again.

## Security

- The admin key can see and delete every download. Keep it out of the extension unless that browser is the admin’s.
- File tokens are unguessable and are enough to download a finished file. Do not paste them into a public chat.
- A server on `:6543` is reachable from other machines on the network. For a laptop, set `ADDR=127.0.0.1:6543`. For a remote server, put HTTPS in front of it (Caddy or nginx) and do not publish the raw port to the internet.
- The Docker image runs as root so the named volumes stay writable. Do not expose it beyond the people who should have keys.

## Development

```bash
make test
make build
```

Tests cover URL checks, the progress parser, the queue (pause keeps the partial file, cancel removes it), user isolation, and the file token download. They do not call YouTube.

The visual system for the extension is written down in [design.md](design.md).

## Command-line tool

`install.sh`, `bin/youtube-gir`, and `make`’s server binary are different programs. The shell tool installs as `youtube-gir` / `youtube-git` and is documented in [docs/cli.fa.md](docs/cli.fa.md). `make build` writes the API server to `dist/youtubegir`.
