import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';
import 'data/models.dart';

/// Library reads. Each is a FutureProvider so a screen gets loading, error and
/// data states for free, and `ref.invalidate` is all a pull-to-refresh needs.

final artistsProvider = FutureProvider.autoDispose<List<Artist>>((ref) {
  return ref.watch(musicRepositoryProvider).artists();
});

final artistAlbumsProvider =
    FutureProvider.autoDispose.family<List<Album>, String>((ref, artistId) {
  return ref.watch(musicRepositoryProvider).albumsByArtist(artistId);
});

final albumProvider =
    FutureProvider.autoDispose.family<AlbumDetail, String>((ref, albumId) {
  return ref.watch(musicRepositoryProvider).album(albumId);
});

final searchProvider =
    FutureProvider.autoDispose.family<List<SearchResult>, String>((ref, query) {
  if (query.trim().isEmpty) return Future.value(const <SearchResult>[]);
  return ref.watch(musicRepositoryProvider).search(query);
});

final playlistsProvider = FutureProvider.autoDispose<List<Playlist>>((ref) {
  return ref.watch(musicRepositoryProvider).playlists();
});

final playlistProvider =
    FutureProvider.autoDispose.family<PlaylistDetail, String>((ref, id) {
  return ref.watch(musicRepositoryProvider).playlist(id);
});
