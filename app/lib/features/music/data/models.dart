/// The music API's JSON shapes, as immutable Dart values.
///
/// Nothing here talks to dio: the models are plain data, which is what lets
/// widget tests build a library without a server.
library;

import 'package:flutter/widgets.dart' show Characters;

class Artist {
  const Artist({
    required this.id,
    required this.name,
    required this.albumCount,
    required this.trackCount,
    this.coverUrl,
  });

  final String id;
  final String name;
  final int albumCount;
  final int trackCount;
  final String? coverUrl;

  /// The letter shown in the list's leading avatar.
  String get initial =>
      name.trim().isEmpty ? '?' : Characters(name.trim()).first.toUpperCase();

  factory Artist.fromJson(Map<String, dynamic> json) => Artist(
        id: json['id'] as String,
        name: json['name'] as String? ?? '',
        albumCount: json['album_count'] as int? ?? 0,
        trackCount: json['track_count'] as int? ?? 0,
        coverUrl: json['cover_url'] as String?,
      );
}

class Album {
  const Album({
    required this.id,
    required this.name,
    required this.albumArtist,
    required this.artistId,
    required this.trackCount,
    required this.durationS,
    this.year,
    this.coverUrl,
  });

  final String id;
  final String name;
  final String albumArtist;
  final String artistId;
  final int trackCount;
  final double durationS;
  final int? year;
  final String? coverUrl;

  factory Album.fromJson(Map<String, dynamic> json) => Album(
        id: json['id'] as String,
        name: json['name'] as String? ?? '',
        albumArtist: json['album_artist'] as String? ?? '',
        artistId: json['artist_id'] as String? ?? '',
        trackCount: json['track_count'] as int? ?? 0,
        durationS: (json['duration_s'] as num?)?.toDouble() ?? 0,
        year: json['year'] as int?,
        coverUrl: json['cover_url'] as String?,
      );
}

class AlbumDetail {
  const AlbumDetail({required this.album, required this.tracks});

  final Album album;
  final List<Track> tracks;

  factory AlbumDetail.fromJson(Map<String, dynamic> json) => AlbumDetail(
        album: Album.fromJson(json),
        tracks: (json['tracks'] as List<dynamic>? ?? const [])
            .map((t) => Track.fromJson(t as Map<String, dynamic>))
            .toList(growable: false),
      );
}

class Track {
  const Track({
    required this.id,
    required this.title,
    required this.artist,
    required this.album,
    required this.albumArtist,
    required this.albumId,
    required this.artistId,
    required this.audioUrl,
    this.trackNo,
    this.discNo,
    this.year,
    this.durationS,
    this.coverUrl,
  });

  final String id;
  final String title;
  final String artist;
  final String album;
  final String albumArtist;
  final String albumId;
  final String artistId;

  /// Server-relative, e.g. `/api/v1/music/tracks/<id>/audio`.
  final String audioUrl;

  final int? trackNo;
  final int? discNo;
  final int? year;
  final double? durationS;
  final String? coverUrl;

  Duration? get duration =>
      durationS == null ? null : Duration(milliseconds: (durationS! * 1000).round());

  /// Who to credit on screen: the track artist when it differs from the album
  /// artist (compilations, features), otherwise the album artist.
  String get displayArtist => artist.isEmpty ? albumArtist : artist;

  factory Track.fromJson(Map<String, dynamic> json) => Track(
        id: json['id'] as String,
        title: json['title'] as String? ?? '',
        artist: json['artist'] as String? ?? '',
        album: json['album'] as String? ?? '',
        albumArtist: json['album_artist'] as String? ?? '',
        albumId: json['album_id'] as String? ?? '',
        artistId: json['artist_id'] as String? ?? '',
        audioUrl: json['audio_url'] as String? ?? '',
        trackNo: json['track_no'] as int?,
        discNo: json['disc_no'] as int?,
        year: json['year'] as int?,
        durationS: (json['duration_s'] as num?)?.toDouble(),
        coverUrl: json['cover_url'] as String?,
      );
}

enum SearchResultKind { artist, album, track }

class SearchResult {
  const SearchResult({
    required this.kind,
    required this.id,
    required this.name,
    required this.subtitle,
    this.albumId = '',
    this.coverUrl,
    this.durationS,
  });

  final SearchResultKind kind;
  final String id;
  final String name;
  final String subtitle;

  /// For a track hit, the album it belongs to, so it can be opened in context.
  final String albumId;
  final String? coverUrl;
  final double? durationS;

  factory SearchResult.fromJson(Map<String, dynamic> json) => SearchResult(
        kind: switch (json['kind'] as String?) {
          'artist' => SearchResultKind.artist,
          'album' => SearchResultKind.album,
          _ => SearchResultKind.track,
        },
        id: json['id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        subtitle: json['subtitle'] as String? ?? '',
        albumId: json['album_id'] as String? ?? '',
        coverUrl: json['cover_url'] as String?,
        durationS: (json['duration_s'] as num?)?.toDouble(),
      );
}

class Playlist {
  const Playlist({
    required this.id,
    required this.name,
    required this.trackCount,
  });

  final String id;
  final String name;
  final int trackCount;

  factory Playlist.fromJson(Map<String, dynamic> json) => Playlist(
        id: json['id'] as String,
        name: json['name'] as String? ?? '',
        trackCount: json['track_count'] as int? ?? 0,
      );
}

class PlaylistDetail {
  const PlaylistDetail({required this.playlist, required this.tracks});

  final Playlist playlist;
  final List<Track> tracks;

  factory PlaylistDetail.fromJson(Map<String, dynamic> json) => PlaylistDetail(
        playlist: Playlist.fromJson(json),
        tracks: (json['tracks'] as List<dynamic>? ?? const [])
            .map((t) => Track.fromJson(t as Map<String, dynamic>))
            .toList(growable: false),
      );
}

/// Formats a duration the way a track list does: 4:07, or 1:02:33 when it runs
/// past an hour.
String formatDuration(Duration? duration) {
  if (duration == null) return '--:--';
  final seconds = duration.inSeconds;
  final minutes = seconds ~/ 60;
  final hours = minutes ~/ 60;
  String twoDigits(int n) => n.toString().padLeft(2, '0');

  if (hours > 0) {
    return '$hours:${twoDigits(minutes % 60)}:${twoDigits(seconds % 60)}';
  }
  return '$minutes:${twoDigits(seconds % 60)}';
}
