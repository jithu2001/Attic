# Migrations

Forward-only, additive SQL migrations applied by golang-migrate at startup
(embedded in the binary, so the image is self-contained).

Rules:

1. Never edit a migration that has been applied anywhere. Add a new one.
2. Files are named `NNNNNN_description.up.sql` / `.down.sql`, numbered
   sequentially from `000001`.
3. Down migrations exist for local development only; production rolls forward.

Phase 1 adds the initial schema (`users`, `devices`, `media_files`, `assets`,
`albums`, `series`, `video_items`, `watch_progress`, `tracks`, `playlists`).
River manages its own tables.
