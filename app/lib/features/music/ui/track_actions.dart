import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../data/models.dart';
import '../music_providers.dart';
import '../player/player_controller.dart';
import '../../../core/api/api_error.dart';
import '../../../core/providers.dart';

/// One track in a list: number, title, artist and duration, with a three-dot
/// menu opening the M3 context sheet.
class TrackTile extends ConsumerWidget {
  const TrackTile({
    super.key,
    required this.track,
    required this.onTap,
    this.leadingNumber,
    this.trailing,
  });

  final Track track;
  final VoidCallback onTap;
  final int? leadingNumber;
  final Widget? trailing;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final text = Theme.of(context).textTheme;
    final colors = Theme.of(context).colorScheme;

    return ListTile(
      leading: leadingNumber == null
          ? null
          : SizedBox(
              width: 32,
              child: Text(
                '$leadingNumber',
                textAlign: TextAlign.center,
                style: text.bodyMedium?.copyWith(color: colors.onSurfaceVariant),
              ),
            ),
      title: Text(track.title, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Text(
        track.displayArtist,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      trailing: trailing ??
          Row(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              Text(
                formatDuration(track.duration),
                style: text.labelSmall?.copyWith(color: colors.onSurfaceVariant),
              ),
              IconButton(
                tooltip: 'More actions',
                icon: const Icon(Icons.more_vert),
                onPressed: () => showTrackActions(context, ref, track),
              ),
            ],
          ),
      onTap: onTap,
    );
  }
}

/// The M3 bottom sheet of per-track actions.
Future<void> showTrackActions(BuildContext context, WidgetRef ref, Track track) {
  return showModalBottomSheet<void>(
    context: context,
    showDragHandle: true,
    builder: (sheetContext) {
      return SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            ListTile(
              title: Text(track.title, maxLines: 1, overflow: TextOverflow.ellipsis),
              subtitle: Text(track.displayArtist),
            ),
            const Divider(height: 1),
            ListTile(
              leading: const Icon(Icons.queue_play_next),
              title: const Text('Play next'),
              onTap: () async {
                Navigator.of(sheetContext).pop();
                await ref.read(playerControllerProvider).playNext(track);
              },
            ),
            ListTile(
              leading: const Icon(Icons.playlist_add),
              title: const Text('Add to playlist'),
              onTap: () {
                Navigator.of(sheetContext).pop();
                showAddToPlaylistSheet(context, ref, track);
              },
            ),
            ListTile(
              leading: const Icon(Icons.person_outline),
              title: const Text('Go to artist'),
              onTap: () {
                Navigator.of(sheetContext).pop();
                context.go('/music/artists/${track.artistId}');
              },
            ),
          ],
        ),
      );
    },
  );
}

/// Picks a playlist to append [track] to, offering to make a new one.
Future<void> showAddToPlaylistSheet(
  BuildContext context,
  WidgetRef ref,
  Track track,
) {
  return showModalBottomSheet<void>(
    context: context,
    showDragHandle: true,
    isScrollControlled: true,
    builder: (sheetContext) {
      return SafeArea(
        child: Consumer(
          builder: (context, ref, _) {
            final playlists = ref.watch(playlistsProvider);
            return playlists.when(
              loading: () => const Padding(
                padding: EdgeInsets.all(32),
                child: Center(child: CircularProgressIndicator()),
              ),
              error: (error, _) => Padding(
                padding: const EdgeInsets.all(24),
                child: Text(error is ApiError ? error.message : 'Could not load playlists.'),
              ),
              data: (items) => Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  ListTile(
                    leading: const Icon(Icons.add),
                    title: const Text('New playlist'),
                    onTap: () async {
                      Navigator.of(sheetContext).pop();
                      await _createPlaylistWithTrack(context, ref, track);
                    },
                  ),
                  if (items.isNotEmpty) const Divider(height: 1),
                  Flexible(
                    child: ListView.builder(
                      shrinkWrap: true,
                      itemCount: items.length,
                      itemBuilder: (context, index) {
                        final playlist = items[index];
                        return ListTile(
                          leading: const Icon(Icons.queue_music),
                          title: Text(playlist.name),
                          subtitle: Text('${playlist.trackCount} tracks'),
                          onTap: () async {
                            Navigator.of(sheetContext).pop();
                            await _appendToPlaylist(context, ref, playlist.id, track);
                          },
                        );
                      },
                    ),
                  ),
                ],
              ),
            );
          },
        ),
      );
    },
  );
}

Future<void> _appendToPlaylist(
  BuildContext context,
  WidgetRef ref,
  String playlistId,
  Track track,
) async {
  final messenger = ScaffoldMessenger.of(context);
  final repository = ref.read(musicRepositoryProvider);
  try {
    // The API replaces the whole ordered list, so appending means reading the
    // current one first. That also means the server validates every id, which
    // catches tracks that have since left the library.
    final detail = await repository.playlist(playlistId);
    final ids = <String>[...detail.tracks.map((t) => t.id), track.id];
    await repository.setPlaylistTracks(playlistId, ids);

    ref.invalidate(playlistsProvider);
    ref.invalidate(playlistProvider(playlistId));
    messenger.showSnackBar(
      SnackBar(content: Text('Added to ${detail.playlist.name}')),
    );
  } on ApiError catch (e) {
    messenger.showSnackBar(SnackBar(content: Text(e.message)));
  }
}

Future<void> _createPlaylistWithTrack(
  BuildContext context,
  WidgetRef ref,
  Track track,
) async {
  final name = await promptForPlaylistName(context);
  if (name == null || !context.mounted) return;

  final messenger = ScaffoldMessenger.of(context);
  final repository = ref.read(musicRepositoryProvider);
  try {
    final playlist = await repository.createPlaylist(name);
    await repository.setPlaylistTracks(playlist.id, <String>[track.id]);
    ref.invalidate(playlistsProvider);
    messenger.showSnackBar(SnackBar(content: Text('Added to ${playlist.name}')));
  } on ApiError catch (e) {
    messenger.showSnackBar(SnackBar(content: Text(e.message)));
  }
}

/// The M3 dialog that asks for a playlist name.
Future<String?> promptForPlaylistName(
  BuildContext context, {
  String initialValue = '',
  String title = 'New playlist',
}) {
  final controller = TextEditingController(text: initialValue);

  return showDialog<String>(
    context: context,
    builder: (dialogContext) {
      return AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          autofocus: true,
          textCapitalization: TextCapitalization.sentences,
          decoration: const InputDecoration(
            labelText: 'Name',
            border: OutlineInputBorder(),
          ),
          onSubmitted: (value) => Navigator.of(dialogContext).pop(value.trim()),
        ),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(controller.text.trim()),
            child: const Text('Save'),
          ),
        ],
      );
    },
  ).then((value) => (value == null || value.isEmpty) ? null : value);
}
