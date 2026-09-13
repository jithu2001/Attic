# Attic

Self-hosted personal media: photo backup, music streaming and a Netflix-style
video library, for your own hardware and your own network.

Two parts:

- **`server/`** — one Go binary (API, scanner, background jobs, streaming),
  shipped as a single Docker image alongside Postgres and Caddy.
- **`app/`** — one Flutter codebase for Android phone, iOS and Android TV.
  Material 3 throughout.

There is no web frontend and no public domain. The server is reached over a
Tailscale network, with TLS from `tailscale cert` or plain HTTP inside the
tailnet.

See [PROGRESS.md](PROGRESS.md) for what currently works, and run `make help`
for the available commands.
