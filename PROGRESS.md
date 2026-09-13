# Attic — Progress

Self-hosted photos, music and video. One Go server binary, one Flutter app
(Android phone, iOS, Android TV). Reachable only over Tailscale.

**Current state:** end of phase 1. Attic is a usable self-hosted **music
streamer**: create accounts, sign in from a phone, browse a real library, and
play it with background playback and lock-screen controls. Photos and Video are
flagged-off placeholders showing an M3 empty state.

## What works

### Server

**Schema and migrations.** `migrations/000001_init` creates the full
master-prompt schema — `users`, `devices`, `media_files`, `assets`, `albums`,
`album_assets`, `series`, `video_items`, `watch_progress`, `tracks`,
`playlists`, `playlist_tracks` — and runs automatically at startup (embedded
golang-migrate). River creates its own tables on the same pass. The server
retries for 60 s if Postgres is still coming up, so `docker compose up` on a
cold machine works without an ordering dance.

**Accounts.** `attic adduser <username>` prompts for a password with echo off
(or `--password-stdin` when scripted), applies migrations if needed, and makes
the first account an admin. There is no open registration. Passwords are
argon2id at t=3, m=64 MB, p=2, stored PHC-encoded; a successful login
transparently upgrades a hash made with weaker parameters.

**Auth.**
- `POST /api/v1/auth/login` → 15-minute access JWT + an opaque refresh token,
  recorded against a named device. Unknown usernames burn the same time as
  known ones, so response timing does not enumerate accounts.
- `POST /api/v1/auth/refresh` rotates the refresh token in a single conditional
  UPDATE, so a replay of a consumed token fails and two concurrent refreshes
  cannot both win.
- `POST /api/v1/auth/logout` revokes that device only; revoking an
  already-dead session is a success.
- `POST /api/v1/auth/media-token` mints a 6-hour token for `?token=` URLs.
  Media tokens are rejected everywhere except media handlers, and a query-string
  token is never accepted for anything else.
- JWT middleware pins HS256 (an `alg: none` token is refused) and role
  middleware gates `/admin/*`.

**Music scanner.** Walks `ATTIC_MUSIC_DIR`, diffs against `media_files` by path
+ size + mtime (compared at microsecond resolution, because Postgres keeps
microseconds and ext4 reports nanoseconds — otherwise every scan would re-hash
the library), streams SHA-256 over new files, reads tags with dhowden/tag,
extracts embedded cover art to `/data/derived/covers/<hash>.<ext>`, and upserts
`tracks`. Runs as River jobs: one `music_scan_library` job fans out into one
`music_scan_dir` job per directory, so a restart loses at most one directory,
and a prune pass removes rows for files that are gone. One unreadable file is
logged and skipped rather than failing its directory.

Triggers: `POST /api/v1/admin/scan` (admin, returns 202), an hourly schedule
that also runs at startup, and an fsnotify watcher with a 30-second debounce.

**Music API.** `GET /music/artists` (grouped by `album_artist`, with album and
track counts), `/music/artists/{id}/albums`, `/music/albums/{id}` (tracks in
disc then track order), `GET /search?q=` (tsvector over title/album/artists,
rolled up into mixed artist, album and track hits), and playlist CRUD with
`PUT /playlists/{id}/tracks` replacing the whole ordered list.

**Streaming.** `GET /music/tracks/{id}/audio` serves originals through
`http.ServeContent`, so Range requests get 206 with a correct `Content-Range` —
that is what makes in-track seeking instant. `?transcode=opus128` (also
`opus96`, `opus64`) pipes one FFmpeg process straight to the response, no
session machinery. `GET /covers/{hash}` is `public, max-age=31536000,
immutable`. Every media path is resolved through a guard that follows symlinks
before checking them against the configured library roots.

**Operations.** `/healthz`, Prometheus `/metrics` (route-pattern labels), the
canonical error envelope everywhere, structured request logs that include
`range` and `content_range` so 206s are visible, graceful shutdown, and
`attic -healthcheck` for the container HEALTHCHECK.

### App

**First run.** Server address screen (M3 `TextField` with helper text naming
the tailnet hostname, `FilledButton` "Connect" that probes `/healthz` and
rejects an address that answers but is not Attic), then sign-in. Errors surface
as `SnackBar`s. Tokens live in `flutter_secure_storage`; the server address is
kept separately, so signing out does not mean retyping a hostname.

