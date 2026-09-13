import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/api_error.dart';
import '../../../core/providers.dart';
import '../../../core/widgets/async_view.dart';
import '../data/models.dart';
import '../music_providers.dart';
import '../player/player_controller.dart';
import 'track_actions.dart';

/// One playlist, with drag-to-reorder and undoable removal.
class PlaylistDetailScreen extends ConsumerStatefulWidget {
  const PlaylistDetailScreen({super.key, required this.playlistId});

  final String playlistId;

  @override
  ConsumerState<PlaylistDetailScreen> createState() => _PlaylistDetailScreenState();
}

class _PlaylistDetailScreenState extends ConsumerState<PlaylistDetailScreen> {
  /// The list as the user has rearranged it. Held locally so a drag feels
  /// instant; the server is told once, afterwards.
  List<Track>? _tracks;

  @override
  Widget build(BuildContext context) {
    final detail = ref.watch(playlistProvider(widget.playlistId));

    return Scaffold(
      appBar: AppBar(
        title: Text(detail.valueOrNull?.playlist.name ?? 'Playlist'),
        actions: <Widget>[
          IconButton(
            tooltip: 'Rename',
            icon: const Icon(Icons.edit_outlined),
            onPressed: detail.valueOrNull == null
                ? null
                : () => _rename(detail.value!.playlist),
          ),
          IconButton(
            tooltip: 'Delete playlist',
            icon: const Icon(Icons.delete_outline),
            onPressed: detail.valueOrNull == null ? null : _confirmDelete,
          ),
        ],
      ),
      body: AsyncView<PlaylistDetail>(
        value: detail,
        onRetry: () => ref.invalidate(playlistProvider(widget.playlistId)),
        isEmpty: (data) => data.tracks.isEmpty,
        emptyIcon: Icons.music_note_outlined,
        emptyTitle: 'This playlist is empty',
        emptyMessage: 'Add tracks from any album’s three-dot menu.',
        builder: (data) {
          final tracks = _tracks ?? data.tracks;
          return Column(
            children: <Widget>[
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
                child: Row(
                  children: <Widget>[
                    FilledButton.icon(
                      onPressed: () =>
                          ref.read(playerControllerProvider).playQueue(tracks),
                      icon: const Icon(Icons.play_arrow),
                      label: const Text('Play'),
                    ),
                    const Spacer(),
                    Text(
                      '${tracks.length} ${tracks.length == 1 ? 'track' : 'tracks'}',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
              Expanded(
                child: ReorderableListView.builder(
                  padding: const EdgeInsets.only(bottom: 96),
                  itemCount: tracks.length,
                  onReorderItem: (oldIndex, newIndex) => _reorder(tracks, oldIndex, newIndex),
                  itemBuilder: (context, index) {
                    final track = tracks[index];
                    return Dismissible(
                      key: ValueKey<String>('${track.id}-$index'),
                      direction: DismissDirection.endToStart,
                      background: const _RemoveBackground(),
                      onDismissed: (_) => _remove(tracks, index),
                      child: TrackTile(
                        track: track,
                        onTap: () => ref
                            .read(playerControllerProvider)
                            .playQueue(tracks, index: index),
                        trailing: ReorderableDragStartListener(
                          index: index,
                          child: const Padding(
                            padding: EdgeInsets.symmetric(horizontal: 12),
                            child: Icon(Icons.drag_handle),
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  void _reorder(List<Track> tracks, int oldIndex, int newIndex) {
    // onReorderItem already accounts for the removed item, so newIndex needs
    // no adjusting here.
    final reordered = List<Track>.of(tracks);
    reordered.insert(newIndex, reordered.removeAt(oldIndex));

    setState(() => _tracks = reordered);
    _save(reordered);
  }

  void _remove(List<Track> tracks, int index) {
    final removed = tracks[index];
    final remaining = List<Track>.of(tracks)..removeAt(index);

    setState(() => _tracks = remaining);
    _save(remaining);

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Removed ${removed.title}'),
        action: SnackBarAction(
          label: 'Undo',
          onPressed: () {
            final restored = List<Track>.of(remaining)..insert(index, removed);
            setState(() => _tracks = restored);
            _save(restored);
          },
        ),
      ),
    );
  }

  /// Pushes the whole ordered list. One request per gesture, and the server
  /// replaces rather than patches, so there is no way for the two to drift.
  Future<void> _save(List<Track> tracks) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(musicRepositoryProvider).setPlaylistTracks(
            widget.playlistId,
            tracks.map((t) => t.id).toList(growable: false),
          );
      ref.invalidate(playlistsProvider);
    } on ApiError catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
      // Put the server's version back on screen rather than leaving a local
      // order that was never saved.
      setState(() => _tracks = null);
      ref.invalidate(playlistProvider(widget.playlistId));
    }
  }

  Future<void> _rename(Playlist playlist) async {
    final name = await promptForPlaylistName(
      context,
      initialValue: playlist.name,
      title: 'Rename playlist',
    );
    if (name == null || !mounted) return;

    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(musicRepositoryProvider).renamePlaylist(playlist.id, name);
      ref.invalidate(playlistProvider(widget.playlistId));
      ref.invalidate(playlistsProvider);
    } on ApiError catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  Future<void> _confirmDelete() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete this playlist?'),
        content: const Text('The tracks stay in your library.'),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);
    try {
      await ref.read(musicRepositoryProvider).deletePlaylist(widget.playlistId);
      ref.invalidate(playlistsProvider);
      if (navigator.canPop()) navigator.pop();
    } on ApiError catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }
}

class _RemoveBackground extends StatelessWidget {
  const _RemoveBackground();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Container(
      color: colors.errorContainer,
      alignment: Alignment.centerRight,
      padding: const EdgeInsets.symmetric(horizontal: 24),
      child: Icon(Icons.delete_outline, color: colors.onErrorContainer),
    );
  }
}
