-- Attic initial schema.
--
-- Forward-only: never edit this file once applied anywhere, add a new
-- migration instead.

-- ---------------------------------------------------------------- accounts --

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username      text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role          text NOT NULL CHECK (role IN ('admin', 'member', 'kid')),
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- One row per signed-in device. The refresh token is stored only as a
-- SHA-256 hash, so a database leak cannot be replayed as a session.
CREATE TABLE devices (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name               text NOT NULL,
    refresh_token_hash bytea NOT NULL UNIQUE,
    expires_at         timestamptz NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    last_seen          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX devices_user_id_idx ON devices (user_id);

-- ------------------------------------------------------------- media files --

-- Every file Attic knows about, photo, video or audio. Content-addressed by
-- sha256 (lowercase hex); `path` is absolute and lives under a library root.
CREATE TABLE media_files (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    path        text NOT NULL UNIQUE,
    size_bytes  bigint NOT NULL,
    sha256      text NOT NULL,
    mtime       timestamptz NOT NULL,
    probe       jsonb,
    verified_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX media_files_sha256_idx ON media_files (sha256);

-- ------------------------------------------------------------------ photos --

CREATE TABLE assets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    file_id     uuid NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('photo', 'video')),
    taken_at    timestamptz,
    width       integer,
    height      integer,
    gps         point,
    camera      text,
    is_favorite boolean NOT NULL DEFAULT false,
    deleted_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX assets_owner_taken_at_idx ON assets (owner_id, taken_at DESC);

CREATE TABLE albums (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title          text NOT NULL,
    cover_asset_id uuid REFERENCES assets (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX albums_owner_id_idx ON albums (owner_id);

CREATE TABLE album_assets (
    album_id uuid NOT NULL REFERENCES albums (id) ON DELETE CASCADE,
    asset_id uuid NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    position integer NOT NULL,
    PRIMARY KEY (album_id, asset_id)
);

-- ------------------------------------------------------------------- video --

CREATE TABLE series (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title       text NOT NULL,
    tmdb_id     integer,
    poster_path text,
    overview    text,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE video_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id     uuid NOT NULL UNIQUE REFERENCES media_files (id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('movie', 'episode')),
    title       text NOT NULL,
    series_id   uuid REFERENCES series (id) ON DELETE SET NULL,
    season      integer,
    episode     integer,
    year        integer,
    tmdb_id     integer,
    overview    text,
    poster_path text,
    runtime_s   integer,
    rating      double precision,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX video_items_series_idx ON video_items (series_id, season, episode);

CREATE TABLE watch_progress (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_id    uuid NOT NULL REFERENCES video_items (id) ON DELETE CASCADE,
    position_s integer NOT NULL DEFAULT 0,
    completed  boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, item_id)
);

-- ------------------------------------------------------------------- music --

CREATE TABLE tracks (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id        uuid NOT NULL UNIQUE REFERENCES media_files (id) ON DELETE CASCADE,
    title          text NOT NULL,
    artist         text NOT NULL DEFAULT '',
    album          text NOT NULL DEFAULT '',
    album_artist   text NOT NULL DEFAULT '',
    track_no       integer,
    disc_no        integer,
    year           integer,
    genre          text,
    duration_s     double precision,
    musicbrainz_id text,
    -- sha256 of the embedded cover art, if any. The image itself is a
    -- regenerable file at /data/derived/covers/<cover_hash>.jpg.
    cover_hash     text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    -- Full-text search over title, album and artists. 'simple' rather than a
    -- language configuration: music metadata is multilingual and stemming
    -- English rules over it does more harm than good.
    search_vector  tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(album, '')), 'B') ||
        setweight(to_tsvector('simple', coalesce(album_artist, '') || ' ' || coalesce(artist, '')), 'C')
    ) STORED
);

CREATE INDEX tracks_browse_idx ON tracks (album_artist, album, disc_no, track_no);
CREATE INDEX tracks_search_idx ON tracks USING GIN (search_vector);
CREATE INDEX tracks_album_idx ON tracks (album);

CREATE TABLE playlists (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX playlists_owner_id_idx ON playlists (owner_id);

-- Position is part of the primary key rather than track_id, so the same track
-- may appear in a playlist more than once.
CREATE TABLE playlist_tracks (
    playlist_id uuid NOT NULL REFERENCES playlists (id) ON DELETE CASCADE,
    track_id    uuid NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    position    integer NOT NULL,
    PRIMARY KEY (playlist_id, position)
);

CREATE INDEX playlist_tracks_track_id_idx ON playlist_tracks (track_id);
