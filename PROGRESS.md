# Attic — Progress

Self-hosted photos, music and video. One Go server binary, one Flutter app
(Android phone, iOS, Android TV). Reachable only over Tailscale.

**Current state:** skeleton (pre-phase-1). Server and app both build, run and
test green.

## What works

### Server
- `config` package loads every setting from the environment with defaults,
  validates the JWT secret and library roots, and builds the slog logger.
- chi router with request-id, real-ip, structured request logging, panic
  recovery, 60 s timeout and Prometheus instrumentation.
- `GET /healthz` — liveness JSON (status, version, uptime). Does not touch the
  database on purpose, so a DB blip cannot get the container killed.
- `GET /metrics` — Prometheus, with `attic_http_requests_total`,
  `attic_http_request_duration_seconds`, `attic_http_requests_in_flight` plus
  Go and process collectors. Route labels use chi patterns, not raw paths.
- `GET /api/v1/ping` — the address-validation endpoint the app's connect screen
  calls. Shipped, so it is frozen: additive changes only.
- Canonical error envelope `{"error": {"code", "message"}}` on 404, 405 and
  every handler.
- Graceful shutdown on SIGINT/SIGTERM with a 20 s drain.
- `attic -healthcheck` self-probe, used by the image HEALTHCHECK.
- Dockerfile (distroless-ish Debian runtime with FFmpeg + Intel QSV/VAAPI
  drivers and libvips), docker-compose with app + postgres:16 + caddy, Caddyfile
  serving the `tailscale cert` key pair.

### App
- Material 3 everywhere: `useMaterial3: true`, dynamic colour on Android 12+ via
  `DynamicColorBuilder` with a seeded (`0xFF6750A4`) fallback, full light and
  dark themes, `themeMode: ThemeMode.system` overridable in Settings.
- Adaptive shell on the M3 window size classes: `NavigationBar` under 600 dp,
  `NavigationRail` at 600 dp and above (which is also the Android TV layout).
- Four top-level destinations — Photos, Music, Video, Settings — wired through
  go_router's `ShellRoute`, so the bar/rail survives destination switches.
- Settings is real, not a placeholder: theme override via `SegmentedButton`,
  server-address entry point, about dialog.
- `ApiClient` (dio) knows the `/api/v1` base path, normalises a bare Tailscale
  hostname into an https URL, and parses the server's error envelope.
- Android manifest declares leanback + non-required touchscreen and a
  `LEANBACK_LAUNCHER` intent filter, so one APK covers phone and TV.

## What is flagged off

All flags live in `app/lib/core/flags.dart` and default to `false`. Turn one on
for local work with `--dart-define=ATTIC_<NAME>=true`.

| Flag | Covers | State |
| --- | --- | --- |
| `ATTIC_AUTH` | Connect + sign-in screens | Screens built; `connect` really validates the address against `/api/v1/ping`, `signIn` is a stub that reports "arrives with the authentication phase". Routes only exist when the flag is on. |
| `ATTIC_PHOTOS` | Photo timeline, albums, viewer | Placeholder screen only |
| `ATTIC_MUSIC` | Music library and player | Placeholder screen only |
| `ATTIC_VIDEO` | Movies and series | Placeholder screen only |
| `ATTIC_BACKGROUND_SYNC` | Camera-roll backup | Not started |

## Not started

- Migrations: `server/migrations/` holds only its README. The initial schema
  (`users`, `devices`, `media_files`, `assets`, `albums`, `album_assets`,
  `series`, `video_items`, `watch_progress`, `tracks`, `playlists`,
  `playlist_tracks`) lands in phase 1, together with golang-migrate running
  embedded on startup.
- No database connection yet: `ATTIC_DATABASE_URL` is loaded and validated but
  nothing dials Postgres, so the server starts without one.
- `internal/auth`, `store`, `scanner`, `jobs`, `stream`, `ffmpeg` are
  doc-comment-only packages.
- River job queue, govips thumbnails, dhowden/tag, ffprobe wrappers.
- App: drift local DB, media_kit, just_audio/audio_service, photo_manager,
  workmanager, flutter_secure_storage usage (the dependency is present, no
  tokens are stored yet).

## Known issues / deviations

- **`cmd/Attic` is capitalised** to match the repository layout in the brief,
  against the usual Go convention of a lowercase command directory. The built
  binary is `attic`.
- **Dependencies not yet added to `pubspec.yaml`:** `media_kit`, `just_audio`,
  `audio_service`, `photo_manager`, `workmanager`, `drift`. They are part of the
  fixed stack and will be added by the phase that first uses them, so a broken
  resolution can never block a phase that does not need them.
- **`/api/v1/ping` is an addition** to the brief's API surface. The connect
  screen needs an unauthenticated endpoint to validate a server address before
  any credentials exist.
- Caddy expects `server/certs/attic.crt` and `attic.key` from
  `tailscale cert`; `make up` does not create them.
- No CI yet.

## Running it

```sh
cp server/.env.example server/.env   # then set ATTIC_JWT_SECRET (openssl rand -hex 32)
make up                              # app + postgres + caddy
curl http://127.0.0.1:8080/healthz

make server-test                     # go test -race ./...
make app-test                        # flutter test
```
