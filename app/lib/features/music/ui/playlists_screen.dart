import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/api/api_error.dart';
import '../../../core/layout/window_size.dart';
import '../../../core/providers.dart';
import '../../../core/widgets/async_view.dart';
import '../data/models.dart';
import '../music_providers.dart';
import 'track_actions.dart';

/// The user's playlists.
class PlaylistsScreen extends ConsumerWidget {
  const PlaylistsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final playlists = ref.watch(playlistsProvider);
    final margin = WindowSizeClass.of(context).screenMargin - 16;

    return Scaffold(
      appBar: AppBar(title: const Text('Playlists')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _create(context, ref),
        icon: const Icon(Icons.add),
        label: const Text('New playlist'),
      ),
      body: RefreshIndicator(
        onRefresh: () async => ref.invalidate(playlistsProvider),
        child: AsyncView<List<Playlist>>(
          value: playlists,
          onRetry: () => ref.invalidate(playlistsProvider),
          isEmpty: (data) => data.isEmpty,
          emptyIcon: Icons.queue_music_outlined,
          emptyTitle: 'No playlists yet',
          emptyMessage: 'Make one here, or add a track to a new playlist from '
              'its three-dot menu.',
          builder: (data) => ListView.builder(
            padding: EdgeInsets.fromLTRB(margin, 8, margin, 96),
            itemCount: data.length,
            itemBuilder: (context, index) {
              final playlist = data[index];
              return ListTile(
                leading: const CircleAvatar(child: Icon(Icons.queue_music)),
                title: Text(playlist.name, maxLines: 1, overflow: TextOverflow.ellipsis),
                subtitle: Text(
                  '${playlist.trackCount} ${playlist.trackCount == 1 ? 'track' : 'tracks'}',
                ),
                trailing: const Icon(Icons.chevron_right),
                onTap: () => context.go('/music/playlists/${playlist.id}'),
              );
            },
          ),
        ),
      ),
    );
  }

  Future<void> _create(BuildContext context, WidgetRef ref) async {
    final name = await promptForPlaylistName(context);
    if (name == null || !context.mounted) return;

    final messenger = ScaffoldMessenger.of(context);
    try {
      final playlist = await ref.read(musicRepositoryProvider).createPlaylist(name);
      ref.invalidate(playlistsProvider);
      if (!context.mounted) return;
      context.go('/music/playlists/${playlist.id}');
    } on ApiError catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }
}