**Session handling.** A `QueuedInterceptor` attaches the bearer token and, on a
401, refreshes once and replays the request. Queued rather than parallel on
purpose: refresh tokens rotate on use, so ten concurrent requests on a stale
token must produce one refresh, not ten. A refresh that fails because the
server said no signs the user out; a refresh that fails because the tailnet is
unreachable keeps the session.

**Music.** Artists list (`ListTile` + initial `CircleAvatar`) → album grid
(`Card.filled`, two columns compact, four expanded) → album detail (large cover
header, tracks with number and duration, three-dot `ModalBottomSheet` offering
play next, add to playlist, go to artist). `SearchAnchor.bar` in the app bar,
with results grouped under titleSmall headers by kind.

**Playback.** just_audio behind audio_service: a persistent gapless queue,
background playback, lock-screen and notification controls with artwork, and
audio-focus handling via audio_session. A mini-player is docked above the
`NavigationBar` (and below the content on wide layouts) whenever something is
playing, and takes zero height when nothing is. Tapping it opens the full
player: large art, seek `Slider`, `IconButton.filledTonal` transport, shuffle
and repeat toggles, queue in a bottom sheet.

**Playlists.** List, create via FAB → M3 dialog, `ReorderableListView` for
drag-to-reorder, swipe to remove with a "Deleted. Undo" SnackBar, rename and
delete with a confirmation dialog.

**Settings.** Theme `SegmentedButton` (System/Light/Dark), account and server
info, read-only device list with relative "last seen", sign out behind a
confirmation dialog.

**Platform.** Android declares the media-playback foreground service, the media
button receiver and `MainActivity : AudioServiceActivity`; iOS declares the
`audio` background mode. The Android manifest also covers TV (leanback,
non-required touchscreen, `LEANBACK_LAUNCHER`).

## What is flagged off

Flags live in `app/lib/core/flags.dart`. A flag flips on in the phase that
finishes its feature; anything unfinished stays off.

| Flag | Covers | State |
| --- | --- | --- |
| `ATTIC_AUTH` | Connect, sign-in, session refresh | **On** — shipped in phase 1 |
| `ATTIC_MUSIC` | Music library, search, playlists, playback | **On** — shipped in phase 1 |
| `ATTIC_PHOTOS` | Photo timeline, albums, viewer | Off — M3 empty state, "Coming soon" |
| `ATTIC_VIDEO` | Movies and series | Off — M3 empty state, "Coming soon" |
| `ATTIC_BACKGROUND_SYNC` | Camera-roll backup | Off — not started |

## Verified on a real device

Tested end to end on a Galaxy S20 FE (Android 13) against the running server,
with a generated library of real MP3 and FLAC files:

- Connect by address → sign in → browse artists → albums → album detail → play.
- **MP3 and FLAC both play.** Album art extracted from the files renders in the
  grid, the album header, the mini-player and the lock screen.
- **Playback survives backgrounding and a locked, dozing screen**: a 7-minute
  track ran from 0:03 to past 2:13 with the app in the background and the
  screen off.
- **Lock-screen controls work**, with artwork, title, artist and a live
  scrubber. Media buttons (the path a headset uses) pause, resume and skip.
- A media-playback foreground service and a transport-category notification are
  posted on Android 13.
- The filesystem watcher picked up a newly copied album within the debounce
  window while the app was running; it appeared on the next browse.
- Recreating the database invalidated the stored session: the app tried `/me`,
  got 401, refreshed, got 401, signed out and returned to the login screen with
  the server address remembered.
- ExoPlayer issued a real range request
  (`bytes=3214-` → `206 bytes 3214-484075/484076`).
- Zero server-side errors across the whole session.

Not verified on-device: **audio-focus interruption** (another app taking focus)
— it is configured through audio_session and just_audio's interruption
handling, but was not exercised against a second audio app. Seeking far enough
ahead to force a fresh range request was also not driven from the UI; the 206
behaviour is covered by tests and by the curl run below.

## How it was verified

The server was run for real against Postgres with a generated library, not just
unit-tested:

- `attic adduser` created an admin and a kid account; the first account became
  an admin automatically.
- The startup scan ingested 11 tracks across 3 albums as River jobs; browse,
  search, covers and playlists all returned correct data.
- Range requests returned 206 with correct `Content-Range` for opening, middle,
  open-ended and suffix ranges, and 416 for an unsatisfiable one. The 206s are
  visible in the server log.
- Dropping a new 3-track album into the library made it appear in the API about
  4 seconds later, with no restart; deleting a file removed it on the next scan.
