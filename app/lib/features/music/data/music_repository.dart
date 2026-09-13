import '../../../core/api/api_client.dart';
import 'models.dart';

/// Reads the music library from the server.
///
/// It depends on [ApiClient] rather than dio directly, which is what lets the
/// widget tests hand it a client backed by a canned adapter.
class MusicRepository {
  const MusicRepository(this._client);

  final ApiClient _client;

  Future<List<Artist>> artists() async {
    final json = await _client.getJson('/music/artists');
    return (json['artists'] as List<dynamic>? ?? const [])
        .map((a) => Artist.fromJson(a as Map<String, dynamic>))
        .toList(growable: false);
  }

  Future<List<Album>> albumsByArtist(String artistId) async {
    final json = await _client.getJson('/music/artists/$artistId/albums');
    return (json['albums'] as List<dynamic>? ?? const [])
        .map((a) => Album.fromJson(a as Map<String, dynamic>))
        .toList(growable: false);
  }

  Future<AlbumDetail> album(String albumId) async =>
      AlbumDetail.fromJson(await _client.getJson('/music/albums/$albumId'));

  Future<List<SearchResult>> search(String query) async {
    final json = await _client.getJson('/search', query: {'q': query});
    return (json['results'] as List<dynamic>? ?? const [])
        .map((r) => SearchResult.fromJson(r as Map<String, dynamic>))
        .toList(growable: false);
  }

  // ------------------------------------------------------------ playlists --

  Future<List<Playlist>> playlists() async {
    final json = await _client.getJson('/playlists');
    return (json['playlists'] as List<dynamic>? ?? const [])
        .map((p) => Playlist.fromJson(p as Map<String, dynamic>))
        .toList(growable: false);
  }

  Future<PlaylistDetail> playlist(String id) async =>
      PlaylistDetail.fromJson(await _client.getJson('/playlists/$id'));

  Future<Playlist> createPlaylist(String name) async =>
      Playlist.fromJson(await _client.postJson('/playlists', body: {'name': name}));

  Future<Playlist> renamePlaylist(String id, String name) async =>
      Playlist.fromJson(await _client.putJson('/playlists/$id', body: {'name': name}));

  Future<void> deletePlaylist(String id) => _client.delete('/playlists/$id');

  /// Replaces a playlist's contents. The whole ordered list goes up at once, so
  /// a drag-to-reorder is one atomic request rather than a diff.
  Future<PlaylistDetail> setPlaylistTracks(String id, List<String> trackIds) async {
    return PlaylistDetail.fromJson(
      await _client.putJson('/playlists/$id/tracks', body: {'track_ids': trackIds}),
    );
  }
}
