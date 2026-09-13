import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/layout/window_size.dart';
import '../../../core/widgets/async_view.dart';
import '../data/models.dart';
import '../music_providers.dart';
import 'cover_image.dart';

/// One artist's albums, as a grid of covers.
class ArtistAlbumsScreen extends ConsumerWidget {
  const ArtistAlbumsScreen({super.key, required this.artistId});

  final String artistId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final albums = ref.watch(artistAlbumsProvider(artistId));

    return Scaffold(
      appBar: AppBar(
        title: Text(albums.valueOrNull?.firstOrNull?.albumArtist ?? 'Albums'),
      ),
      body: AsyncView<List<Album>>(
        value: albums,
        onRetry: () => ref.invalidate(artistAlbumsProvider(artistId)),
        isEmpty: (data) => data.isEmpty,
        emptyIcon: Icons.album_outlined,
        emptyTitle: 'No albums',
        builder: (data) => AlbumGrid(albums: data),
      ),
    );
  }
}

/// A responsive grid of album cards: two columns on a phone, four when there
/// is room, which is the M3 guidance for compact versus expanded windows.
class AlbumGrid extends StatelessWidget {
  const AlbumGrid({super.key, required this.albums});

  final List<Album> albums;

  static int columnsFor(WindowSizeClass sizeClass) =>
      sizeClass.isCompact ? 2 : 4;

  @override
  Widget build(BuildContext context) {
    final sizeClass = WindowSizeClass.of(context);
    final margin = sizeClass.screenMargin;

    return GridView.builder(
      padding: EdgeInsets.fromLTRB(margin, 16, margin, 96),
      gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: columnsFor(sizeClass),
        crossAxisSpacing: 16,
        mainAxisSpacing: 16,
        // Roughly square art plus two lines of text beneath it. The card gives
        // the text whatever it needs and lets the artwork take the rest, so a
        // larger system font scale shrinks the cover instead of overflowing.
        childAspectRatio: 0.68,
      ),
      itemCount: albums.length,
      itemBuilder: (context, index) => AlbumCard(album: albums[index]),
    );
  }
}

/// One album in the grid.
class AlbumCard extends StatelessWidget {
  const AlbumCard({super.key, required this.album});

  final Album album;

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final colors = Theme.of(context).colorScheme;

    return Card.filled(
      clipBehavior: Clip.antiAlias,
      margin: EdgeInsets.zero,
      child: InkWell(
        onTap: () => context.go('/music/albums/${album.id}'),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            Expanded(
              child: CoverImage(coverUrl: album.coverUrl, borderRadius: 0),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Text(
                    album.name,
                    style: text.titleMedium,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    album.year?.toString() ?? '—',
                    style: text.bodySmall?.copyWith(color: colors.onSurfaceVariant),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
