import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/layout/window_size.dart';
import '../../../core/widgets/async_view.dart';
import '../data/models.dart';
import '../music_providers.dart';
import '../player/player_controller.dart';
import 'cover_image.dart';
import 'track_actions.dart';

/// One album: a large cover header, then its tracks in play order.
class AlbumDetailScreen extends ConsumerWidget {
  const AlbumDetailScreen({super.key, required this.albumId});

  final String albumId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final album = ref.watch(albumProvider(albumId));

    return Scaffold(
      body: AsyncView<AlbumDetail>(
        value: album,
        onRetry: () => ref.invalidate(albumProvider(albumId)),
        builder: (detail) => _AlbumBody(detail: detail),
      ),
    );
  }
}

class _AlbumBody extends ConsumerWidget {
  const _AlbumBody({required this.detail});

  final AlbumDetail detail;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final text = Theme.of(context).textTheme;
    final colors = Theme.of(context).colorScheme;
    final margin = WindowSizeClass.of(context).screenMargin;
    final album = detail.album;

    return CustomScrollView(
      slivers: <Widget>[
        SliverAppBar(
          pinned: true,
          expandedHeight: 300,
          flexibleSpace: FlexibleSpaceBar(
            title: Text(album.name, style: text.titleMedium),
            centerTitle: false,
            titlePadding: const EdgeInsetsDirectional.only(start: 56, bottom: 16, end: 16),
            background: Stack(
              fit: StackFit.expand,
              children: <Widget>[
                CoverImage(coverUrl: album.coverUrl, borderRadius: 0),
                // A scrim so the title stays legible over any artwork, light
                // or dark.
                DecoratedBox(
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.center,
                      end: Alignment.bottomCenter,
                      colors: <Color>[
                        Colors.transparent,
                        colors.surface.withValues(alpha: 0.9),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        SliverToBoxAdapter(
          child: Padding(
            padding: EdgeInsets.fromLTRB(margin, 8, margin, 8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                InkWell(
                  onTap: () => context.go('/music/artists/${album.artistId}'),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(vertical: 4),
                    child: Text(
                      album.albumArtist,
                      style: text.titleSmall?.copyWith(color: colors.primary),
                    ),
                  ),
                ),
                Text(
                  _summary(album),
                  style: text.bodySmall?.copyWith(color: colors.onSurfaceVariant),
                ),
                const SizedBox(height: 16),
                Row(
                  children: <Widget>[
                    FilledButton.icon(
                      onPressed: () => ref
                          .read(playerControllerProvider)
                          .playQueue(detail.tracks),
                      icon: const Icon(Icons.play_arrow),
                      label: const Text('Play'),
                    ),
                    const SizedBox(width: 12),
                    FilledButton.tonalIcon(
                      onPressed: () async {
                        final player = ref.read(playerControllerProvider);
                        await player.toggleShuffle(true);
                        await player.playQueue(detail.tracks);
                      },
                      icon: const Icon(Icons.shuffle),
                      label: const Text('Shuffle'),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        ),
        SliverList.builder(
          itemCount: detail.tracks.length,
          itemBuilder: (context, index) {
            final track = detail.tracks[index];
            return TrackTile(
              track: track,
              leadingNumber: track.trackNo ?? index + 1,
              onTap: () => ref
                  .read(playerControllerProvider)
                  .playQueue(detail.tracks, index: index),
            );
          },
        ),
        const SliverToBoxAdapter(child: SizedBox(height: 96)),
      ],
    );
  }

  static String _summary(Album album) {
    final parts = <String>[
      if (album.year != null) '${album.year}',
      '${album.trackCount} ${album.trackCount == 1 ? 'track' : 'tracks'}',
      if (album.durationS > 0)
        formatDuration(Duration(seconds: album.durationS.round())),
    ];
    return parts.join(' · ');
  }
}