- `kill -9` mid-stream, then restart: the library, playlists and sessions were
  all intact and playback resumed on the next request.
- Refresh rotated the token and invalidated the old one (replay → 401); logout
  revoked one device and left the other signed in; a kid account got 403 on
  `POST /admin/scan` while the admin got 202.
- Store integration tests run the real SQL — migrations, the generated tsvector
  column, browse ordering, playlist ordering — against a live Postgres.

## Known issues / deviations

- **The phase 1 prompt called the project "Haven"** (`haven adduser`). The
  master prompt, the repository, the module path and the binary are all
  **Attic**, so the name was kept. Nothing else about the prompt was changed.
- **`cmd/Attic` is capitalised** to match the repository layout in the brief,
  against the usual Go convention. The built binary is `attic`.
- **Covers are stored as `<hash>.<ext>`, not always `<hash>.jpg`.** The brief
  says `.jpg`; writing a PNG under a `.jpg` name would be a lie, so the source
  format is preserved and the cover handler probes the known extensions. The
  API path (`/api/v1/covers/{hash}`) is unchanged.
- **Track durations need ffprobe.** The Docker image ships FFmpeg, so this is
  only a gap when running the binary on a host without it — the library still
  scans and plays, but `duration_s` is null and the scrubber falls back to the
  decoder's own duration once playback starts. The local end-to-end run above
  device test above ran with a real FFmpeg and durations were read correctly.
  The transcode path (`?transcode=opus128`) is implemented and unit-covered but
  has **not** been exercised against a real encoder.
- **Artist and album ids are derived, not stored** (URL-safe base64 of the
  album-artist name, and of the artist + album pair). Artists and albums have
  no rows of their own, exactly as the schema describes, and derived ids stay
  stable across a rescan that rewrites every track row.
- **`/api/v1/ping` and `POST /api/v1/auth/media-token` are additions** to the
  brief's API surface. The first validates a server address before credentials
  exist; the second exists because a platform media stack cannot refresh an
  `Authorization` header mid-stream.
- **`search` results carry `album_id`** so a track hit can open in its album.
- **Music browse endpoints are unpaginated.** Artists, an artist's albums and an
  album's tracks are all naturally bounded; search is capped at 20 (50 max).
  The unbounded endpoints — the photo and video timelines — will use the cursor
  pagination the conventions call for.
- **Integration tests were run against Postgres 18** (what is installed
  locally); production compose runs postgres:16. Nothing in the schema is
  version-specific.
- **FLAC track and disc numbers needed a fix.** dhowden/tag reads only the
  canonical `tracknumber` field and parses it with `Atoi`, so the very common
  `TRACKNUMBER=3/12` form — and ffmpeg's non-canonical `track` key — both came
  back as zero and the album silently sorted by title. The scanner now falls
  back to the raw tags and accepts the "of total" form. Found on the phone,
  fixed, and covered by a table-driven test.
- **Android permits cleartext HTTP** via a network security config. The brief
  supports plain HTTP inside a tailnet, and Android blocks it by default, which
  would make those servers unreachable. HTTPS is still used whenever the
  address has a scheme, and a bare hostname is upgraded to https first.
- **The media notification needs `POST_NOTIFICATIONS` on Android 13+.** The
  permission is declared, but the app never *asks* for it at runtime, so on a
  fresh install the lock-screen controls will not appear until the user grants
  notifications by hand. Requesting it needs a package outside the fixed stack
  (`permission_handler`), so it is left for a decision rather than added
  silently.
- **The library has been tested with ~14 tracks, not ≥1k.** The scan path is
  per-directory and bounded, and the diff avoids re-hashing unchanged files, but
  the ≥1k-track check in the phase brief needs a real library to confirm.
  (Real-FLAC and real-MP3 playback *were* confirmed on the device above.)
- Caddy expects `server/certs/attic.crt` and `attic.key` from `tailscale cert`;
  `make up` does not create them.
- No CI yet.

## Running it

```sh
cp server/.env.example server/.env   # set ATTIC_JWT_SECRET and ATTIC_MUSIC_HOST_DIR
make up                              # app + postgres + caddy
make adduser USER=ada                # create the first (admin) account

curl http://127.0.0.1:8080/healthz

make server-test                     # go test -race ./...
make app-test                        # flutter test
make app-analyze                     # flutter analyze

# Store integration tests need a throwaway database:
ATTIC_TEST_DATABASE_URL=postgres://attic@localhost:5432/attic_test?sslmode=disable \
  make server-test-integration
```
