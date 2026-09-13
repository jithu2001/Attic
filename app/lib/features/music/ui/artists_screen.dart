import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/layout/window_size.dart';
import '../../../core/widgets/async_view.dart';
import '../data/models.dart';
import '../music_providers.dart';
import 'search_field.dart';

/// The music library's front door: every album artist, alphabetically.
class ArtistsScreen extends ConsumerWidget {
  const ArtistsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final artists = ref.watch(artistsProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Music'),
        actions: <Widget>[
          IconButton(
            tooltip: 'Playlists',
            icon: const Icon(Icons.queue_music_outlined),
            onPressed: () => context.go('/music/playlists'),
          ),
        ],
        bottom: const PreferredSize(
          preferredSize: Size.fromHeight(72),
          child: Padding(
            padding: EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: MusicSearchField(),
          ),
        ),
      ),
      body: RefreshIndicator(
        onRefresh: () async => ref.invalidate(artistsProvider),
        child: AsyncView<List<Artist>>(
          value: artists,
          onRetry: () => ref.invalidate(artistsProvider),
          isEmpty: (data) => data.isEmpty,
          emptyIcon: Icons.library_music_outlined,
          emptyTitle: 'No music yet',
          emptyMessage: 'Put some files in your music folder — Attic picks them '
              'up within a minute.',
          builder: (data) => _ArtistList(artists: data),
        ),
      ),
    );
  }
}

class _ArtistList extends StatelessWidget {
  const _ArtistList({required this.artists});

  final List<Artist> artists;

  @override
  Widget build(BuildContext context) {
    final margin = WindowSizeClass.of(context).screenMargin - 16;

    return ListView.builder(
      padding: EdgeInsets.fromLTRB(margin, 8, margin, 96),
      itemCount: artists.length,
      itemBuilder: (context, index) {
        final artist = artists[index];
        return ListTile(
          leading: CircleAvatar(
            child: Text(artist.initial),
          ),
          title: Text(artist.name, maxLines: 1, overflow: TextOverflow.ellipsis),
          subtitle: Text(_summary(artist)),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => context.go('/music/artists/${artist.id}'),
        );
      },
    );
  }

  static String _summary(Artist artist) {
    final albums = '${artist.albumCount} ${artist.albumCount == 1 ? 'album' : 'albums'}';
    final tracks = '${artist.trackCount} ${artist.trackCount == 1 ? 'track' : 'tracks'}';
    return '$albums · $tracks';
  }
}
